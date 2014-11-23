package main

import (
    "net/http"
    "log"

    "github.com/garyburd/redigo/redis"
)

const (
    REDIS_ADDR  = "localhost:6379"
    SERVER_ADDR = ":3003"
)

func main() {
    redisPool := redis.NewPool(func() (redis.Conn, error) {
        c, err := redis.Dial("tcp", REDIS_ADDR)
        if err != nil {
            return nil, err
        }
        return c, nil
    }, 10)

    // The API :P
    http.HandleFunc("/wish", makeHandler(redisPool))

    // Simple Healthcheck
    http.HandleFunc("/__ping__",
        func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("pong\n"))
        })

    // Quite channel
    quitChan := make(chan int)

    // Start Background task runner
    go goScraper(redisPool, quitChan)

    log.Println("Server listening on port 3003")
    log.Fatal(http.ListenAndServe(SERVER_ADDR, nil))
}
