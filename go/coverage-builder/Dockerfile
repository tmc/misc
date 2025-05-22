# Build stage
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go build -o coverage-builder

# Runtime stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates git curl
WORKDIR /app
COPY --from=builder /app/coverage-builder .
COPY --from=builder /app/templates templates/
EXPOSE 8080
CMD ["./coverage-builder"]