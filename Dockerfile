FROM billyteves/alpine-golang-glide:1.2.0 as builder

WORKDIR /go/src/github.com/aptible/supercronic/
COPY glide.yaml .
RUN glide install
COPY . .
RUN make build

FROM alpine:latest

WORKDIR /root/
COPY --from=builder /go/src/github.com/aptible/supercronic/supercronic .
ENTRYPOINT ["./supercronic"]
