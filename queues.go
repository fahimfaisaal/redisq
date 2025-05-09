package redisq

import "github.com/redis/go-redis/v9"

type queues struct {
	client *redis.Client
}

type Queues interface {
	NewQueue(queueKey string) *Queue
	NewPriorityQueue(queueKey string) *PriorityQueue
	NewDistributedQueue(queueKey string) *DistributedQueue
	NewDistributedPriorityQueue(queueKey string) *DistributedPriorityQueue
	Close() error
}

func New(url string) Queues {
	opts, err := redis.ParseURL(url)
	if err != nil {
		panic(err)
	}

	client := redis.NewClient(opts)

	return &queues{
		client: client,
	}
}

func (qs *queues) NewQueue(queueKey string) *Queue {
	return newQueue(qs.client, queueKey)
}

func (qs *queues) NewPriorityQueue(queueKey string) *PriorityQueue {
	return newPriorityQueue(qs.client, queueKey)
}

func (qs *queues) NewDistributedQueue(queueKey string) *DistributedQueue {
	return newDistributedQueue(qs.client, queueKey)
}

func (qs *queues) NewDistributedPriorityQueue(queueKey string) *DistributedPriorityQueue {
	return newDistributedPriorityQueue(qs.client, queueKey)
}

func (qs *queues) Close() error {
	return qs.client.Close()
}
