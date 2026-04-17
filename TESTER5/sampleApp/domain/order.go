package domain

import (
	"domainconcern/utils/money"
	"time"
)

type OrderStatus string

const (
	StatusPending   OrderStatus = "PENDING"
	StatusConfirmed OrderStatus = "CONFIRMED"
	StatusCancelled OrderStatus = "CANCELLED"
)

type OrderItem struct {
	ProductID string
	Quantity  int
	Price     money.Money
}

type Order struct {
	ID         string
	CustomerID string
	Items      []OrderItem
	Total      money.Money
	Status     OrderStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type CreateOrderRequest struct {
	CustomerID string `json:"customer_id" binding:"required"`
	Items      []struct {
		ProductID string `json:"product_id" binding:"required"`
		Quantity  int    `json:"quantity" binding:"required,gt=0"`
		Price     string `json:"price" binding:"required"`
		Currency  string `json:"currency"`
	} `json:"items" binding:"required,min=1"`
}

type OrderResponse struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	Total      string `json:"total"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}
