#!/bin/bash

# Script to install gitwh.service to systemd
# Usage: ./install_gitwh_service.sh

SERVICE_FILE="gitwh.service"
SERVICE_NAME="gitwh"
CONFIG_FILE="config.yaml"
CONFIG_DEST="/etc/gitwh.yaml"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root

#if [[ $EUID -eq 0 ]]; then
#    print_error "This script should not be run as root. It will use sudo when needed."
#    exit 1
#fi

# Check if service file exists
if [[ ! -f "$SERVICE_FILE" ]]; then
    print_error "Service file '$SERVICE_FILE' not found in current directory."
    exit 1
fi

# Detect binary path
BINARY_PATH=$(which gitwh)
if [[ -z "$BINARY_PATH" ]]; then
    print_error "gitwh binary not found in PATH. Please ensure gitwh is installed and in your PATH."
    exit 1
fi

BINARY_DIR=$(dirname "$BINARY_PATH")
print_status "Detected gitwh binary at: $BINARY_PATH"
print_status "Binary directory: $BINARY_DIR"

# Check if config file exists
if [[ ! -f "$CONFIG_FILE" ]]; then
    print_error "Config file '$CONFIG_FILE' not found in current directory."
    exit 1
fi

print_status "Found service file: $SERVICE_FILE"
print_status "Found config file: $CONFIG_FILE"

# Step 1: Create service file with correct binary path and copy to systemd directory
print_status "Creating service file with binary path and copying to /etc/systemd/system/..."
TEMP_SERVICE_FILE="/tmp/${SERVICE_FILE}.tmp"
sed "s|__BINARY_DIR__|${BINARY_DIR}|g" "$SERVICE_FILE" > "$TEMP_SERVICE_FILE"

if sudo cp "$TEMP_SERVICE_FILE" "/etc/systemd/system/$SERVICE_FILE"; then
    print_status "Service file copied successfully with binary path: $BINARY_DIR"
    rm -f "$TEMP_SERVICE_FILE"
else
    print_error "Failed to copy service file"
    rm -f "$TEMP_SERVICE_FILE"
    exit 1
fi

# Step 2: Set proper permissions
print_status "Setting proper permissions..."
if sudo chmod 644 "/etc/systemd/system/$SERVICE_FILE"; then
    print_status "Permissions set successfully"
else
    print_error "Failed to set permissions"
    exit 1
fi

# Step 3: Copy config file to /etc/
print_status "Copying config file to $CONFIG_DEST..."
if sudo cp "$CONFIG_FILE" "$CONFIG_DEST"; then
    print_status "Config file copied successfully"
else
    print_error "Failed to copy config file"
    exit 1
fi

# Set proper permissions for config file
print_status "Setting proper permissions for config file..."
if sudo chmod 644 "$CONFIG_DEST"; then
    print_status "Config file permissions set successfully"
else
    print_error "Failed to set config file permissions"
    exit 1
fi

# Step 4: Reload systemd daemon
print_status "Reloading systemd daemon..."
if sudo systemctl daemon-reload; then
    print_status "Systemd daemon reloaded successfully"
else
    print_error "Failed to reload systemd daemon"
    exit 1
fi

# Step 4: Enable the service
print_status "Enabling $SERVICE_NAME service..."
if sudo systemctl enable "$SERVICE_NAME.service"; then
    print_status "Service enabled successfully"
else
    print_error "Failed to enable service"
    exit 1
fi

# Step 5: Start the service
print_status "Starting $SERVICE_NAME service..."
if sudo systemctl start "$SERVICE_NAME.service"; then
    print_status "Service started successfully"
else
    print_error "Failed to start service"
    exit 1
fi

# Step 6: Check service status
print_status "Checking service status..."
echo "----------------------------------------"
sudo systemctl status "$SERVICE_NAME.service" --no-pager
echo "----------------------------------------"

# Additional information
print_status "Service installation completed!"
echo ""
echo "Useful commands:"
echo "  Check status:    sudo systemctl status $SERVICE_NAME.service"
echo "  Stop service:    sudo systemctl stop $SERVICE_NAME.service"
echo "  Start service:   sudo systemctl start $SERVICE_NAME.service"
echo "  Restart service: sudo systemctl restart $SERVICE_NAME.service"
echo "  View logs:       sudo journalctl -u $SERVICE_NAME.service -f"
echo "  Disable service: sudo systemctl disable $SERVICE_NAME.service"