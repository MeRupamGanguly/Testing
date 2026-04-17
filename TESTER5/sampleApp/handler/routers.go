package handler

import (
	"github.com/gin-gonic/gin"
)

func NewRouter(orderHandler *OrderHandlerGin) *gin.Engine {
	r := gin.Default()

	api := r.Group("/orders")
	{
		api.POST("", orderHandler.CreateOrder)
		api.GET("", orderHandler.GetOrder)
		api.POST("/confirm", orderHandler.ConfirmOrder)
		api.POST("/cancel", orderHandler.CancelOrder)
	}

	return r
}
