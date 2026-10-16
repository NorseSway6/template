# 1 стадия
FROM golang:1.26.8-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/tripgo ./cmd/trip-service/main.go

# 2 стадия
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=builder /app/tripgo /tripgo

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/tripgo"]
