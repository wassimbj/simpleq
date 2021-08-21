package main

import (
	"context"
	"fmt"
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
		q.opts.client.FlushAll(context.Background())

		for i := 1; i <= 10; i++ {
			q.Add("Data - " + strconv.Itoa(i))
		}

		// add a 3 delayed jobs
		for j := 1; j <= 3; j++ {
			q.Add("Delayed will exec after"+strconv.Itoa(j), JobOpts{
				delay: int64((time.Second * time.Duration(j)) / time.Millisecond),
			})
		}

		// using q.wait() will wait forever, because its waiting for the health check to stop too,
		// and health check doesn't stop until there is something wrong, or we call the .Cancel()
		// so we sleep a bit to pretend that the healthcheck finished its work.
		time.Sleep(time.Second)

		activeQueue, delayedQueue := q.Len()
		if activeQueue < 10 || delayedQueue < 3 {
			t.Fail()
		}

		q.Cancel()
	})
}

func TestProcessQueue(t *testing.T) {
	t.Run(".Process()", func(t *testing.T) {
		q := NewQueue("testQueue", QueueOpts{
			client: redisClient(),
		})

		var count int64 = 0
		for i := 0; i < 5; i++ {
			q.Process(func(job interface{}) error {
				fmt.Println("Processing: ", job)
				if job != "" {
					count++
				}
				return nil
			})
		}

		// same as above, using q.wait will keep the process running forever so won't benefit from that,
		// just add a deadline to wait for the process for a bit before exec the above code

		// deadline, of 6 secs to wait for the delayed jobs to move to the active queue.
		time.Sleep(time.Second * 6)
		// 13 = active jobs + delayed jobs added in the previous test
		if count < 13 {
			t.Fail()
		}

		q.Cancel()

	})
}
