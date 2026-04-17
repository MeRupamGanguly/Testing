package handler

import (
	"domainconcern/sampleApp/domain"
	"domainconcern/sampleApp/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type OrderHandlerGin struct {
	orderService *service.OrderService
}

func NewOrderHandler(orderService *service.OrderService) *OrderHandlerGin {
	return &OrderHandlerGin{orderService: orderService}
}

func (h *OrderHandlerGin) CreateOrder(c *gin.Context) {
	var req domain.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert to service request
	svcReq := service.CreateOrderRequest{CustomerID: req.CustomerID}
	for _, it := range req.Items {
		svcReq.Items = append(svcReq.Items, struct {
			ProductID string
			Quantity  int
			Price     string
			Currency  string
		}{
			ProductID: it.ProductID,
			Quantity:  it.Quantity,
			Price:     it.Price,
			Currency:  it.Currency,
		})
	}

	order, err := h.orderService.CreateOrder(c.Request.Context(), svcReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, mapToResponse(order))
}

func (h *OrderHandlerGin) GetOrder(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing order id"})
		return
	}

	order, err := h.orderService.GetOrder(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, mapToResponse(order))
}

func (h *OrderHandlerGin) ConfirmOrder(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing order id"})
		return
	}

	if err := h.orderService.ConfirmOrder(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *OrderHandlerGin) CancelOrder(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing order id"})
		return
	}

	if err := h.orderService.CancelOrder(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func mapToResponse(order *domain.Order) domain.OrderResponse {
	return domain.OrderResponse{
		ID:         order.ID,
		CustomerID: order.CustomerID,
		Total:      order.Total.String(),
		Status:     string(order.Status),
		CreatedAt:  order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:  order.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
