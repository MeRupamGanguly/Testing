package storage

import (
	"bytes"
	"cloudstorage/storage"
	"cloudstorage/storage/models"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockStorage := new(MockStorageProvider)
	handler := storage.NewStorageHandler(mockStorage)
	router := gin.Default()
	router.GET("/list", handler.HandleList)
	expectedFiles := []string{"file1.txt", "file2.jpg"}
	mockStorage.On("List", mock.Anything, "my-bucket", "test-prefix").Return(expectedFiles, nil)
	req, _ := http.NewRequest("GET", "/list?bucket=my-bucket&prefix=test-prefix", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var response models.ListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, expectedFiles, response.Objects)
}

func TestHandleUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockStorage := new(MockStorageProvider)
	handler := storage.NewStorageHandler(mockStorage)
	router := gin.Default()
	router.POST("/upload", handler.HandleUpload)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("bucket", "test-bucket")
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("hello world"))
	writer.Close()
	mockStorage.On("Upload", mock.Anything, "test-bucket", "test.txt", mock.Anything).Return(nil)
	req, _ := http.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
}

func TestHandleDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockStorage := new(MockStorageProvider)
	handler := storage.NewStorageHandler(mockStorage)
	router := gin.Default()
	router.DELETE("/delete", handler.HandleDelete)
	mockStorage.On("Delete", mock.Anything, "my-bucket", "trash.txt").Return(nil)
	reqBody := `{"bucket":"my-bucket", "filepath":"trash.txt"}`
	req, _ := http.NewRequest("DELETE", "/delete?bucket=my-bucket&filepath=trash.txt", nil)
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	mockStorage.AssertExpectations(t)
}

func TestHandleDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockStorage := new(MockStorageProvider)
	handler := storage.NewStorageHandler(mockStorage)
	router := gin.Default()
	router.GET("/download", handler.HandleDownload)
	content := "fake file content"
	reader := io.NopCloser(strings.NewReader(content))
	mockStorage.On("Download", mock.Anything, "my-bucket", "file.txt").Return(reader, nil)
	req, _ := http.NewRequest("GET", "/download?bucket=my-bucket&filepath=file.txt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, content, w.Body.String())
	assert.Equal(t, "attachment; filename=file.txt", w.Header().Get("Content-Disposition"))
}
func TestHandleSignedURL(t *testing.T) {

	gin.SetMode(gin.TestMode)
	mockStorage := new(MockStorageProvider)
	handler := storage.NewStorageHandler(mockStorage)
	router := gin.Default()
	router.POST("/signed-url", handler.HandleSignedURL)
	expectedURL := "https://storage.googleapis.com/test-bucket/test.txt?token=secret"
	mockStorage.On("GetSignedURL",
		mock.Anything,
		"test-bucket",
		"test.txt",
		15*time.Minute,
	).Return(expectedURL, nil)
	reqBody := models.SignedURLRequest{
		Bucket:            "test-bucket",
		Filepath:          "test.txt",
		DurationInMinutes: 15,
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/signed-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var response models.SignedURLResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, expectedURL, response.SignedURL)
	assert.Equal(t, "test-bucket", response.Bucket)
	mockStorage.AssertExpectations(t)
}
