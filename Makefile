.PHONY: build install uninstall clean test fmt vet check help

# Binary name and directories
BINARY_NAME=youtube-manager
CMD_DIR=cmd/$(BINARY_NAME)
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet
GOMOD=$(GOCMD) mod

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "✅ Build complete! Binary: $(BUILD_DIR)/$(BINARY_NAME)"

# Install binary to /usr/local/bin or TARGET directory
install: build
ifndef TARGET
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "✅ Installation complete!"
else
	@echo "Installing $(BINARY_NAME) to $(TARGET)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(TARGET)/ 2>/dev/null || sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(TARGET)/
	@echo "✅ Installation complete!"
endif

# Uninstall binary from system
uninstall:
	@echo "Looking for $(BINARY_NAME) in system..."
	@BINARY_PATH=$$(which $(BINARY_NAME) 2>/dev/null); \
	if [ -z "$$BINARY_PATH" ]; then \
		echo "$(BINARY_NAME) not found in PATH"; \
		exit 0; \
	fi; \
	if [ -f "$$BINARY_PATH" ]; then \
		echo "Found $(BINARY_NAME) at $$BINARY_PATH"; \
		echo "Removing..."; \
		sudo rm -f "$$BINARY_PATH"; \
		echo "✅ Uninstallation complete!"; \
	fi

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "✅ Clean complete!"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "✅ Dependencies updated!"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...
	@echo "✅ Tests complete!"

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...
	@echo "✅ Format complete!"

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "✅ Vet complete!"

# Run all checks
check: fmt vet test
	@echo "✅ All checks passed!"

# Show help
help:
	@echo "Available targets:"
	@echo "  build      - Build the binary"
	@echo "  install    - Build and install to /usr/local/bin (or TARGET env variable)"
	@echo "  uninstall  - Remove installed binary"
	@echo "  clean      - Remove build artifacts"
	@echo "  deps       - Download and tidy dependencies"
	@echo "  test       - Run tests"
	@echo "  fmt        - Format code"
	@echo "  vet        - Run go vet"
	@echo "  check      - Run fmt, vet, and test"
	@echo "  help       - Show this help message"
