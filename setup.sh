#!/bin/bash

# Mukhostho DB Setup and Build Script

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Configuration
PROJECT_NAME="mukhostho-db"
GO_VERSION="1.21"
BUILD_DIR="build"
DOCS_DIR="docs"

# Function to print colored output
print_header() {
    echo -e "${PURPLE}=================================${NC}"
    echo -e "${PURPLE}$1${NC}"
    echo -e "${PURPLE}=================================${NC}"
}

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check system requirements
check_requirements() {
    print_header "Checking System Requirements"
    
    # Check Go version
    if command -v go &> /dev/null; then
        GO_CURRENT_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        print_status "Found Go version: $GO_CURRENT_VERSION"
        
        # Simple version comparison (assuming semantic versioning)
        if [[ "$(printf '%s\n' "$GO_VERSION" "$GO_CURRENT_VERSION" | sort -V | head -n1)" != "$GO_VERSION" ]]; then
            print_warning "Go version $GO_VERSION or higher is recommended"
        fi
    else
        print_error "Go is not installed. Please install Go $GO_VERSION or higher."
        print_status "Visit: https://golang.org/doc/install"
        exit 1
    fi
    
    # Check git
    if ! command -v git &> /dev/null; then
        print_error "Git is not installed. Please install Git."
        exit 1
    fi
    
    # Check make
    if ! command -v make &> /dev/null; then
        print_warning "Make is not installed. Some build commands may not work."
    fi
    
    # Check curl
    if ! command -v curl &> /dev/null; then
        print_warning "curl is not installed. API testing may not work."
    fi
    
    print_success "System requirements check completed"
}

# Initialize project
init_project() {
    print_header "Initializing Project"
    
    # Create necessary directories
    print_status "Creating project directories..."
    mkdir -p "$BUILD_DIR"
    mkdir -p "$DOCS_DIR"
    mkdir -p "data"
    mkdir -p "logs"
    mkdir -p "scripts"
    mkdir -p "examples"
    mkdir -p "test"
    
    # Initialize Go module if not exists
    if [ ! -f "go.mod" ]; then
        print_status "Initializing Go module..."
        go mod init github.com/dark-elixir/mukhostho-db
    fi
    
    print_success "Project initialization completed"
}

# Install dependencies
install_dependencies() {
    print_header "Installing Dependencies"
    
    print_status "Downloading Go dependencies..."
    go mod download
    go mod tidy
    
    print_success "Dependencies installed successfully"
}

# Run tests
run_tests() {
    print_header "Running Tests"
    
    print_status "Running unit tests..."
    go test -v ./test ./internal/...
    
    print_status "Running tests with race detection..."
    go test -race ./test ./internal/...
    
    print_status "Running benchmarks..."
    go test -bench=. -benchmem ./test
    
    print_success "All tests completed successfully"
}

# Build application
build_application() {
    print_header "Building Application"
    
    print_status "Building main application..."
    go build -ldflags "-s -w" -o "$BUILD_DIR/mukhostho" cmd/mukhostho/main.go
    
    print_status "Building client example..."
    go build -o "$BUILD_DIR/client_example" examples/client_example.go
    
    # Make scripts executable
    chmod +x scripts/*.sh 2>/dev/null || true
    
    print_success "Build completed successfully"
}

# Generate documentation
generate_docs() {
    print_header "Generating Documentation"
    
    print_status "Generating Go documentation..."
    if command -v godoc &> /dev/null; then
        echo "Documentation server can be started with: godoc -http=:6060"
        echo "Then visit: http://localhost:6060/pkg/github.com/dark-elixir/mukhostho-db/"
    else
        print_warning "godoc not installed. Install with: go install golang.org/x/tools/cmd/godoc@latest"
    fi
    
    print_status "API documentation is available in README.md"
    
    print_success "Documentation generation completed"
}

# Setup development environment
setup_dev_environment() {
    print_header "Setting Up Development Environment"
    
    # Install development tools
    print_status "Installing development tools..."
    
    # Air for live reloading
    if ! command -v air &> /dev/null; then
        print_status "Installing air for live reloading..."
        go install github.com/cosmtrek/air@latest
    fi
    
    # golangci-lint for linting
    if ! command -v golangci-lint &> /dev/null; then
        print_status "Installing golangci-lint..."
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2
    fi
    
    # gosec for security analysis
    if ! command -v gosec &> /dev/null; then
        print_status "Installing gosec..."
        go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
    fi
    
    # godoc for documentation
    if ! command -v godoc &> /dev/null; then
        print_status "Installing godoc..."
        go install golang.org/x/tools/cmd/godoc@latest
    fi
    
    # Create air configuration if it doesn't exist
    if [ ! -f ".air.toml" ]; then
        print_status "Creating air configuration..."
        cat > .air.toml << 'EOF'
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main cmd/mukhostho/main.go"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "build", "data"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_root = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
EOF
    fi
    
    print_success "Development environment setup completed"
}

# Run code quality checks
quality_checks() {
    print_header "Running Code Quality Checks"
    
    # Format code
    print_status "Formatting code..."
    go fmt ./...
    
    # Run linter
    if command -v golangci-lint &> /dev/null; then
        print_status "Running linter..."
        golangci-lint run
    else
        print_warning "golangci-lint not found, skipping linting"
    fi
    
    # Run security check
    if command -v gosec &> /dev/null; then
        print_status "Running security analysis..."
        gosec ./...
    else
        print_warning "gosec not found, skipping security analysis"
    fi
    
    print_success "Code quality checks completed"
}

# Create systemd service file
create_systemd_service() {
    print_header "Creating Systemd Service"
    
    cat > mukhostho-db.service << EOF
[Unit]
Description=Mukhostho DB - High-Performance In-Memory Database
After=network.target

[Service]
Type=simple
User=mukhostho
Group=mukhostho
WorkingDirectory=/opt/mukhostho-db
ExecStart=/opt/mukhostho-db/mukhostho
ExecReload=/bin/kill -HUP \$MAINPID
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

# Security settings
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/opt/mukhostho-db/data

[Install]
WantedBy=multi-user.target
EOF
    
    print_status "Systemd service file created: mukhostho-db.service"
    print_status "To install the service:"
    print_status "  sudo cp mukhostho-db.service /etc/systemd/system/"
    print_status "  sudo systemctl daemon-reload"
    print_status "  sudo systemctl enable mukhostho-db"
    print_status "  sudo systemctl start mukhostho-db"
    
    print_success "Systemd service creation completed"
}

# Package for distribution
package_distribution() {
    print_header "Creating Distribution Package"
    
    VERSION=$(date +%Y%m%d%H%M%S)
    PACKAGE_NAME="${PROJECT_NAME}-${VERSION}"
    PACKAGE_DIR="$BUILD_DIR/$PACKAGE_NAME"
    
    print_status "Creating package directory..."
    mkdir -p "$PACKAGE_DIR"
    
    # Copy binary
    cp "$BUILD_DIR/mukhostho" "$PACKAGE_DIR/"
    
    # Copy configuration
    cp config.json "$PACKAGE_DIR/"
    
    # Copy documentation
    cp README.md "$PACKAGE_DIR/"
    cp ARCHITECTURE.md "$PACKAGE_DIR/" 2>/dev/null || true
    
    # Copy examples
    mkdir -p "$PACKAGE_DIR/examples"
    cp examples/* "$PACKAGE_DIR/examples/" 2>/dev/null || true
    
    # Copy scripts
    mkdir -p "$PACKAGE_DIR/scripts"
    cp scripts/* "$PACKAGE_DIR/scripts/" 2>/dev/null || true
    
    # Copy systemd service
    cp mukhostho-db.service "$PACKAGE_DIR/" 2>/dev/null || true
    
    # Create installation script
    cat > "$PACKAGE_DIR/install.sh" << 'EOF'
#!/bin/bash
# Mukhostho DB Installation Script

set -e

print_status() {
    echo "[INFO] $1"
}

print_success() {
    echo "[SUCCESS] $1"
}

print_error() {
    echo "[ERROR] $1"
}

# Default installation directory
INSTALL_DIR="/opt/mukhostho-db"
DATA_DIR="/var/lib/mukhostho-db"
LOG_DIR="/var/log/mukhostho-db"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --data-dir)
            DATA_DIR="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [--install-dir DIR] [--data-dir DIR]"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

print_status "Installing Mukhostho DB to $INSTALL_DIR"

# Create user and group
if ! id "mukhostho" &>/dev/null; then
    print_status "Creating mukhostho user..."
    sudo useradd -r -s /bin/false mukhostho
fi

# Create directories
print_status "Creating directories..."
sudo mkdir -p "$INSTALL_DIR"
sudo mkdir -p "$DATA_DIR"
sudo mkdir -p "$LOG_DIR"

# Copy files
print_status "Copying files..."
sudo cp mukhostho "$INSTALL_DIR/"
sudo cp config.json "$INSTALL_DIR/"
sudo cp -r examples "$INSTALL_DIR/" 2>/dev/null || true
sudo cp -r scripts "$INSTALL_DIR/" 2>/dev/null || true

# Set permissions
print_status "Setting permissions..."
sudo chown -R mukhostho:mukhostho "$INSTALL_DIR"
sudo chown -R mukhostho:mukhostho "$DATA_DIR"
sudo chown -R mukhostho:mukhostho "$LOG_DIR"
sudo chmod +x "$INSTALL_DIR/mukhostho"

# Install systemd service
if [ -f "mukhostho-db.service" ] && command -v systemctl &>/dev/null; then
    print_status "Installing systemd service..."
    sudo cp mukhostho-db.service /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl enable mukhostho-db
    print_status "Service installed. Start with: sudo systemctl start mukhostho-db"
fi

print_success "Installation completed!"
print_status "Binary installed to: $INSTALL_DIR/mukhostho"
print_status "Data directory: $DATA_DIR"
print_status "Log directory: $LOG_DIR"
EOF
    chmod +x "$PACKAGE_DIR/install.sh"
    
    # Create archive
    print_status "Creating archive..."
    cd "$BUILD_DIR"
    tar -czf "${PACKAGE_NAME}.tar.gz" "$PACKAGE_NAME"
    cd ..
    
    print_success "Distribution package created: $BUILD_DIR/${PACKAGE_NAME}.tar.gz"
}

# Main function
main() {
    case "${1:-all}" in
        "requirements")
            check_requirements
            ;;
        "init")
            init_project
            ;;
        "deps")
            install_dependencies
            ;;
        "test")
            run_tests
            ;;
        "build")
            build_application
            ;;
        "docs")
            generate_docs
            ;;
        "dev")
            setup_dev_environment
            ;;
        "quality")
            quality_checks
            ;;
        "service")
            create_systemd_service
            ;;
        "package")
            package_distribution
            ;;
        "all")
            check_requirements
            init_project
            install_dependencies
            quality_checks
            run_tests
            build_application
            generate_docs
            create_systemd_service
            package_distribution
            ;;
        "help")
            echo "Mukhostho DB Setup Script"
            echo "Usage: $0 [command]"
            echo ""
            echo "Commands:"
            echo "  requirements  - Check system requirements"
            echo "  init          - Initialize project structure"
            echo "  deps          - Install dependencies"
            echo "  test          - Run all tests"
            echo "  build         - Build application"
            echo "  docs          - Generate documentation"
            echo "  dev           - Setup development environment"
            echo "  quality       - Run code quality checks"
            echo "  service       - Create systemd service file"
            echo "  package       - Create distribution package"
            echo "  all           - Run all steps (default)"
            echo "  help          - Show this help"
            exit 0
            ;;
        *)
            print_error "Unknown command: $1"
            echo "Use '$0 help' for usage information"
            exit 1
            ;;
    esac
    
    print_success "Setup script completed successfully!"
}

# ASCII Art Banner
echo -e "${PURPLE}"
cat << 'EOF'
 __  __       _    _               _   _           ____  ____  
|  \/  |_   _| | _| |__   ___  ___| |_| |__   ___ |  _ \| __ ) 
| |\/| | | | | |/ / '_ \ / _ \/ __| __| '_ \ / _ \| | | |  _ \ 
| |  | | |_| |   <| | | | (_) \__ \ |_| | | | (_) | |_| | |_) |
|_|  |_|\__,_|_|\_\_| |_|\___/|___/\__|_| |_|\___/|____/|____/ 
                                                               
           High-Performance In-Memory Database
EOF
echo -e "${NC}"

# Run main function with all arguments
main "$@"
