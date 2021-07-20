package main

import (
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

func createQueue(name string) *Queue {
	return NewQueue(name, QueueOpts{
		client: redis.NewClient(&redis.Options{
			Addr:     "localhost:3434",
			Password: "",
		}),
	})
}

func main() {

	emailsQueue := createQueue("emailsQueue")

	// c := emailsQueue.opts.client.LRange(emailsQueue.ctx, "emailsQueue:active", 0, -1)
	// fmt.Print(c.Val())

	emailsQueue.process(func(job interface{}) error {
		fmt.Println("Job to process: ", job)
		return nil
	})

	// emailsQueue.add("data 1")
	// emailsQueue.add("data 2")
	// emailsQueue.add("data 4")
	// emailsQueue.add("delayed job data 5", JobOpts{
	// 	delay: int64((time.Second * 7) / time.Millisecond),
	// })

	// emailsQueue.Add("Send welcome email")

	// http.HandleFunc("/create", func(res http.ResponseWriter, req *http.Request) {
	// 	emailsQueue.Add("Send welcome email")
	// 	fmt.Println("Created Account")
	// })
	// http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {

	// 	fmt.Print("Hello world")

	// })
	// http.ListenAndServe(":7777", nil)
	// fmt.Println("Hello World this should run...")

	time.Sleep(time.Hour)

	// emailsQueue.stop()
}
