# Multi-stage Dockerfile for hiro-aristech-api
# Stage 1: Build the Go application with access to private repositories
# Stage 2: Create minimal production image

# =============================================================================
# BUILD STAGE - Compile Go application
# =============================================================================
FROM golang:1.26.4-alpine3.23 AS builder

RUN apk update && apk add bash git openssh tar wget

# Configure Git for SSH access to private repositories
RUN git config --global url."git@bitbucket.org:".insteadOf "https://bitbucket.org/"

# Copy source code
COPY . /build
WORKDIR /build

# Build the application (no CGO required)
RUN --mount=type=ssh \
    mkdir -p ~/.ssh && \
    ssh-keyscan bitbucket.org >> ~/.ssh/known_hosts && \
    export GOPRIVATE=bitbucket.org/almatoag && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=$VERSION" -o hiro-aristech-api ./cmd

# =============================================================================
# PRODUCTION STAGE - Minimal runtime image
# =============================================================================
FROM alpine:3.23 AS production

# Configure timezone for consistent logging
RUN apk --no-cache add tzdata \
    && cp /usr/share/zoneinfo/Europe/Berlin /etc/localtime \
    && echo "Europe/Berlin" > /etc/timezone
ENV TZ=Europe/Berlin

# Create a non-root user for security
# Running as root is a security risk - if the application is compromised,
# an attacker would have full root privileges. Using a non-privileged user
# limits potential damage from security vulnerabilities.
RUN addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser

# Create app directory and set permissions
WORKDIR /app
RUN chown appuser:appgroup /app

# Copy binary with proper ownership and secure permissions
COPY --from=builder --chown=appuser:appgroup /build/hiro-aristech-api .

# Set secure permissions - remove write permissions for security
# Only read and execute permissions are needed for the binary
RUN chmod 555 /app/hiro-aristech-api

# Switch to non-root user before starting the application
USER appuser

ENTRYPOINT ["./hiro-aristech-api"]
