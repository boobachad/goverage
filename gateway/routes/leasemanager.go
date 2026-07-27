package routes

import (
	"context"
	"log"
	"os"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
)

type LeaseManager struct {
	localBudget atomic.Int64
	rdb         *redis.Client
	leaseScript *redis.Script
	isFetching  atomic.Bool
	chunkSize   int64
	threshold   int64
}

func NewLeaseManager(rdb *redis.Client, chunkSize, threshold int64) *LeaseManager {
	content, err := os.ReadFile("redis_scripts/lease.lua")
	var script *redis.Script
	if err == nil {
		script = redis.NewScript(string(content))
	} else {
		log.Println("Warning: lease.lua not found, fallback to inline")
		// A simple lease script: pull 'chunkSize' from budget:fleet if enough exists, else pull remaining, else 0
		script = redis.NewScript(`
			local current = tonumber(redis.call("GET", KEYS[1]) or "0")
			local amount = tonumber(ARGV[1])
			if current >= amount then
				redis.call("DECRBY", KEYS[1], amount)
				return amount
			elseif current > 0 then
				redis.call("SET", KEYS[1], 0)
				return current
			else
				return 0
			end
		`)
	}

	return &LeaseManager{
		rdb:         rdb,
		leaseScript: script,
		chunkSize:   chunkSize,
		threshold:   threshold,
	}
}

func (lm *LeaseManager) Consume(amount int64) bool {
	for {
		current := lm.localBudget.Load()
		if current < amount {
			// Starved. We must fetch synchronously to see if there is budget.
			if lm.isFetching.CompareAndSwap(false, true) {
				res, err := lm.leaseScript.Run(context.Background(), lm.rdb, []string{"budget:fleet"}, lm.chunkSize).Result()
				lm.isFetching.Store(false)
				if err == nil {
					if fetched, ok := res.(int64); ok && fetched > 0 {
						lm.localBudget.Add(fetched)
						continue // Retry loop with new budget
					}
				}
			}
			return false // Truly out of budget
		}

		if lm.localBudget.CompareAndSwap(current, current-amount) {
			if current-amount < lm.threshold {
				lm.maybeFetch() // Asyncly fetch for next time
			}
			return true
		}
	}
}

// Refund amount back to the local budget.
func (lm *LeaseManager) Refund(amount int64) {
	lm.localBudget.Add(amount)
}

func (lm *LeaseManager) maybeFetch() {
	if lm.isFetching.CompareAndSwap(false, true) {
		go func() {
			defer lm.isFetching.Store(false)
			res, err := lm.leaseScript.Run(context.Background(), lm.rdb, []string{"budget:fleet"}, lm.chunkSize).Result()
			if err != nil {
				log.Println("LeaseManager fetch error:", err)
				return
			}
			if fetched, ok := res.(int64); ok && fetched > 0 {
				lm.localBudget.Add(fetched)
			}
		}()
	}
}
