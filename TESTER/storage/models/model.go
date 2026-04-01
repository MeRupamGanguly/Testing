package models

import "time"

type DownloadRequest struct {
	Bucket   string `form:"bucket" binding:"required"`
	Filepath string `form:"filepath" binding:"required"`
}

type DeleteRequest struct {
	Bucket   string `form:"bucket" binding:"required"`
	Filepath string `form:"filepath" binding:"required"`
}

type SignedURLRequest struct {
	Bucket            string `json:"bucket" binding:"required"`
	Filepath          string `json:"filepath" binding:"required"`
	DurationInMinutes int    `json:"duration_minutes" binding:"required,min=1,max=1440"`
}

type ListRequest struct {
	Bucket string `form:"bucket" binding:"required"`
	Prefix string `form:"prefix"`
}

type UploadResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	Bucket     string `json:"bucket"`
	ObjectName string `json:"object_name"`
}

type DeleteResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	Bucket     string `json:"bucket"`
	ObjectName string `json:"object_name"`
}

type SignedURLResponse struct {
	Success    bool      `json:"success"`
	Bucket     string    `json:"bucket"`
	ObjectName string    `json:"object_name"`
	SignedURL  string    `json:"signed_url"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type ListResponse struct {
	Success bool     `json:"success"`
	Bucket  string   `json:"bucket"`
	Objects []string `json:"objects"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}
