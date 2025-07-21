FROM golang:1.21-alpine AS builder

# Install dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o chrome-to-har-docker ./examples/11-integration/docker-example.go

FROM alpine:latest

# Install Chrome dependencies
RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    harfbuzz \
    ca-certificates \
    ttf-freefont

# Create non-root user
RUN addgroup -g 1000 -S chrome && \
    adduser -u 1000 -S chrome -G chrome

WORKDIR /app
COPY --from=builder /app/chrome-to-har-docker .

# Set Chrome path
ENV CHROME_PATH=/usr/bin/chromium-browser

USER chrome

ENTRYPOINT ["./chrome-to-har-docker"]