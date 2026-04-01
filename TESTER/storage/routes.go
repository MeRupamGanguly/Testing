package storage

import (
	"github.com/gin-gonic/gin"
)

func NewRoutes(handler *StorageHandler) *gin.Engine {
	r := gin.Default()

	v1 := r.Group("/api/v1/storage")
	{
		v1.POST("/upload", handler.HandleUpload)
		v1.GET("/download", handler.HandleDownload)
		v1.POST("/delete", handler.HandleDelete)
		v1.POST("/signed-url", handler.HandleSignedURL)
		v1.GET("/list", handler.HandleList)
		v1.PUT("/modify", handler.HandleUpload)
	}

	return r
}
