all: build start

.PHONY: build
build:
	@echo "Building..."
	@mkdir -p bin/
	@cd ./bin && go build ../cmd/main.go
	@echo "Build complete."

.PHONY: start
start:
	@echo "Executing..."
	@./bin/main

.PHONY: test
test:
	@echo "Testing cmd/..."
	@go test ./cmd/

.PHONY: clean
clean:
	@echo "Cleaning binaries..."
	@rm -rf bin
	@echo "Clean complete."

.PHONY: build-migrations
build-migrations:
	@echo "Building migration tools..."
	@go build -o bin/migrate-up cmd/up/main.go
	@go build -o bin/migrate-down cmd/down/main.go
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