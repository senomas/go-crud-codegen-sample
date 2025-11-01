ARG DOCKER_REGISTRY=docker.io
ARG GOLANG_VER=ver
ARG DEBIAN_BASE_VER=ver
ARG UI_BUILDER_VER=ver

FROM ${DOCKER_REGISTRY}/golang:${GOLANG_VER} AS base

WORKDIR /app/log
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY util ./util/

FROM base AS migration

COPY cmd ./cmd/
RUN go build -o /app/migrate ./cmd/migrate

COPY migrations .

FROM base AS builder

COPY *.go .
COPY model ./model/
COPY handler ./handler/
RUN go build -o app .

COPY --from=migration /app/migrate ./migrate
COPY migrations .
