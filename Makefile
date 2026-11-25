ENV_FILE := .env
LOCAL_ENV_FILE := .tripgo/env.local
RUNTIME_ENV_FILE := .tripgo/env.runtime
IMAGE_NAME := tripgo-app
CONTAINER_NAME := tripgo_app
PORT := 8080

.PHONY: generate run migrate migrate-down migrate-status migrate-create \
	env-start env-stop env-status docker-build docker-run docker-stop \
	lint lint-fix test test-integration test-mutation

generate:
	go tool oapi-codegen \
		-generate types,chi-server \
		-package api \
		-include-operation-ids createTrip,getTrip,finishTrip,createTripPosition,listTripPositions,health,ready \
		-o internal/generated/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml
	go tool oapi-codegen \
		-generate types \
		-package api \
		-o internal/generated/push.gen.go \
		contracts/openapi/push-service.openapi.yaml
	go generate ./...

env-start:
	@set -e; \
	if [ -f "$(ENV_FILE)" ] && [ ! -f "$(LOCAL_ENV_FILE)" ]; then \
		mv "$(ENV_FILE)" "$(LOCAL_ENV_FILE)"; \
	fi; \
	tripgoctl environment start; \
	tripgoctl connect; \
	mkdir -p .tripgo; \
	cat .env.example > "$(RUNTIME_ENV_FILE)"; \
	if [ -f "$(LOCAL_ENV_FILE)" ]; then cat "$(LOCAL_ENV_FILE)" >> "$(RUNTIME_ENV_FILE)"; fi; \
	cat "$(ENV_FILE)" >> "$(RUNTIME_ENV_FILE)"

run: env-start
	@set -e; \
	set -a; . "./$(RUNTIME_ENV_FILE)"; set +a; \
	go tool goose -dir ./migrations postgres "$$DATABASE_URL" up; \
	exec go run ./cmd/trip-service/

migrate-create:
	go tool goose -dir migrations -s create $(name) sql

migrate: env-start
	@set -a; . "./$(RUNTIME_ENV_FILE)"; set +a; \
	go tool goose -dir ./migrations postgres "$$DATABASE_URL" up

migrate-down: env-start
	@set -a; . "./$(RUNTIME_ENV_FILE)"; set +a; \
	go tool goose -dir ./migrations postgres "$$DATABASE_URL" down

migrate-status: env-start
	@set -a; . "./$(RUNTIME_ENV_FILE)"; set +a; \
	go tool goose -dir ./migrations postgres "$$DATABASE_URL" status

docker-run: env-start
	@set -e; \
	set -a; . "./$(RUNTIME_ENV_FILE)"; set +a; \
	go tool goose -dir ./migrations postgres "$$DATABASE_URL" up; \
	docker run -d \
		--name $(CONTAINER_NAME) \
		--network host \
		--env-file $(RUNTIME_ENV_FILE) \
		$(IMAGE_NAME)

env-stop:
	tripgoctl environment stop

env-status:
	tripgoctl environment status

docker-build:
	docker build -t $(IMAGE_NAME) -f deploy/Dockerfile .

docker-stop:
	docker rm -f $(CONTAINER_NAME) 2>/dev/null || true

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

test:
	go test -v -race -cover ./...
	go test -v -race -count=2 -tags=integration ./internal/repository/trip

test-integration:
	go test -v -race -count=2 -tags=integration ./internal/repository/trip

test-mutation:
	gremlins unleash ./internal/domain --timeout-coefficient 20
	gremlins unleash ./internal/usecase --timeout-coefficient 20
