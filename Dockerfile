# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/dxp-verifier ./cmd/verifier

# Final stage
FROM alpine:latest

# Add ca certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/dxp-verifier .

# Copy the .env file
COPY .env .

# Create the directory structure for dashboard files
RUN mkdir -p pkg/dashboard/templates pkg/dashboard/static

# Copy the dashboard templates and static files to the correct location
COPY pkg/dashboard/templates pkg/dashboard/templates
COPY pkg/dashboard/static pkg/dashboard/static

# Expose necessary ports (adjust based on your application)
EXPOSE 8080

# Command to run the executable
CMD ["./dxp-verifier", "dashboard"]