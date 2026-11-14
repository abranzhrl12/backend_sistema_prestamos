# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
COPY authkit/go.mod authkit/go.sum ./authkit/
RUN go mod download

COPY authkit ./authkit
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/prestamos-api ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app
COPY --from=build /out/prestamos-api ./prestamos-api
COPY migrations ./migrations

USER app
EXPOSE 8080

ENTRYPOINT ["/app/prestamos-api"]
