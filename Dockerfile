FROM alpine:3.6 as builder

RUN apk add --no-cache go glide git build-base

ENV GOPATH /go
WORKDIR /go/src/github.com/aptible/supercronic/
COPY glide.yaml .
COPY glide.lock .
RUN glide install
COPY . .
RUN go build -i

FROM alpine
COPY --from=builder /go/src/github.com/aptible/supercronic/supercronic /usr/local/bin/
ENTRYPOINT ["supercronic"]

