package storage

import (
	"cloudstorage/storage/models"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type StorageHandler struct {
	Storage StorageProvider
}

func NewStorageHandler(p StorageProvider) *StorageHandler {
	return &StorageHandler{Storage: p}
}

func (h *StorageHandler) HandleUpload(c *gin.Context) {
	log.Println("Upload Handler Called")
	bucket := c.PostForm("bucket")
	if bucket == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Success: false, Error: "Bucket name is required"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Success: false, Error: "No file attached"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Success: false, Error: "Read error"})
		return
	}
	defer src.Close()

	if err := h.Storage.Upload(c.Request.Context(), bucket, file.Filename, src); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.UploadResponse{
		Success:    true,
		Message:    "File uploaded",
		Bucket:     bucket,
		ObjectName: file.Filename,
	})
}

func (h *StorageHandler) HandleDownload(c *gin.Context) {
	log.Println("Download Handler Called")
	var req models.DownloadRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Success: false, Error: err.Error()})
		return
	}

	reader, err := h.Storage.Download(c.Request.Context(), req.Bucket, req.Filepath)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Success: false, Error: "File not found"})
		return
	}
	defer reader.Close()

	c.Header("Content-Disposition", "attachment; filename="+req.Filepath)
	c.DataFromReader(http.StatusOK, -1, "application/octet-stream", reader, nil)
}

func (h *StorageHandler) HandleDelete(c *gin.Context) {
	log.Println("Delete Handler Called")
	var req models.DeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Success: false, Error: err.Error()})
		return
	}

	if err := h.Storage.Delete(c.Request.Context(), req.Bucket, req.Filepath); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.DeleteResponse{
		Success:    true,
		Message:    "Deleted successfully",
		Bucket:     req.Bucket,
		ObjectName: req.Filepath,
	})
}

func (h *StorageHandler) HandleSignedURL(c *gin.Context) {
	log.Println("Signed Url Handler Called")
	var req models.SignedURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Success: false, Error: err.Error()})
		return
	}

	duration := time.Duration(req.DurationInMinutes) * time.Minute
	url, err := h.Storage.GetSignedURL(c.Request.Context(), req.Bucket, req.Filepath, duration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.SignedURLResponse{
		Success:    true,
		Bucket:     req.Bucket,
		ObjectName: req.Filepath,
		SignedURL:  url,
		ExpiresAt:  time.Now().Add(duration),
	})
}

func (h *StorageHandler) HandleList(c *gin.Context) {
	log.Println("List Handler Called")
	var req models.ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Success: false, Error: err.Error()})
		return
	}

	objects, err := h.Storage.List(c.Request.Context(), req.Bucket, req.Prefix)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.ListResponse{
		Success: true,
		Bucket:  req.Bucket,
		Objects: objects,
	})
}
