FROM golang:1.26.1-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
# Build from the cmd directory
RUN go build -o main ./SampleApp/cmd/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/SampleApp/config.yaml .
COPY --from=builder /app/flags.json .
EXPOSE 8080
CMD ["./main"]
