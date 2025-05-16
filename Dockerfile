# Step - 1: Build the application
# --------------------------------------------------------
# Use the official Go image for building the application
FROM golang:1.24.2 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download the Go module dependencies
RUN go mod download

# Copy the source code into the container
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/

# Build the solo-cheetah binary with minimal flags
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ./cheetah -v ./cmd/cheetah

# Create a non-root user
ENV USER=appuser
ENV UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"

# Step - 2: Create a minimal image
# --------------------------------------------------------
# Use a minimal base image for the final container
FROM scratch

# Set the working directory inside the container
WORKDIR /app

# Copy the minimal /etc/passwd file to set up the non-root user
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Set the user to the non-root user
USER appuser:appuser

# Copy the built binary from the builder stage
COPY --from=builder /app/cheetah .

# Define volumes for data and config
VOLUME ["/app/data", "/app/config", "/app/logs", "/app/stats"]

EXPOSE 6060

ENTRYPOINT ["/app/cheetah"]
