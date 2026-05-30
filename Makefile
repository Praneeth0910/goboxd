.PHONY: build run test integration load lint vet clean help

# Variables
DOCKER_COMPOSE  := docker compose
DOCKER          := docker
GO              := go
GOLANGCI_LINT   := golangci-lint
GOBOXD_IMAGE    := goboxd
CONTAINER_NAME  := goboxd-test

# Colors for output
GREEN  := \033[0;32m
YELLOW := \033[0;33m
NC     := \033[0m # No Color

help:
	@echo "$(GREEN)goboxd Makefile targets:$(NC)"
	@echo "  $(YELLOW)build$(NC)       - Build Docker image"
	@echo "  $(YELLOW)run$(NC)         - Run with docker-compose"
	@echo "  $(YELLOW)test$(NC)        - Run Go unit tests"
	@echo "  $(YELLOW)integration$(NC) - Run integration tests (docker run + curl)"
	@echo "  $(YELLOW)load$(NC)        - Run load test script"
	@echo "  $(YELLOW)lint$(NC)        - Run golangci-lint"
	@echo "  $(YELLOW)vet$(NC)         - Run go vet"
	@echo "  $(YELLOW)clean$(NC)       - Remove binaries and temp files"

# Build Docker image
build:
	@echo "$(GREEN)Building Docker image...$(NC)"
	$(DOCKER) build -t $(GOBOXD_IMAGE) .

# Run with docker-compose
run: build
	@echo "$(GREEN)Starting goboxd...$(NC)"
	$(DOCKER) run --rm -p 8080:8080 -v $(PWD)/languages.yaml:/etc/goboxd/languages.yaml:ro --privileged --cgroupns=host $(GOBOXD_IMAGE)

# Run Go unit tests
test:
	@echo "$(GREEN)Running unit tests...$(NC)"
	$(GO) test -v -race -count=1 ./...

# Integration test: start container, check /healthz endpoint
integration: build
	@echo "$(GREEN)Running integration tests...$(NC)"
	@echo "Starting container..."
	$(DOCKER) run -d \
		--name $(CONTAINER_NAME) \
		-p 18080:8080 \
		--privileged \
		--cgroupns=host \
		--rm \
		$(GOBOXD_IMAGE) > /dev/null
	@echo "Waiting for container to be ready..."
	@sleep 5
	@echo "Testing /healthz endpoint..."
	@if $(DOCKER) run --rm --network host \
		curlimages/curl:latest \
		curl -f http://localhost:18080/healthz > /dev/null 2>&1; then \
		echo "$(GREEN)✓ Health check passed$(NC)"; \
	else \
		echo "$(YELLOW)✗ Health check failed$(NC)"; \
		$(DOCKER) kill $(CONTAINER_NAME) > /dev/null 2>&1 || true; \
		exit 1; \
	fi
	@echo "Running Go integration tests..."
	@GOBOXD_URL=http://localhost:18080 $(GO) test -v -tags=integration ./tests/... || ( \
		echo "$(YELLOW)✗ Integration tests failed$(NC)"; \
		$(DOCKER) kill $(CONTAINER_NAME) > /dev/null 2>&1 || true; \
		exit 1 \
	)
	@echo "$(GREEN)✓ Integration tests passed$(NC)"
	@$(DOCKER) kill $(CONTAINER_NAME) > /dev/null 2>&1 || true
	@exit 0

# Run load test script
load:
	@echo "$(GREEN)Running load tests...$(NC)"
	@if [ -x scripts/load-test.sh ]; then \
		scripts/load-test.sh; \
	else \
		echo "$(YELLOW)Error: scripts/load-test.sh not found or not executable$(NC)"; \
		exit 1; \
	fi

# Run golangci-lint
lint:
	@echo "$(GREEN)Running golangci-lint...$(NC)"
	$(GOLANGCI_LINT) run ./...

# Run go vet
vet:
	@echo "$(GREEN)Running go vet...$(NC)"
	$(GO) vet ./...

# Clean up binaries and temp files
clean:
	@echo "$(GREEN)Cleaning up...$(NC)"
	rm -f bin/goboxd
	rm -f *.out
	rm -rf coverage.*
	$(DOCKER) rmi $(GOBOXD_IMAGE) > /dev/null 2>&1 || true
	$(DOCKER) kill $(CONTAINER_NAME) > /dev/null 2>&1 || true
	find . -name "*.test" -delete
	find . -name ".DS_Store" -delete
	@echo "$(GREEN)✓ Clean complete$(NC)"
