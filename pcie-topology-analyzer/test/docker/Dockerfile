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
    && rm -rf /var/lib/apt/lists/*

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

