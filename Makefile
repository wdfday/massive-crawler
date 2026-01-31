.PHONY: build run docker-build docker-up docker-up-build docker-down docker-logs clean help up

# Build locally
build:
	go build -o main ./main.go

# Run locally
run:
	go run ./main.go

# Docker commands
docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-up-build:
	docker-compose up -d --build

# Alias ngắn gọn
up: docker-up-build

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f crawler

docker-restart:
	docker-compose restart crawler

# Build Docker image
docker-image:
	docker build -t us-data-crawler:latest .

# Run Docker container
docker-run:
	docker run --rm \
		--env-file .env \
		-v $(PWD)/data:/app/data \
		-v $(PWD)/indices:/app/indices:ro \
		--network host \
		us-data-crawler:latest

# Clean
clean:
	rm -f main
	docker-compose down -v
	docker rmi us-data-crawler:latest 2>/dev/null || true

# Help
help:
	@echo "Available commands:"
	@echo "  make build          - Build Go binary locally"
	@echo "  make run            - Run Go application locally"
	@echo "  make docker-build   - Build Docker images"
	@echo "  make docker-up      - Start Docker containers"
	@echo "  make docker-up-build - Build and start Docker containers (recommended)"
	@echo "  make up              - Alias cho docker-up-build (ngắn gọn nhất)"
	@echo "  make docker-down    - Stop Docker containers"
	@echo "  make docker-logs    - View crawler logs"
	@echo "  make docker-restart - Restart crawler"
	@echo "  make docker-image   - Build Docker image"
	@echo "  make docker-run     - Run Docker container"
	@echo "  make clean          - Clean build artifacts and Docker resources"
