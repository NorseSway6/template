ENV_FILE := .env

ifneq (,$(wildcard $(ENV_FILE)))
    include $(ENV_FILE)
    export
endif

IMAGE_NAME := tripgo-app
CONTAINER_NAME := tripgo_app
PORT := 8080

.PHONY: generate run migrate migrate-down migrate-status migarte-create \
	env-stop env-status docker-build docker-run docker-stop \
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

run:
	tripgoctl environment start && tripgoctl connect
	@set -e; \
	cp .env.example "$(ENV_FILE)"; \
	set -a; . "./$(ENV_FILE)"; set +a; \
	goose -dir ./migrations postgres "$$DATABASE_URL" up; \
	exec go run ./cmd/trip-service/

migrate-create:
	goose -dir migrations -s create $(name) sql

migrate:
	goose -dir ./migrations postgres "$$DATABASE_URL" up

migrate-down:
	goose -dir ./migrations postgres "$$DATABASE_URL" down

migrate-status:
	goose -dir ./migrations postgres "$$DATABASE_URL" status

env-stop:
	tripgoctl environment stop

env-status:
	tripgoctl environment status

docker-build:
	docker build -t $(IMAGE_NAME) -f deploy/Dockerfile .

docker-run:
	docker run -d \
		--name $(CONTAINER_NAME) \
		--network host \
		--env-file $(ENV_FILE) \
		$(IMAGE_NAME)

docker-stop:
	docker rm -f $(CONTAINER_NAME) 2>/dev/null || true

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

test:
	go test -v -race -cover ./...

test-integration:
	go test -v -race -count=2 -tags=integration ./internal/repository/trip

test-mutation:
	gremlins unleash ./internal/domain --timeout-coefficient 20
	gremlins unleash ./internal/usecase --timeout-coefficient 20
