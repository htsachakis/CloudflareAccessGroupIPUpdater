FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum* ./

# Download dependencies if go.sum exists
RUN if [ -f go.sum ]; then go mod download; fi

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o cloudflare-access-group-ip-updater

# Create a minimal image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/cloudflare-access-group-ip-updater .

# Run the application
CMD ["./cloudflare-access-group-ip-updater"]

LABEL org.opencontainers.image.source=https://github.com/htsachakis/cloudflare-access-group-ip-updater
LABEL org.opencontainers.image.description="Cloudflare Access Group IP Updater"
LABEL org.opencontainers.image.licenses=MIT