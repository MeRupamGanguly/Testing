package repository

import (
	"batchscheduling/BatchScheduling/models"
	"database/sql"
	"fmt"

	"math"
	"math/rand"
	"time"
)

type Repository struct {
	Conn *sql.DB
}

// ClaimTasks selects tasks eligible for processing or replay
func (r *Repository) ClaimTasks(limit int) ([]models.Task, error) {
	query := `
		UPDATE batch_tasks
		SET status = 'PROCESSING', updated_at = NOW()
		WHERE id IN (
			SELECT id FROM batch_tasks
			WHERE (status = 'PENDING' OR status = 'FAILED')
			AND next_run_at <= NOW()
			AND attempts < max_attempts
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		RETURNING id, payload, attempts, max_attempts;`

	rows, err := r.Conn.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		rows.Scan(&t.ID, &t.Payload, &t.Attempts, &t.MaxAttempts)
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// ResolveTask handles success (Idempotency)
func (r *Repository) ResolveTask(id string) error {
	_, err := r.Conn.Exec("UPDATE batch_tasks SET status = 'COMPLETED', updated_at = NOW() WHERE id = $1", id)
	return err
}

// FailWithBackoff calculates exponential backoff + jitter
func (r *Repository) FailWithBackoff(t models.Task, errStr string) error {
	// Exponential Backoff: 2^attempts * 30s + Random Jitter
	jitter := rand.Intn(30)
	backoff := int(math.Pow(2, float64(t.Attempts)))*30 + jitter
	nextRun := time.Now().Add(time.Duration(backoff) * time.Second)

	status := "FAILED"
	// Dynamically check against the task's defined max attempts
	if t.Attempts >= t.MaxAttempts-1 {
		status = "DEAD_LETTER"
	}

	query := `
		UPDATE batch_tasks
		SET status = $1, attempts = attempts + 1, next_run_at = $2, last_error = $3, updated_at = NOW()
		WHERE id = $4`

	_, err := r.Conn.Exec(query, status, nextRun, errStr, t.ID)
	return err
}

// GetDeadLetterTasks retrieves tasks that have exhausted all retry attempts.
// This is used for manual inspection or administrative dashboards.
func (r *Repository) GetDeadLetterTasks(limit int) ([]models.Task, error) {
	query := `
		SELECT id, payload, attempts, last_error, updated_at
		FROM batch_tasks
		WHERE status = 'DEAD_LETTER'
		ORDER BY updated_at DESC
		LIMIT $1`

	rows, err := r.Conn.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		// Note: Ensure your models.Task struct has these fields
		// or use a specific DTO for dead letters.
		rows.Scan(&t.ID, &t.Payload, &t.Attempts, &t.LastError, &t.UpdatedAt)
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// ResurrectTask moves a task from DEAD_LETTER back to PENDING.
// This is the "Replay" mechanism used after an engineer fixes the root cause.
func (r *Repository) ResurrectTask(id string) error {
	query := `
		UPDATE batch_tasks
		SET status = 'PENDING',
		    attempts = 0,
		    next_run_at = NOW(),
		    last_error = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'DEAD_LETTER'`

	result, err := r.Conn.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no dead letter task found with ID: %s", id)
	}

	return nil
}
