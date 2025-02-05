package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Args[1],
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	ctx := context.Background()

	err := rdb.Set(ctx, "counter", 0.0, 0).Err()
	if err != nil {
		log.Fatalf("creating redis key: %v", err)
	}

	for {
		err = rdb.Incr(ctx, "counter").Err()
		if err != nil {
			log.Fatalf("incrementing counter: %v", err)
		}

		cmd := rdb.Get(ctx, "counter")
		if err := cmd.Err(); err != nil {
			log.Fatalf("getting current value: %v", err)
		}

		current, _ := cmd.Float64()
		log.Printf("Current value: %f", current)
		time.Sleep(time.Second)
	}
}
