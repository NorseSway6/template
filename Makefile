ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: generate run migrate-up migrate-down migrate-status migarte-create

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
