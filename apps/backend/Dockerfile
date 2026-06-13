# Build Stage
FROM docker.io/library/golang:1.22-alpine AS builder

WORKDIR /app

# Copy module manifests first so dependency download layers cache well
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application (the main package lives in ./src, not the module root)
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./src

# Run Stage
FROM docker.io/library/alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the pre-built binary from the previous stage
COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]
