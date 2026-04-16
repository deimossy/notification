ARG GO_VERSION=1.25-alpine
ARG ALPINE_VERSION=3.22

ARG BINARY_NAME=app
ARG MIGRATOR_BINARY_NAME=migrator
ARG CONFIG_DIR=config
ARG WORKDIR_PATH=/app

FROM golang:${GO_VERSION} AS builder

ARG BINARY_NAME
ARG MIGRATOR_BINARY_NAME
ARG CONFIG_DIR

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ${BINARY_NAME} ./cmd/app
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ${MIGRATOR_BINARY_NAME} ./cmd/migrator

FROM alpine:${ALPINE_VERSION} AS app

ARG BINARY_NAME
ARG CONFIG_DIR
ARG WORKDIR_PATH

WORKDIR ${WORKDIR_PATH}
COPY --from=builder /build/${BINARY_NAME} .
RUN mkdir -p ${WORKDIR_PATH}/${CONFIG_DIR}
COPY --from=builder /build/${CONFIG_DIR}/ ./${CONFIG_DIR}/

EXPOSE 8080

CMD ["./app"]

FROM alpine:${ALPINE_VERSION} AS migrator

ARG MIGRATOR_BINARY_NAME
ARG CONFIG_DIR
ARG WORKDIR_PATH

WORKDIR ${WORKDIR_PATH}
COPY --from=builder /build/${MIGRATOR_BINARY_NAME} .
RUN mkdir -p ${WORKDIR_PATH}/${CONFIG_DIR}
COPY --from=builder /build/${CONFIG_DIR}/ ./${CONFIG_DIR}/
COPY --from=builder /build/migrations/ ./migrations/

CMD ["./migrator", "-command", "up"]
