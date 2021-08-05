```go
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

	emailsQueue.Process(func(job interface{}) error {
		fmt.Println("Job to process: ", job)
		return nil
	})

	http.HandleFunc("/auth/create", func(res http.ResponseWriter, req *http.Request) {
		emailsQueue.Add("{Welcome email data}")
		// fmt.Println("Created Account")
	})

	http.HandleFunc("/order", func(res http.ResponseWriter, req *http.Request) {
		emailsQueue.Add("{Delayed job data}", JobOpts{
			delay: int64((time.Second * 7) / time.Millisecond),
		})
	})

	http.ListenAndServe(":7777", nil)
}
```
