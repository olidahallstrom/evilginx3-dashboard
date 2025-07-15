#!/bin/bash

# Evilginx2 Ubuntu Setup Script
# This script ensures proper setup and compatibility on Ubuntu VPS

set -e  # Exit on any error

echo "🐧 Evilginx2 Ubuntu VPS Setup Script"
echo "======================================"

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
if [[ $EUID -eq 0 ]]; then
   print_error "This script should not be run as root for security reasons"
   print_warning "Please run as a regular user with sudo privileges"
   exit 1
fi

# Check Ubuntu version
print_status "Checking Ubuntu version..."
if [ -f /etc/os-release ]; then
    . /etc/os-release
    echo "OS: $NAME $VERSION"
    
    # Check if it's Ubuntu
    if [[ "$ID" != "ubuntu" ]]; then
        print_warning "This script is designed for Ubuntu. Your OS: $ID"
        print_warning "Proceeding anyway, but some steps might need adjustment"
    fi
else
    print_warning "Cannot detect OS version. Proceeding with Ubuntu assumptions."
fi

# Update system packages
print_status "Updating system packages..."
sudo apt update

# Install required system dependencies
print_status "Installing system dependencies..."
sudo apt install -y \
    curl \
    wget \
    git \
    build-essential \
    ca-certificates \
    software-properties-common \
    ufw \
    net-tools

# Check if Go is installed and version
print_status "Checking Go installation..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    print_status "Go version: $GO_VERSION"
    
    # Check if Go version is 1.22 or higher
    if [[ $(echo "$GO_VERSION" | cut -d. -f1) -ge 1 ]] && [[ $(echo "$GO_VERSION" | cut -d. -f2) -ge 22 ]]; then
        print_status "Go version is compatible (1.22+)"
    else
        print_warning "Go version $GO_VERSION might be too old. Recommended: 1.22+"
        print_status "Installing latest Go..."
        
        # Remove old Go installation
        sudo rm -rf /usr/local/go
        
        # Download and install latest Go
        GO_LATEST=$(curl -s https://golang.org/VERSION?m=text)
        wget -q "https://golang.org/dl/${GO_LATEST}.linux-amd64.tar.gz"
        sudo tar -C /usr/local -xzf "${GO_LATEST}.linux-amd64.tar.gz"
        rm "${GO_LATEST}.linux-amd64.tar.gz"
        
        # Update PATH
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
        export PATH=$PATH:/usr/local/go/bin
        
        print_status "Go updated to: $(go version)"
    fi
else
    print_status "Go not found. Installing Go..."
    
    # Download and install Go
    GO_LATEST=$(curl -s https://golang.org/VERSION?m=text)
    wget -q "https://golang.org/dl/${GO_LATEST}.linux-amd64.tar.gz"
    sudo tar -C /usr/local -xzf "${GO_LATEST}.linux-amd64.tar.gz"
    rm "${GO_LATEST}.linux-amd64.tar.gz"
    
    # Update PATH
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    export PATH=$PATH:/usr/local/go/bin
    
    print_status "Go installed: $(go version)"
fi

# Check if we're in the correct directory
if [ ! -f "go.mod" ] || [ ! -f "main.go" ]; then
    print_error "Please run this script from the Evilginx2 project directory"
    print_error "Current directory: $(pwd)"
    print_error "Expected files: go.mod, main.go"
    exit 1
fi

# Check Go module
print_status "Checking Go module..."
if grep -q "github.com/kgretzky/evilginx2" go.mod; then
    print_status "Go module verified"
else
    print_error "Invalid Go module. Expected evilginx2 module."
    exit 1
fi

# Download dependencies
print_status "Downloading Go dependencies..."
go mod download
go mod tidy

# Build the project
print_status "Building Evilginx2..."
if go build -o evilginx2 .; then
    print_status "Build successful!"
else
    print_error "Build failed!"
    exit 1
fi

# Make scripts executable
print_status "Making scripts executable..."
chmod +x evilginx2
if [ -f "start_with_dashboard.sh" ]; then
    chmod +x start_with_dashboard.sh
fi

# Check network configuration
print_status "Checking network configuration..."
if command -v ufw &> /dev/null; then
    print_status "UFW firewall detected"
    print_warning "Make sure to configure UFW to allow necessary ports:"
    print_warning "  - Dashboard: sudo ufw allow 8080"
    print_warning "  - HTTP: sudo ufw allow 80"
    print_warning "  - HTTPS: sudo ufw allow 443"
    print_warning "  - DNS: sudo ufw allow 53"
fi

# Check if port 8080 is available
if netstat -tuln | grep -q ":8080 "; then
    print_warning "Port 8080 is already in use. Dashboard might not start."
    print_warning "Use a different port: ./evilginx2 -dashboard 8081"
fi

# Create systemd service file (optional)
print_status "Creating systemd service file..."
cat > evilginx2.service << EOF
[Unit]
Description=Evilginx2 Phishing Framework
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$(pwd)
ExecStart=$(pwd)/evilginx2 -dashboard 8080
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

print_status "Systemd service file created: evilginx2.service"
print_warning "To install as system service:"
print_warning "  sudo cp evilginx2.service /etc/systemd/system/"
print_warning "  sudo systemctl daemon-reload"
print_warning "  sudo systemctl enable evilginx2"
print_warning "  sudo systemctl start evilginx2"

# Final checks
print_status "Running final compatibility checks..."

# Check if binary works
if ./evilginx2 -v &> /dev/null; then
    print_status "Binary execution test: PASSED"
else
    print_error "Binary execution test: FAILED"
    exit 1
fi

# Check required directories
if [ ! -d "phishlets" ]; then
    print_warning "phishlets directory not found. Creating..."
    mkdir -p phishlets
fi

if [ ! -d "redirectors" ]; then
    print_warning "redirectors directory not found. Creating..."
    mkdir -p redirectors
fi

print_status "Setup completed successfully!"
echo ""
echo "🚀 Quick Start:"
echo "   1. Start with dashboard: ./evilginx2 -dashboard 8080"
echo "   2. Or use convenience script: ./start_with_dashboard.sh"
echo "   3. Access dashboard at: http://YOUR_VPS_IP:8080"
echo ""
echo "📋 Important Notes:"
echo "   - Configure firewall to allow ports 80, 443, 53, and 8080"
echo "   - Run as non-root user for security"
echo "   - Dashboard works alongside terminal interface"
echo "   - Check DASHBOARD_README.md for detailed usage"
echo ""
echo "🔧 Troubleshooting:"
echo "   - If port 8080 is busy: ./evilginx2 -dashboard 8081"
echo "   - For permission issues: check file ownership"
echo "   - For network issues: verify firewall settings"
echo ""
print_status "Ready to run on Ubuntu VPS! 🎉" 