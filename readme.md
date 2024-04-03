
**I just built this project for learning, but if you like the simplicity why not give a hand.**

### Install

```bash
go get https://github.com/wassimbj/simpleq
```

### how it works
```go
/*

* The big picture of how the task queue works
                           ← ← ← ← ← ← ← ← ← ← ←
                          ↓                    ↑
                          ↓            --> failed => re-queue the job to the active queue
                  -- > active       /
                /                  /
job -> queue ->               ---> 
                \                  \
                 -- > delayed       \  --> completed => remove the job

*/
```

### Example

[a better one](https://github.com/wassimbj/simpleq/blob/master/example.md)

```go

emailsQueue := NewQueue("queueName", QueueOpts{
   client: redis.NewClient(&redis.Options{
      Addr:     "localhost:3434",
      Password: "",
   }),
})

emailsQueue.Process(func(job interface{}) error {
   fmt.Println("Job to process: ", job)
   return nil
})

emailsQueue.Add("welcome email")
emailsQueue.Add("delayed job, will exec after 7 sec", JobOpts{
   delay: int64((time.Second * 7) / time.Millisecond),
})


emailsQueue.Wait()

```

### TODO

- [ ] Add Cron Jobs
- [ ] Add Priorities
- [ ] Rate limiter for jobs
