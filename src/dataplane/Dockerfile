FROM golang:1.23-bookworm AS builder

WORKDIR /app

# Copy go.mod and go.sum and download dependencies
COPY src/dataplane/go.mod src/dataplane/go.sum ./
RUN go mod download

# Copy the source code
COPY src/dataplane/ .

# Build the Go application statically
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dataplane cmd/dataplane/main.go

# Final stage: minimalist runtime image
FROM scratch

# Copy certificates for HTTPS (optional, but good practice for proxy)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/dataplane .

# Run as non-root user (good practice)
USER 1000:1000

# Strict GC Tuning environment variables
ENV GOMEMLIMIT=1000MiB
ENV GOGC=800

EXPOSE 8080 6060

ENTRYPOINT ["./dataplane"]
