.PHONY: build test vet lint fmt tidy proto check-fixtures sync-fixtures bench docker run-cli run-grpc install-tools

GO ?= go
BUF ?= buf
BACKEND_DIR ?= ../backend
IMAGE ?= alora-agent

build:
	$(GO) build ./...

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

lint:
	golangci-lint run

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

proto:
	$(BUF) generate
	@mv gen/alorav1/proto/*.pb.go gen/alorav1/ && rmdir gen/alorav1/proto

check-fixtures:
	@source="$(BACKEND_DIR)/tests/fixtures/calc_cases.json"; target="tests/fixtures/calc_cases.json"; \
	if [ ! -f "$$source" ] || [ ! -f "$$target" ]; then echo "fixture topilmadi" >&2; exit 1; fi; \
	test "$$(sha256sum "$$source" | awk '{print $$1}')" = "$$(sha256sum "$$target" | awk '{print $$1}')"

sync-fixtures:
	BACKEND_DIR=$(BACKEND_DIR) ./scripts/sync_fixtures.sh

bench:
	$(GO) test -bench=. -benchmem ./...

docker:
	docker build -f docker/Dockerfile -t $(IMAGE) .

run-cli:
	$(GO) run ./cmd/agent -mode cli

run-grpc:
	$(GO) run ./cmd/agent -mode grpc

install-tools:
	$(GO) install github.com/bufbuild/buf/cmd/buf@v1.57.0
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	$(GO) install golang.org/x/vuln/cmd/govulncheck@v1.1.4
