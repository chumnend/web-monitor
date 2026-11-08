all: build start

.PHONY: build
build:
	@echo "Building application..."
	@mkdir -p bin/
	@go build -o bin/api cmd/api/main.go
	@go build -o bin/worker cmd/worker/main.go
	@echo "App built."

.PHONY: start
start:
	@echo "Executing API..."
	@./bin/api

.PHONY: start-worker
start-worker:
	@echo "Executing worker..."
	@./bin/worker

.PHONY: test
test:
	@echo "Testing internal/..."
	@go test -cover ./internal/...

.PHONY: clean
clean:
	@echo "Cleaning binaries..."
	@rm -rf bin
	@echo "Clean complete."

.PHONY: build-migrations
build-migrations:
	@echo "Building migration tools..."
	@go build -o bin/migrate-up cmd/migrate-up/main.go
	@go build -o bin/migrate-down cmd/migrate-down/main.go
	@echo "Migration tools built."

.PHONY: migrate-up
migrate-up: build-migrations
	@echo "Applying database migrations (up)..."
	@./bin/migrate-up
	@echo "Migrations up complete."

.PHONY: migrate-down
migrate-down: build-migrations
	@echo "Rolling back database migrations (down)..."
	@./bin/migrate-down
	@echo "Migrations down complete."