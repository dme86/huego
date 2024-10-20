# Base image
FROM golang:1.23-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go source code
COPY . .

# Initialize Go modules and download dependencies
RUN go mod init huego && \
    go mod tidy && \
    go mod download

# Build the Go app
RUN go build -o /hue-exporter

# Final image
FROM alpine:latest

# Copy the built binary from the builder stage
COPY --from=builder /hue-exporter /usr/local/bin/hue-exporter

# Expose the port
EXPOSE 8000

# Set the entrypoint to the binary
ENTRYPOINT ["/usr/local/bin/hue-exporter"]

