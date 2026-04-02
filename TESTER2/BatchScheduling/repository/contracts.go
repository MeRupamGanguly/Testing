package repository

import (
	"batchscheduling/BatchScheduling/models"
)

type TaskRepository interface {
	ClaimTasks(limit int) ([]models.Task, error)
	ResolveTask(id string) error
	FailWithBackoff(t models.Task, errStr string) error
	GetDeadLetterTasks(limit int) ([]models.Task, error)
	ResurrectTask(id string) error
}
