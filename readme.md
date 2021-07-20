
**If you want to know how something works, build it by yourself, --me**

i just built this project for learning purposes only, but if you like the simplicity of the usage, why not take it from here.

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

// http.post("/create, func(){
   emailsQueue.add("welcome email")
   emailsQueue.add("delayed job, will exec after 7 sec", JobOpts{
      delay: int64((time.Second * 7) / time.Millisecond),
// })
})

```
