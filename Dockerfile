ARG GO_VERSION=1.23

FROM golang:${GO_VERSION} as builder

WORKDIR $GOPATH/github.com/maxihafer/supercronic/
COPY . .

ENV USER=cron
ENV UID=10010

RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-w -s \
	-o /supercronic .

FROM alpine:3.20

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

COPY --from=builder /cron-verslaggever /cron-verslaggever

USER cron:cron

ENTRYPOINT ["/supercronic"]
