package redisq

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisQueue struct {
	client     *redis.Client
	queueKey   string
	mx         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	expiration time.Duration
}

func NewRedisQueue(queueKey string, url string) *RedisQueue {
	opts, err := redis.ParseURL(url)
	if err != nil {
		panic(err)
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithCancel(context.Background())

	return &RedisQueue{
		client:   client,
		queueKey: queueKey,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// SetExpiration sets the expiration time for the RedisQueue
func (q *RedisQueue) SetExpiration(expiration time.Duration) {
	q.mx.Lock()
	defer q.mx.Unlock()
	q.expiration = expiration
}

func (q *RedisQueue) Dequeue() (any, bool) {
	q.mx.Lock()
	defer q.mx.Unlock()

	result, err := q.client.LPop(q.ctx, q.queueKey).Bytes()
	if err == redis.Nil {
		return nil, false
	}
	if err != nil {
		log.Printf("Error dequeuing: %v", err)
		return nil, false
	}

	return result, true
}

func (q *RedisQueue) toBytes(item any) ([]byte, error) {
	var data []byte
	switch v := item.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}
	return data, nil
}

func (q *RedisQueue) Enqueue(item any) bool {
	q.mx.Lock()
	defer q.mx.Unlock()
	data, err := q.toBytes(item)

	if err != nil {
		log.Printf("Error converting item to bytes: %v", err)
		return false
	}

	pipe := q.client.Pipeline()
	pipe.RPush(q.ctx, q.queueKey, data)

	if q.expiration > 0 {
		pipe.Expire(q.ctx, q.queueKey, q.expiration)
	}

	if _, err := pipe.Exec(q.ctx); err != nil {
		log.Printf("Error enqueueing item: %v", err)
		return false
	}

	return true
}

func (q *RedisQueue) Len() int {
	q.mx.Lock()
	defer q.mx.Unlock()

	length, err := q.client.LLen(q.ctx, q.queueKey).Result()
	if err != nil {
		return 0
	}
	return int(length)
}

func (q *RedisQueue) Values() []any {
	q.mx.Lock()
	defer q.mx.Unlock()

	results, err := q.client.LRange(q.ctx, q.queueKey, 0, -1).Result()
	if err != nil {
		return []any{}
	}

	values := make([]any, 0, len(results))
	for _, result := range results {
		// Just convert the string to bytes without base64 decoding
		values = append(values, []byte(result))
	}

	return values
}

func (q *RedisQueue) Close() error {
	q.cancel() // Cancel context to stop notification listener
	return q.client.Close()
}

func (q *RedisQueue) Listen() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if r := recover(); r != nil {
		q.Close()
		signal.Stop(sigChan)
		close(sigChan)
		panic("Redis queue listener terminated due to panic")
	}

	<-sigChan
}
