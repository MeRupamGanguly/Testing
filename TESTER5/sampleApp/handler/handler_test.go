package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"domainconcern/sampleApp/domain"
	"domainconcern/sampleApp/service"

	"github.com/gin-gonic/gin"
)

// mockOrderRepository (same as in handler_test.go, but we duplicate for clarity)
type mockOrderRepositoryGin struct {
	mu      sync.RWMutex
	orders  map[string]*domain.Order
	saveErr error
	findErr error
}

func newMockOrderRepositoryGin() *mockOrderRepositoryGin {
	return &mockOrderRepositoryGin{
		orders: make(map[string]*domain.Order),
	}
}

func (m *mockOrderRepositoryGin) Save(ctx context.Context, order *domain.Order) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orders[order.ID] = order
	return nil
}

func (m *mockOrderRepositoryGin) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	order, ok := m.orders[id]
	if !ok {
		return nil, errors.New("order not found")
	}
	return order, nil
}

func setupGinTest() (*gin.Engine, *mockOrderRepositoryGin, *service.OrderService) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockOrderRepositoryGin()
	orderSvc := service.NewOrderService(mockRepo, "USD")
	orderHandler := NewOrderHandler(orderSvc)
	router := NewRouter(orderHandler)
	return router, mockRepo, orderSvc
}

func TestGinOrderHandler_CreateOrder(t *testing.T) {
	router, _, _ := setupGinTest()

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name: "valid order",
			body: map[string]interface{}{
				"customer_id": "cust123",
				"items": []map[string]interface{}{
					{"product_id": "p1", "quantity": 2, "price": "19.99", "currency": "USD"},
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing customer_id",
			body: map[string]interface{}{
				"items": []map[string]interface{}{
					{"product_id": "p1", "quantity": 1, "price": "10.00", "currency": "USD"},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON",
			body:       `{"customer_id": "cust123", "items": [`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "empty items",
			body: map[string]interface{}{
				"customer_id": "cust123",
				"items":       []interface{}{},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "different currencies",
			body: map[string]interface{}{
				"customer_id": "cust123",
				"items": []map[string]interface{}{
					{"product_id": "p1", "quantity": 1, "price": "10.00", "currency": "USD"},
					{"product_id": "p2", "quantity": 1, "price": "10.00", "currency": "EUR"},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "zero quantity",
			body: map[string]interface{}{
				"customer_id": "cust123",
				"items": []map[string]interface{}{
					{"product_id": "p1", "quantity": 0, "price": "10.00", "currency": "USD"},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody []byte
			var err error
			switch v := tt.body.(type) {
			case string:
				reqBody = []byte(v)
			default:
				reqBody, err = json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestGinOrderHandler_GetOrder(t *testing.T) {
	router, _, svc := setupGinTest()

	// Pre-create an order via service
	createReq := service.CreateOrderRequest{
		CustomerID: "cust123",
		Items: []struct {
			ProductID string
			Quantity  int
			Price     string
			Currency  string
		}{
			{ProductID: "p1", Quantity: 2, Price: "19.99", Currency: "USD"},
		},
	}
	order, err := svc.CreateOrder(context.Background(), createReq)
	if err != nil {
		t.Fatal(err)
	}
	existingID := order.ID

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"existing order", "?id=" + existingID, http.StatusOK},
		{"missing id", "", http.StatusBadRequest},
		{"non-existent id", "?id=missing", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/orders"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("expected %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestGinOrderHandler_ConfirmOrder(t *testing.T) {
	router, mockRepo, svc := setupGinTest()

	// Create a pending order
	pendingOrder, _ := svc.CreateOrder(context.Background(), service.CreateOrderRequest{
		CustomerID: "cust123",
		Items: []struct {
			ProductID string
			Quantity  int
			Price     string
			Currency  string
		}{
			{ProductID: "p1", Quantity: 1, Price: "10.00", Currency: "USD"},
		},
	})
	pendingID := pendingOrder.ID

	// Create an already confirmed order directly via repo
	confirmedOrder := &domain.Order{
		ID:         "confirmed123",
		CustomerID: "cust456",
		Status:     domain.StatusConfirmed,
	}
	mockRepo.Save(context.Background(), confirmedOrder)

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"confirm pending", "?id=" + pendingID, http.StatusNoContent},
		{"confirm already confirmed", "?id=confirmed123", http.StatusBadRequest},
		{"confirm non-existent", "?id=missing", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/orders/confirm"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("expected %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestGinOrderHandler_CancelOrder(t *testing.T) {
	router, mockRepo, svc := setupGinTest()

	// Create a pending order
	pendingOrder, _ := svc.CreateOrder(context.Background(), service.CreateOrderRequest{
		CustomerID: "cust123",
		Items: []struct {
			ProductID string
			Quantity  int
			Price     string
			Currency  string
		}{
			{ProductID: "p1", Quantity: 1, Price: "10.00", Currency: "USD"},
		},
	})
	pendingID := pendingOrder.ID

	// Create an already cancelled order
	cancelledOrder := &domain.Order{
		ID:         "cancelled123",
		CustomerID: "cust456",
		Status:     domain.StatusCancelled,
	}
	mockRepo.Save(context.Background(), cancelledOrder)

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"cancel pending", "?id=" + pendingID, http.StatusNoContent},
		{"cancel already cancelled", "?id=cancelled123", http.StatusBadRequest},
		{"cancel non-existent", "?id=missing", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/orders/cancel"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("expected %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}
