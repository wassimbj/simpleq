package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

type QueueOpts struct {
	client *redis.Client
}

type Queue struct {
	name string
	// stop processing
	stop chan error
	mx   *sync.Mutex
	done chan struct{}
	opts QueueOpts
	wg   sync.WaitGroup
}

type JobOpts struct {
	// in millisecond
	delay int64
}

type queueTypes struct {
	delayed, active, failed string
}

type queueLen struct {
	delayed, active, failed int64
}
type Job struct {
	Payload interface{}
	Delay   int64
	Id      string
}

// opts are optinal
func NewQueue(name string, opts ...QueueOpts) *Queue {
	var options QueueOpts
	if len(opts) > 1 {
		log.Println("NewQueue expect at last 1 params and at most 2")
	} else if len(opts) == 1 {
		options = opts[0]
	} else {
		options = QueueOpts{
			client: redis.NewClient(&redis.Options{
				Addr:     "localhost:6379",
				Password: "",
			}),
		}
	}

	queue := &Queue{
		name: name,
		stop: make(chan error, 1),
		wg:   sync.WaitGroup{},
		mx:   &sync.Mutex{},
		opts: options,
	}

	healthCheck(queue)

	return queue
}

// process the actual job
func (q *Queue) Process(cb func(job interface{}) error) {
	q.wg.Add(1)
	go func() {
		for {
			select {
			case <-q.stop:
				q.wg.Done()
				return
			default:
			}
			qLen := q.Len()
			// if queue is empty sleep for a second
			if qLen.active == 0 {
				time.Sleep(time.Second * 3)
			}

			q.mx.Lock() // mutex
			// get next job to process
			data := q.GetNextJob(q.name)
			// callback func to give the user the data
			err := cb(data)

			if err == nil {
				e := q.RemoveJob(queue(q.name).active, data)
				if e != nil {
					fmt.Println("Error (Remove job): ", e.Error())
				}
			} else {
				// put the job at the end of the queue
				requeueJob(q, data)
			}
			q.mx.Unlock() // mutex
		}
	}()
}

// adds a job to the right queue
func (q *Queue) Add(data interface{}, opts ...JobOpts) {
	ops := JobOpts{}
	if len(opts) >= 1 {
		ops = opts[0]
	}
	delay := getCurrMilli() + ops.delay

	dataToInsert := []interface{}{
		q.EncodeJob(Job{
			Payload: data,
			Delay:   ops.delay,
			Id:      newUuid(),
		}),
	}

	q.wg.Add(1)
	go func() {
		q.mx.Lock()
		defer func() {
			q.mx.Unlock()
			q.wg.Done()
		}()

		if ops.delay == 0 {
			lcmd := q.opts.client.RPush(context.Background(), queue(q.name).active, dataToInsert)
			if lcmd.Err() != nil {
				log.Fatal("RPush Error: ", lcmd.Err().Error())
				return
			}
		} else {
			zcmd := q.opts.client.ZAdd(context.Background(), queue(q.name).delayed, &redis.Z{
				Score:  float64(delay),
				Member: dataToInsert[0],
			})
			if zcmd.Err() != nil {
				log.Fatal("Zadd Error: ", zcmd.Err().Error())
				return
			}
		}
	}()
}

// returns the next job in the active queue, and checks for delayed jobs
func (q *Queue) GetNextJob(qName string) interface{} {

	// don't get and remove the job (BLPOP), just get it and if its successfully processed we remove it, manually
	licmd := q.opts.client.LIndex(context.Background(), queue(q.name).active, 0)

	// check for delayed jobs and add them to the active queue if their time is up
	checkForDelayedJob(q)

	if licmd.Val() == "" {
		return ""
	}
	return licmd.Val()
}

// remove job from the queue
func (q *Queue) RemoveJob(key string, job interface{}) error {
	res := q.opts.client.LRem(context.Background(), key, 0, job)
	return res.Err()
}

// stops the running workers
func (q *Queue) Cancel() {
	q.stop <- nil
}

// wait for the goroutines
func (q *Queue) Wait() {
	q.wg.Wait()
}

// returns the length of the active queue
func (q *Queue) Len() queueLen {
	activeQueue := q.opts.client.LLen(context.Background(), queue(q.name).active)
	delayedQueue := q.opts.client.ZCard(context.Background(), queue(q.name).delayed)

	return queueLen{
		delayed: delayedQueue.Val(),
		active:  activeQueue.Val(),
	}
	// return activeQueue.Val(), delayedQueue.Val()
}

func (q *Queue) EncodeJob(data interface{}) []byte {
	d, _ := json.Marshal(data)
	return d
}

func (q *Queue) DecodeJob(data []byte) Job {
	var v Job
	json.Unmarshal(data, &v)
	return v
}

// ####### Private methods #######

func requeueJob(q *Queue, job interface{}) {
	pipe := q.opts.client.TxPipeline()
	pipe.LRem(context.Background(), queue(q.name).active, 0, job)
	pipe.RPush(context.Background(), queue(q.name).active, job)
	pipe.Exec(context.Background())

}

func healthCheck(queue *Queue) {
	queue.wg.Add(1)
	go func() {
		ticker := time.NewTicker(time.Second * 2)
		for {
			select {
			case <-queue.stop:
				queue.wg.Done()
				return
			case <-ticker.C:
				err := queue.opts.client.Ping(context.Background()).Err()
				if err != nil {
					queue.stop <- err
					fmt.Println("Unhealthy, stoping everything, (Error): ", err.Error())
				}
			}
		}
	}()
}

func checkForDelayedJob(q *Queue) {
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		delayedJob := q.opts.client.ZRangeWithScores(context.Background(), queue(q.name).delayed, 0, 0)
		if len(delayedJob.Val()) == 0 {
			return
		}
		score := int64(delayedJob.Val()[0].Score)
		member := delayedJob.Val()[0].Member

		if getCurrMilli() >= score {
			// Found a delayed job
			pipe := q.opts.client.TxPipeline()
			pipe.RPush(context.Background(), queue(q.name).active, member)
			pipe.ZRem(context.Background(), queue(q.name).delayed, member)
			pipe.Exec(context.Background())
		}
	}()
}

func newUuid() string {
	id := uuid.New()
	return id.String()
}

func getCurrMilli() int64 {
	return time.Now().UnixNano() / 1000000
}

func queue(qName string) queueTypes {
	return queueTypes{
		delayed: qName + ":delayed",
		active:  qName + ":active",
	}
}
