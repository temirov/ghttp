# Stage 1: Build the application
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /ghttp ./cmd/ghttp

# Stage 2: Create the final image
FROM alpine:latest

WORKDIR /

# Copy the binary from the builder stage
COPY --from=builder /ghttp /ghttp

# Expose the default port
EXPOSE 8080

# Set the entrypoint
ENTRYPOINT ["/ghttp"]
