package proxy

import (
	"context"
	"log"
	"net/http"
	"sync"
)

// WorkerPool manages a pool of workers for handling HTTP requests
type WorkerPool struct {
	jobQueue   chan *job
	wg         sync.WaitGroup
	maxWorkers int
}

// job represents a request to be processed
type job struct {
	w    http.ResponseWriter
	r    *http.Request
	done chan struct{}
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(maxWorkers int) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = 10 // Default to 10 workers if invalid number provided
	}

	pool := &WorkerPool{
		jobQueue:   make(chan *job, maxWorkers*2), // Buffer size twice the number of workers
		maxWorkers: maxWorkers,
	}

	// Start the workers
	pool.start()
	return pool
}

// start launches the worker goroutines
func (wp *WorkerPool) start() {
	for i := 0; i < wp.maxWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
	log.Printf("Started %d workers in the pool", wp.maxWorkers)
}

// worker processes jobs from the job queue
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	for job := range wp.jobQueue {
		// Process the request
		handler := job.r.Context().Value(handlerContextKey).(http.Handler)
		handler.ServeHTTP(job.w, job.r)

		// Signal that the job is done
		close(job.done)
	}
}

// Enqueue adds a new job to the queue
func (wp *WorkerPool) Enqueue(w http.ResponseWriter, r *http.Request, handler http.Handler) {
	// Create a done channel for synchronization
	done := make(chan struct{})

	// Store the handler in the request context
	ctx := context.WithValue(r.Context(), handlerContextKey, handler)
	r = r.WithContext(ctx)

	// Create a new job
	job := &job{
		w:    w,
		r:    r,
		done: done,
	}

	// Add the job to the queue
	wp.jobQueue <- job

	// Wait for the job to complete
	<-done
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop() {
	close(wp.jobQueue)
	wp.wg.Wait()
	log.Printf("Worker pool stopped")
}

// handlerContextKey is a key for storing the http.Handler in the request context
type contextKey string
const handlerContextKey contextKey = "handler"