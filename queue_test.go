package main

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
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

const numTasks = 1000

func TestAddQueue(t *testing.T) {
	t.Run(".Add()", func(t *testing.T) {
		q := NewQueue("testQueue", QueueOpts{
			client: redisClient(),
		})

		// flush all to get the expected 10
		q.opts.client.FlushAll(context.Background())

		for i := 1; i <= numTasks; i++ {
			q.Add("Data - " + strconv.Itoa(i))
		}

		// add a 3 delayed jobs
		for j := 1; j <= 3; j++ {
			q.Add("Delayed Job - "+strconv.Itoa(j*3), JobOpts{
				delay: int64((time.Second * time.Duration(j*3)) / time.Millisecond),
			})
		}

		// using q.wait() will wait forever, because its waiting for the health check to stop too,
		// and health check doesn't stop until there is something wrong, or we call the .Cancel()
		// so we sleep a bit to pretend that the healthcheck finished its work.
		time.Sleep(time.Second * 1)

		qLen := q.Len()
		if qLen.active < numTasks && qLen.delayed < 3 {
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
		//
		for i := 0; i < 5; i++ {
			q.Process(func(job interface{}) error {
				if job != "" {
					fmt.Println("Processing: ", job)
					atomic.AddInt64(&count, 1)
				}
				return nil
			})
		}

		// sleep to wait for the delayed jobs
		time.Sleep(time.Second * 10)

		if count < numTasks+3 {
			t.Fail()
		}

		q.Cancel()

	})
}
