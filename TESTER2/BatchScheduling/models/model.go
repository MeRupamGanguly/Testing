package models

import (
	"time"
)

type Task struct {
	ID          string    `db:"id"`
	Payload     string    `db:"payload"`
	Status      string    `db:"status"`
	Attempts    int       `db:"attempts"`
	MaxAttempts int       `db:"max_attempts"`
	NextRunAt   time.Time `db:"next_run_at"`
	LastError   string    `db:"last_error"`
	UpdatedAt   time.Time `db:"updated_at"`
}
