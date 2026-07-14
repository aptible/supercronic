# Build stage
ARG GO_VERSION=1.26.4
FROM docker.io/library/golang:${GO_VERSION}-alpine AS build

ARG VERSION=dev

WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Build the static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags "-s -w -X main.Version=${VERSION}" \
    -o /out/supercronic ./

# Final stage
FROM scratch

# OCI labels.
LABEL org.opencontainers.image.title="supercronic" \
      org.opencontainers.image.description="Crontab-compatible job runner for containers" \
      org.opencontainers.image.source="https://github.com/aptible/supercronic"


COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/group  /etc/group

COPY --from=build /out/supercronic /usr/local/bin/supercronic

USER nobody:nobody

ENTRYPOINT ["/usr/local/bin/supercronic"]
