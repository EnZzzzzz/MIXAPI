FRONTEND_DIR = ./web
BACKEND_DIR = .
BINARY_NAME = mixapi
DOCKER_IMAGE = mixapi
VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")

.PHONY: all build-frontend start-backend build-go build-docker

all: build-frontend build-go build-docker

start-dev: build-frontend start-backend

build-frontend:
	@echo "Building frontend..."
	@cd $(FRONTEND_DIR) && bun install && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(VERSION) bun run build

start-backend:
	@echo "Starting backend dev server..."
	@cd $(BACKEND_DIR) && go run main.go &

build-go:
	@echo "Building Go binary: $(BINARY_NAME) ..."
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_NAME) main.go
	@echo "Build complete: $(BINARY_NAME)"

build-docker:
	@echo "Building Docker Linux binary: $(BINARY_NAME) ..."
	CGO_ENABLED=0 GOOS=linux GOARCH=$(shell go env GOARCH) go build -ldflags="-s -w" -o $(BINARY_NAME) main.go
	@echo "Building Docker image: $(DOCKER_IMAGE) ..."
	docker build -t $(DOCKER_IMAGE) .
	@echo "Docker image build complete: $(DOCKER_IMAGE)"
