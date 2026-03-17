.PHONY: build run test lint docker-up docker-down seed loadtest vet fmt

# Build the server binary (requires libvips-dev)
build:
	CGO_ENABLED=1 go build -o bin/server ./cmd/server

# Run the server locally (requires MinIO + Redis running)
run: build
	./bin/server

# Run all tests (short mode skips integration tests)
test:
	go test ./... -v

# Run only unit tests (no external dependencies needed)
test-unit:
	go test ./... -short -v

# Lint: go vet + format check
lint: vet fmt

vet:
	go vet ./...

fmt:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

# Docker Compose
docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

# Seed sample images (server must be running)
seed:
	bash scripts/seed.sh

# Run load tests (server must be running)
loadtest:
	bash scripts/loadtest.sh
