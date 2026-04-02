package batch

import (
	"batchscheduling/BatchScheduling/models"
	"batchscheduling/BatchScheduling/repository"
	"context"
	"log"
	"sync"
)

type Batch struct {
	Repo repository.TaskRepository
}

func NewBatch(r repository.TaskRepository) *Batch {
	return &Batch{Repo: r}
}

/*
 * Instead of spinning up a new goroutine for every single task
 * the batch spins up a fixed workerCount.
 * The tasks fetched from the DB are loaded into a buffered channel.
 * The workers continuously pull from this channel.
 * The engine listens for context cancellation. If the system is shutting down or the batch has timed out,
 * the workers will immediately drop what they are doing and exit cleanly.
 */
func (e *Batch) RunBatch(ctx context.Context, batchSize int, workerCount int) {
	tasks, err := e.Repo.ClaimTasks(batchSize)
	if err != nil {
		log.Printf("Failed to claim tasks: %v", err)
		return
	}

	if len(tasks) == 0 {
		return // No tasks, exit early
	}

	var wg sync.WaitGroup
	// Buffer the channel to the batch size so the sender never blocks
	taskChan := make(chan models.Task, len(tasks))

	// Worker Pool
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range taskChan {
				// Respect context cancellation/timeouts
				select {
				case <-ctx.Done():
					log.Printf("Context cancelled, dropping task %s", t.ID)
					return
				default:
				}

				if err := e.process(ctx, t); err != nil {
					e.Repo.FailWithBackoff(t, err.Error())
				} else {
					e.Repo.ResolveTask(t.ID)
				}
			}
		}()
	}

	for _, t := range tasks {
		taskChan <- t
	}
	close(taskChan)
	wg.Wait()
}

// Business logic should also accept the context
func (e *Batch) process(ctx context.Context, t models.Task) error {
	log.Printf("Processing Task ID: %s", t.ID)
	// Add business logic here...
	// Ensure HTTP calls or DB queries inside here use `ctx`
	return nil
}
