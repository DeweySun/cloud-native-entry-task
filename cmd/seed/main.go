package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"go-entry-task/internal/config"
	"go-entry-task/internal/service"
	"go-entry-task/internal/store/mysqlstore"
)

func main() {
	configPath := flag.String("config", "config/config.json", "path to config JSON")
	countFlag := flag.Int("count", 0, "number of users to seed; defaults to config seed.count")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	count := cfg.Seed.Count
	if *countFlag > 0 {
		count = *countFlag
	}
	db, err := mysqlstore.Open(cfg.Database.DSN, cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns, cfg.Database.ConnMaxLifetime.Duration)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	store := mysqlstore.New(db)
	ctx := context.Background()
	if err := seed(ctx, store, cfg, count); err != nil {
		log.Fatal(err)
	}
	total, err := store.CountUsers(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("seed complete, users=%d", total)
}

func seed(ctx context.Context, store *mysqlstore.Store, cfg config.Config, count int) error {
	if count <= 0 {
		return nil
	}
	batchSize := cfg.Seed.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}
	workers := cfg.Seed.Concurrency
	if workers <= 0 {
		workers = 1
	}
	jobs := make(chan int, workers*2)
	var done atomic.Int64
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for startID := range jobs {
				size := min(batchSize, count-startID+1)
				users, err := makeUsers(startID, size, cfg.Security.PasswordIterations, cfg.Seed.DefaultPassword)
				if err != nil {
					log.Printf("make users failed: %v", err)
					continue
				}
				if err := store.InsertSeedUsers(ctx, users); err != nil {
					log.Printf("insert users failed: %v", err)
					continue
				}
				n := done.Add(int64(size))
				if n%100000 == 0 || int(n) == count {
					log.Printf("seeded %d/%d in %s", n, count, time.Since(start).Round(time.Second))
				}
			}
		}()
	}
	for startID := 1; startID <= count; startID += batchSize {
		jobs <- startID
	}
	close(jobs)
	wg.Wait()
	if int(done.Load()) < count {
		return fmt.Errorf("seed inserted %d of %d users", done.Load(), count)
	}
	return nil
}

func makeUsers(startID, count, iterations int, password string) ([]mysqlstore.SeedUser, error) {
	users := make([]mysqlstore.SeedUser, 0, count)
	for i := 0; i < count; i++ {
		id := startID + i
		salt, hash, err := service.HashPassword(password, iterations)
		if err != nil {
			return nil, err
		}
		users = append(users, mysqlstore.SeedUser{
			Username:     fmt.Sprintf("user%08d", id),
			Nickname:     fmt.Sprintf("User %08d", id),
			PasswordSalt: salt,
			PasswordHash: hash,
			PasswordIter: iterations,
		})
	}
	return users, nil
}
