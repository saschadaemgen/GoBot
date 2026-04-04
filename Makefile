.PHONY: build test lint run clean fmt vet

BINARY    := gobot
BUILD_DIR := bin
CMD_DIR   := ./cmd/gobot

# Version injection via ldflags (set VERSION externally or defaults to dev).
VERSION   ?= dev
LDFLAGS   := -ldflags "-X github.com/saschadaemgen/gobot/internal/config.buildVersion=$(VERSION)"

build:
	@echo "Building $(BINARY)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)

run: build
	@echo "Running $(BINARY)..."
	@$(BUILD_DIR)/$(BINARY)

test:
	@echo "Running tests..."
	go test ./... -v -count=1

lint: vet
	@echo "Running staticcheck..."
	@which staticcheck > /dev/null 2>&1 || echo "Install: go install honnef.co/go/tools/cmd/staticcheck@latest"
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || true

vet:
	@echo "Running go vet..."
	go vet ./...

fmt:
	@echo "Formatting..."
	gofmt -s -w .

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
