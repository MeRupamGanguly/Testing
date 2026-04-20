FROM golang:1.26.1-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
# Build the binary from the specified path
RUN go build -o main ./SampleApp/cmd/main.go

FROM alpine:latest
WORKDIR /app

# Create a nested structure so ../../flags.json points to /app/flags.json
# Path structure:
# /app/flags.json
# /app/bin/service/main
RUN mkdir -p /app/bin/service

# Copy the binary and config into the deep subdirectory
COPY --from=builder /app/main /app/bin/service/main
COPY --from=builder /app/SampleApp/config.yaml /app/bin/service/config.yaml

# Copy flags.json to the root /app directory
COPY --from=builder /app/flags.json /app/flags.json

# Set execution context to the binary's directory
WORKDIR /app/bin/service
EXPOSE 8080
CMD ["./main"]
