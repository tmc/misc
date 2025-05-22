#!/bin/bash
# Installation script for ctx-src-server
set -e

# Default installation directory
INSTALL_DIR="/opt/ctx-src-server"
CACHE_DIR="/var/cache/ctx-src"
SERVICE_USER="ctx-src"
USE_GCSFUSE=false
GCS_BUCKET=""

# Parse command-line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --cache-dir)
            CACHE_DIR="$2"
            shift 2
            ;;
        --user)
            SERVICE_USER="$2"
            shift 2
            ;;
        --use-gcsfuse)
            USE_GCSFUSE=true
            shift
            ;;
        --gcs-bucket)
            GCS_BUCKET="$2"
            USE_GCSFUSE=true
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Check for root permissions
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit 1
fi

echo "Installing ctx-src-server..."
echo "Installation directory: $INSTALL_DIR"
echo "Cache directory: $CACHE_DIR"
echo "Service user: $SERVICE_USER"
echo "Use GCSFuse: $USE_GCSFUSE"
if [ "$USE_GCSFUSE" = true ]; then
    echo "GCS Bucket: $GCS_BUCKET"
fi

# Create service user if it doesn't exist
if ! id -u "$SERVICE_USER" &>/dev/null; then
    echo "Creating service user: $SERVICE_USER"
    useradd -r -s /sbin/nologin -d "$INSTALL_DIR" "$SERVICE_USER"
fi

# Create installation and cache directories
mkdir -p "$INSTALL_DIR" "$CACHE_DIR"
chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR" "$CACHE_DIR"

# Install dependencies
echo "Installing dependencies..."
if [ -f /etc/debian_version ]; then
    # Debian/Ubuntu
    apt-get update
    apt-get install -y git curl fuse
    
    if [ "$USE_GCSFUSE" = true ]; then
        # Install gcsfuse
        export GCSFUSE_REPO=gcsfuse-`lsb_release -c -s`
        echo "deb http://packages.cloud.google.com/apt $GCSFUSE_REPO main" | tee /etc/apt/sources.list.d/gcsfuse.list
        curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
        apt-get update
        apt-get install -y gcsfuse
    fi
elif [ -f /etc/redhat-release ]; then
    # RHEL/CentOS/Fedora
    yum install -y git curl fuse
    
    if [ "$USE_GCSFUSE" = true ]; then
        # Install gcsfuse
        tee /etc/yum.repos.d/gcsfuse.repo > /dev/null <<EOT
[gcsfuse]
name=gcsfuse (packages.cloud.google.com)
baseurl=https://packages.cloud.google.com/yum/repos/gcsfuse-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
EOT
        yum install -y gcsfuse
    fi
else
    echo "Unsupported distribution"
    exit 1
fi

# Download and build the binaries
echo "Building ctx-src-server..."
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

# Clone the repository
git clone https://github.com/tmc/misc.git
cd misc/ctx-plugins

# Build ctx-src
cd ctx-src
go build -o ctx-src .
cp ctx-src "$INSTALL_DIR/"
cp ctx-src.sh "$INSTALL_DIR/"

# Build ctx-src-server
cd ../ctx-src-server
go build -o ctx-src-server .
cp ctx-src-server "$INSTALL_DIR/"

# Clean up temporary directory
cd /
rm -rf "$TEMP_DIR"

# Create systemd service
echo "Installing systemd service..."
SYSTEMD_SERVICE="/etc/systemd/system/ctx-src-server.service"

# Configure service file
cat > "$SYSTEMD_SERVICE" <<EOT
[Unit]
Description=ctx-src-server
After=network.target

[Service]
User=$SERVICE_USER
Group=$SERVICE_USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/ctx-src-server --addr=:8080 --cache-dir=$CACHE_DIR --verbose
Restart=on-failure
RestartSec=5
EOT

# Add gcsfuse configuration if needed
if [ "$USE_GCSFUSE" = true ]; then
    if [ -z "$GCS_BUCKET" ]; then
        echo "Error: GCS bucket name must be provided with --use-gcsfuse"
        exit 1
    fi
    
    # Update service file for gcsfuse
    sed -i "/ExecStart=/c\\ExecStart=$INSTALL_DIR/ctx-src-server --addr=:8080 --gcs-bucket=$GCS_BUCKET --verbose" "$SYSTEMD_SERVICE"
    
    # Add required capabilities for FUSE
    cat >> "$SYSTEMD_SERVICE" <<EOT
CapabilityBoundingSet=CAP_SYS_ADMIN
AmbientCapabilities=CAP_SYS_ADMIN
EOT
fi

# Complete the service file
cat >> "$SYSTEMD_SERVICE" <<EOT

[Install]
WantedBy=multi-user.target
EOT

# Enable and start the service
systemctl daemon-reload
systemctl enable ctx-src-server
systemctl start ctx-src-server

echo "Installation complete!"
echo "ctx-src-server is now running at http://localhost:8080"
echo "To check status: systemctl status ctx-src-server"