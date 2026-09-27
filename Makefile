ENV_FILE := .env

ifneq (,$(wildcard $(ENV_FILE)))
    include $(ENV_FILE)
    export
endif

IMAGE_NAME := tripgo-app
CONTAINER_NAME := tripgo_app
PORT := 8080

.PHONY: generate run migrate-up migrate-down migrate-status migarte-create \
	env-start env-stop env-status env-connect docker-build docker-run docker-stop

generate:
	go tool oapi-codegen \
		-generate types,chi-server \
		-package api \
		-include-operation-ids createTrip,getTrip,finishTrip,health,ready \
		-o internal/generated/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml

run:
	go run ./cmd/trip-service/

migrate-create:
	goose -dir migrations -s create $(name) sql

migrate-up:
	goose -dir ./migrations postgres "$$DATABASE_URL" up

migrate-down:
	goose -dir ./migrations postgres "$$DATABASE_URL" down

migrate-status:
	goose -dir ./migrations postgres "$$DATABASE_URL" status

env-start:
	tripgoctl environment start

env-stop:
	tripgoctl environment stop

env-status:
	tripgoctl environment status

env-connect:
	tripgoctl connect

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

