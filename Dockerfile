# syntax=docker/dockerfile:1

# Create a stage for building the application.
ARG GO_VERSION=1.24.4
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
WORKDIR /src

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

# This is the architecture you're building for, which is passed in by the builder.
# Placing it here allows the previous steps to be cached across architectures.
ARG TARGETARCH
ARG VERSION="<unset>"

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 GOARCH=$TARGETARCH \
    go build -ldflags "-X main.Version=${VERSION}" \
    -o /bin/supercronic .

################################################################################
FROM alpine:latest AS final

RUN --mount=type=cache,target=/var/cache/apk \
    apk --update add \
    ca-certificates \
    tzdata \
    && \
    update-ca-certificates

# Create a non-privileged user that the app will run under.
# See https://docs.docker.com/go/dockerfile-user-best-practices/
ARG UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --system \
    --no-create-home \
    --uid "${UID}" \
    supercronic
USER supercronic

COPY --from=build /bin/supercronic /bin/

ENTRYPOINT [ "/bin/supercronic" ]
