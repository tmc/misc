FROM golang:1.20-alpine as builder

# Install build dependencies
RUN apk add --no-cache git bash

# Set the working directory
WORKDIR /app

# First clone the repository to get both ctx-src and ctx-src-server
RUN git clone https://github.com/tmc/misc.git
WORKDIR /app/misc/ctx-plugins/ctx-src
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ctx-src .

# Build ctx-src-server
WORKDIR /app/misc/ctx-plugins/ctx-src-server
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ctx-src-server .

# Use the Cloud Run base image which supports gcsfuse
FROM gcr.io/gae-runtimes/buildpacks/google-22/run:latest

# Install required dependencies
RUN apt-get update && \
    apt-get install -y git bash curl gnupg lsb-release ca-certificates && \
    update-ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install gcsfuse
RUN export GCSFUSE_REPO=gcsfuse-`lsb_release -c -s` && \
    echo "deb http://packages.cloud.google.com/apt $GCSFUSE_REPO main" | tee /etc/apt/sources.list.d/gcsfuse.list && \
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - && \
    apt-get update && \
    apt-get install -y gcsfuse && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Copy the compiled binaries from the builder stage
COPY --from=builder /app/misc/ctx-plugins/ctx-src-server/ctx-src-server /usr/local/bin/ctx-src-server
COPY --from=builder /app/misc/ctx-plugins/ctx-src/ctx-src /usr/local/bin/ctx-src
COPY --from=builder /app/misc/ctx-plugins/ctx-src/ctx-src.sh /usr/local/bin/ctx-src.sh

# Create cache directories
RUN mkdir -p /tmp/ctx-src-cache /mnt/ctx-src-cache

# Set the working directory
WORKDIR /app

# Cloud Run will handle the port automatically
EXPOSE 8080

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/ctx-src-server"]

# Use default command arguments for Cloud Run
CMD ["--addr=:8080", "--cache-dir=/tmp/ctx-src-cache"]