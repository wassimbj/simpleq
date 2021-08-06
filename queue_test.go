package main

import (
	"strconv"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

func redisClient() *redis.Client {

	client := redis.NewClient(&redis.Options{
		Addr:     "0.0.0.0:3434",
		Password: "",
	})

	return client
}

func TestAddQueue(t *testing.T) {
	t.Run(".Add()", func(t *testing.T) {
		q := NewQueue("testQueue", QueueOpts{
			client: redisClient(),
		})

		// flush all to get the expected 10
		q.opts.client.FlushAll(q.ctx)

		for i := 1; i <= 10; i++ {
			q.Add("Data - " + strconv.Itoa(i))
		}

		// add a 3 delayed jobs
		for j := 1; j <= 3; j++ {
			q.Add("Delayed will exec after"+strconv.Itoa(j), JobOpts{
				delay: int64((time.Second * time.Duration(j)) / time.Millisecond),
			})
		}

		q.Wait()

		activeQueue, delayedQueue := q.Len()
		if activeQueue < 10 || delayedQueue < 3 {
			t.Fail()
		}
	})
}

func TestProcessQueue(t *testing.T) {
	t.Run(".Process()", func(t *testing.T) {
		q := NewQueue("testQueue", QueueOpts{
			client: redisClient(),
		})

		var count int64 = 0
		q.Process(func(job interface{}) error {
			// fmt.Println("Processing: ", job)
			if job != "" {
				count++
			}
			return nil
		})

		// using q.wait will keep the process running forever so won't benefit from that,
		// just add a deadline to wait for the process for a bit before exec the above code

		// deadline
		time.Sleep(time.Second * 7)
		// 13 = active jobs + delayed jobs added in the previous test
		if count < 13 {
			t.Fail()
		}
		q.Cancel()

	})
}
