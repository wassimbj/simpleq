

```go

emailsQueue := NewQueue("queueName", QueueOpts{
   client: redis.NewClient(&redis.Options{
      Addr:     "localhost:3434",
      Password: "",
   }),
})

emailsQueue.process(func(job interface{}) error {
   fmt.Println("Job to process: ", job)
   return nil
})

emailsQueue.add("data 1")
emailsQueue.add("data 2")
emailsQueue.add("data 4")
emailsQueue.add("delayed job, will exec after 7 sec", JobOpts{
   delay: int64((time.Second * 7) / time.Millisecond),
})

```
