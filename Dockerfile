# syntax=docker/dockerfile:1.2

# Image page: <https://hub.docker.com/_/golang>
# Inportant note: Do not forget to update the version here on Go update
FROM golang:1.14-alpine as builder

WORKDIR /src

COPY . .

RUN apk add --no-cache make bash

RUN CGO_ENABLED=0 make build

WORKDIR /tmp/rootfs

# prepare rootfs for runtime
RUN set -x \
    && mkdir -p \
        ./etc \
        ./bin \
    && echo 'supercronic:x:10001:10001::/nonexistent:/sbin/nologin' > ./etc/passwd \
    && echo 'supercronic:x:10001:' > ./etc/group \
    && mv /src/supercronic ./bin/supercronic

# use empty filesystem
FROM scratch as runtime

LABEL \
    # Docs: <https://github.com/opencontainers/image-spec/blob/master/annotations.md>
    org.opencontainers.image.title="supercronic" \
    org.opencontainers.image.description="Cron for containers" \
    org.opencontainers.image.url="https://github.com/aptible/supercronic" \
    org.opencontainers.image.source="https://github.com/aptible/supercronic" \
    org.opencontainers.image.vendor="aptible" \
    org.opencontainers.image.licenses="MIT"

# import from builder
COPY --from=builder /tmp/rootfs /

# use an unprivileged user
USER supercronic:supercronic

ENTRYPOINT ["/bin/supercronic"]
