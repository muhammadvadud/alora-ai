.PHONY: run cli http dev build test vet fmt tidy lint clean docker install-tools

# Agentni ishga tushirish (config'dagi mode bo'yicha)
run:
	go run ./cmd/agent

# CLI rejim (interaktiv terminal)
cli:
	go run ./cmd/agent -mode cli

# HTTP rejim (API server)
http:
	go run ./cmd/agent -mode http

# Hot reload bilan ishlab chiqish (Air kerak: make install-tools)
dev:
	air

# Binarni qurish
build:
	go build -o ./bin/agent ./cmd/agent

# Testlar
test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

lint:
	golangci-lint run ./...

clean:
	rm -rf bin tmp

# Docker image qurish
docker:
	docker build -t baholash-agent .

# Ishlab chiqish asboblari (Air hot reload, golangci-lint)
install-tools:
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
