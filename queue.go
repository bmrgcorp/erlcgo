package erlcgo

import (
	"context"
	"sync"
	"time"
)

// RequestQueue manages queued API requests to prevent rate limit issues.
type RequestQueue struct {
	mu       sync.Mutex
	queue    chan *queuedRequest
	workers  int
	interval time.Duration
	running  bool
	stop     chan struct{}
}

type queuedRequest struct {
	ctx      context.Context
	execute  func() error
	response chan error
}

// NewRequestQueue creates a new request queue with the specified number of workers
// and interval between requests.
func NewRequestQueue(workers int, interval time.Duration) *RequestQueue {
	if workers <= 0 {
		workers = 1
	}
	if interval <= 0 {
		interval = time.Second // Default to 1 second between requests
	}

	return &RequestQueue{
		queue:    make(chan *queuedRequest, 50), // Reduced buffer size
		workers:  workers,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// Start begins processing queued requests
func (q *RequestQueue) Start() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.running {
		return
	}
	q.running = true

	for i := 0; i < q.workers; i++ {
		go q.worker()
	}
}

// Stop halts request processing
func (q *RequestQueue) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.running {
		return
	}
	q.running = false
	close(q.stop)
}

func (q *RequestQueue) worker() {
	ticker := time.NewTicker(q.interval)
	defer ticker.Stop()

	for {
		select {
		case <-q.stop:
			return
		case req := <-q.queue:
			select {
			case <-req.ctx.Done():
				req.response <- req.ctx.Err()
			default:
				req.response <- req.execute()
				<-ticker.C
			}
		}
	}
}

// Enqueue adds a request to the queue and returns a channel for the response
func (q *RequestQueue) Enqueue(ctx context.Context, execute func() error) error {
	req := &queuedRequest{
		ctx:      ctx,
		execute:  execute,
		response: make(chan error, 1),
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case q.queue <- req:
		return <-req.response
	}
}

// Depth returns the current number of requests waiting in the queue.
func (q *RequestQueue) Depth() int {
	return len(q.queue)
}
