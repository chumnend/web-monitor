all: build-frontend build start

.PHONY: build-frontend
build-frontend:
	@echo "Building frontend..."
	@cd frontend && yarn install
	@cd frontend && yarn build
	@mkdir -p bin/
	@cp -r frontend/dist/* bin/
	@echo "Frontend build complete."

.PHONY: build
build:
	@echo "Building backend..."
	@cd ./bin && go build ../cmd/main.go
	@echo "Backend build complete."

.PHONY: start
start:
	@echo "Executing..."
	@./bin/main

.PHONY: clean
clean:
	@echo "Cleaning binaries..."
	@rm -rf bin
	@echo "Cleaning complete."