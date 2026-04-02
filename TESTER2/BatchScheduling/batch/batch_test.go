package batch

import (
	"batchscheduling/BatchScheduling/models"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock Definition ---

type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) ClaimTasks(limit int) ([]models.Task, error) {
	args := m.Called(limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockRepository) ResolveTask(id string) error {
	return m.Called(id).Error(0)
}

func (m *MockRepository) FailWithBackoff(t models.Task, errStr string) error {
	return m.Called(t, errStr).Error(0)
}

func (m *MockRepository) GetDeadLetterTasks(limit int) ([]models.Task, error) {
	args := m.Called(limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockRepository) ResurrectTask(id string) error {
	return m.Called(id).Error(0)
}

// --- Unit Tests ---

func TestRunBatch_Success(t *testing.T) {
	repo := new(MockRepository)
	Batch := NewBatch(repo)

	tasks := []models.Task{
		{ID: "1", Attempts: 0, MaxAttempts: 5},
		{ID: "2", Attempts: 0, MaxAttempts: 5},
	}

	repo.On("ClaimTasks", 10).Return(tasks, nil)
	repo.On("ResolveTask", "1").Return(nil)
	repo.On("ResolveTask", "2").Return(nil)

	Batch.RunBatch(context.Background(), 10, 2)

	repo.AssertExpectations(t)
}

func TestRunBatch_RepoClaimError(t *testing.T) {
	repo := new(MockRepository)
	Batch := NewBatch(repo)

	repo.On("ClaimTasks", 10).Return(nil, errors.New("connection refused"))

	Batch.RunBatch(context.Background(), 10, 2)

	repo.AssertNotCalled(t, "ResolveTask", mock.Anything)
}

func TestRunBatch_ContextTimeout(t *testing.T) {
	repo := new(MockRepository)
	Batch := NewBatch(repo)

	tasks := []models.Task{{ID: "1", Attempts: 0}}
	repo.On("ClaimTasks", 10).Return(tasks, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	Batch.RunBatch(ctx, 10, 1)

	// Should exit without processing
	repo.AssertNotCalled(t, "ResolveTask", "1")
}

func TestRunBatch_WorkerBottleneck(t *testing.T) {
	repo := new(MockRepository)
	Batch := NewBatch(repo)

	var tasks []models.Task
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("task-%d", i)
		tasks = append(tasks, models.Task{ID: id})
		repo.On("ResolveTask", id).Return(nil)
	}

	repo.On("ClaimTasks", 3).Return(tasks, nil)

	// 3 tasks with only 1 worker (serial processing)
	Batch.RunBatch(context.Background(), 3, 1)

	repo.AssertExpectations(t)
}

func TestResurrect_Success(t *testing.T) {
	repo := new(MockRepository)
	repo.On("ResurrectTask", "dead-1").Return(nil)

	err := repo.ResurrectTask("dead-1")
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
