ARG DOCKER_REGISTRY=docker.io

FROM ${DOCKER_REGISTRY}/debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive

RUN --mount=type=cache,id=debian-bookworm,target=/var/cache/apt \
  --mount=type=cache,id=debian-bookworm,target=/var/lib/apt \
  apt-get update && apt-get install -y --no-install-recommends \
  curl ca-certificates

RUN --mount=type=cache,id=debian-bookworm,target=/var/cache/apt \
  --mount=type=cache,id=debian-bookworm,target=/var/lib/apt \
  apt-get update && apt-get install -y --no-install-recommends \
  build-essential curl ca-certificates \
  libxml2 libcrypt1 libkrb5-3 libgssapi-krb5-2 \
  libldap-2.5-0 libaio1 libnuma1 zlib1g

