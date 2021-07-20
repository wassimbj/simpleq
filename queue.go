package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

/*

* The big picture of how the task queue works

!				 									--> failed => move the job back to the active queue
?	                 -- > active		  /
?                  /				 \	    /
?   job -> queue ->             ---->
?                  \           /	    \
?                   -- > delayed	  	  \
*													--> completed => remove the job

!PLAN
* jobs are processed at least once
* no need for failed queue, just if a job failed re-queue it back to be processed again
* if a job completes successfully remove it
* check each 2 seconds on redis, if something is wrong, stop everything
* each time we get a new job to process, we check for the delayed queue...
*...if the job time is equal to the current or its passed we move it the active
* a cron job is a delayed job that just doesn't get deleted...
*...it gets requeued over and over again with a new timestamp

*/

type QueueOpts struct {
	client *redis.Client
}

type Queue struct {
	name string
	done chan struct{}
	// stop when there is a system error
	stop chan error
	ctx  context.Context
	opts QueueOpts
	// wg   sync.WaitGroup
}

type JobOpts struct {
	// in millisecond
	delay int64
}

type queueTypes struct {
	delayed, active, failed string
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
		done: make(chan struct{}),
		ctx:  context.Background(),
		// wg:   sync.WaitGroup{},
		opts: options,
	}

	healthCheck(queue)

	return queue
}

func healthCheck(queue *Queue) {
	go func() {
		ticker := time.NewTicker(time.Second * 2)
		for {
			select {
			case <-queue.stop:
				return
			case <-ticker.C:
				err := queue.opts.client.Ping(queue.ctx).Err()
				if err != nil {
					queue.stop <- err
					fmt.Println("Unhealthy, stoping everything, (Error): ", err.Error())
					return
				}
				// fmt.Println("everything is cool")
			}
		}
	}()
}

// process the actual job
func (q *Queue) process(cb func(job interface{}) error) {
	go func() {
		for {
			select {
			case <-q.stop:
				// case <-q.done:
				return
			default:

				data := q.getJobs(q.name)
				err := cb(data)

				if err == nil {
					e := q.removeJob(queueName(q.name).active, data)
					if e != nil {
						fmt.Println("Remove job: ", e.Error())
					}
				} else {
					// put the job at the end of the queue
					q.opts.client.LPush(q.ctx, queueName(q.name).active, data)
				}
			}
			// wait 500ms before looping again...
			time.Sleep(time.Millisecond * 500)
		}
	}()
}

// remove job from the queue
func (q *Queue) removeJob(key string, job interface{}) error {
	res := q.opts.client.LRem(q.ctx, key, 0, job)
	return res.Err()
}

// stops the running worker
func (q *Queue) cancel() {
	q.done <- struct{}{}
}

// pulls off jobs from the queue and process them
func (q *Queue) getJobs(qName string) interface{} {

	// don't get and remove the job, just get it and if its successfully processed we remove it, manually
	lcmd := q.opts.client.LRange(q.ctx, queueName(q.name).active, 0, 0)

	// check if the delayed job's time is up
	checkForDelayedJob(q)

	if len(lcmd.Val()) == 0 {
		return ""
	}

	// fmt.Println("JOB: ", lcmd.Val())
	return lcmd.Val()[0]
}

func checkForDelayedJob(q *Queue) {
	delayedJob := q.opts.client.ZRangeWithScores(q.ctx, queueName(q.name).delayed, 0, 0)
	if len(delayedJob.Val()) == 0 {
		return
	}
	score := int64(delayedJob.Val()[0].Score)
	member := delayedJob.Val()[0].Member
	// data := delayedJob.Val()[0].Member.(string)
	// job := decodeJob([]byte(data))
	fmt.Println("Checking for delayed job...")
	fmt.Println(score, getCurrMilli())

	if getCurrMilli() >= score {
		fmt.Println("Yeee, found a delayed !!")
		q.opts.client.RPush(q.ctx, queueName(q.name).active, member)
		q.opts.client.ZRem(q.ctx, queueName(q.name).delayed, member)
	}
}

// adds a job to the right queue
func (q *Queue) add(data interface{}, opts ...JobOpts) {
	ops := JobOpts{}
	if len(opts) >= 1 {
		ops = opts[0]
	}
	delay := getCurrMilli() + ops.delay
	// fmt.Println(currMilli + ops.delay)

	dataToInsert := []interface{}{
		encodeJob(Job{
			Payload: data,
			Delay:   ops.delay,
			Id:      newUuid(),
		}),
	}

	go func() {
		// defer q.wg.Done()
		if ops.delay == 0 {
			lcmd := q.opts.client.RPush(q.ctx, queueName(q.name).active, dataToInsert)
			// fmt.Println("Rpushed job: ", ops.delay, data)
			if lcmd.Err() != nil {
				log.Println("RPush Error: ", lcmd.Err().Error())
				return
			}
			// log.Println("Rpush Res: ", lcmd.Val())
			// return
		} else {
			zcmd := q.opts.client.ZAdd(q.ctx, queueName(q.name).delayed, &redis.Z{
				Score:  float64(delay),
				Member: dataToInsert[0],
			})
			// fmt.Println("Zadded job: ", delay, data)
			if zcmd.Err() != nil {
				log.Println("Zadd Error: ", zcmd.Err().Error())
				return
			}
			// log.Println("Zadd Res: ", zcmd.Val())
			// return
		}
	}()
}

// Private methods

func newUuid() string {
	id := uuid.New()
	return id.String()
}

func encodeJob(data interface{}) []byte {
	d, _ := json.Marshal(data)
	return d
}

func decodeJob(data []byte) Job {
	var v Job
	json.Unmarshal(data, &v)
	return v
}

func getCurrMilli() int64 {
	return time.Now().UnixNano() / 1000000
}

func queueName(qName string) queueTypes {
	// queue
	return queueTypes{
		delayed: qName + ":delayed",
		active:  qName + ":active",
		failed:  qName + ":failed",
	}
}
