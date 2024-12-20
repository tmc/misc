FROM ubuntu:22.04

# Install required packages
RUN apt-get update && apt-get install -y \
    numactl \
    pciutils \
    hwloc \
    libvirt-clients \
    libxml2-utils \
    python3 \
    python3-pip \
    bc \
    curl \
    git \
    gcc \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install Go (using a specific version known to work)
RUN curl -L https://go.dev/dl/go1.21.5.linux-amd64.tar.gz | tar -C /usr/local -xzf - && \
    echo 'export PATH=$PATH:/usr/local/go/bin:/root/go/bin' >> /root/.bashrc

# Set Go environment variables
ENV PATH=$PATH:/usr/local/go/bin:/root/go/bin
ENV GOPATH=/root/go
ENV CGO_ENABLED=1

# Install cgpt
RUN go install github.com/tmc/cgpt/cmd/cgpt@latest

# Copy mock scripts
COPY test/docker/mock/ /usr/local/bin/
RUN chmod +x /usr/local/bin/mock-*

# Set up mock device structure
RUN mkdir -p \
    /sys/devices/pci0000:00 \
    /sys/bus/pci/devices \
    /sys/kernel/mm/ksm \
    /proc/sys/vm

WORKDIR /app

# Verify installations
RUN go version && \
    cgpt --version

