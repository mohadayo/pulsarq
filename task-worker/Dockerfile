FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod .
COPY *.go .
RUN go build -o task-worker .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/task-worker .

EXPOSE 5001
CMD ["./task-worker"]
