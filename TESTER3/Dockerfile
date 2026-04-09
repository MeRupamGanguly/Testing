# Change from 1.23-alpine to 1.26-alpine (or whichever version is in your go.mod)
FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY ldkillswitch/ ./ldkillswitch/
COPY webapp/ ./webapp/

WORKDIR /app/webapp
RUN go mod download
RUN go build -o /main ./cmd/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /main .
COPY --from=builder /app/webapp/config.yaml .
EXPOSE 8080
CMD ["./main"]
