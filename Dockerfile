FROM billyteves/alpine-golang-glide:1.2.0 as builder

WORKDIR /go/src/github.com/aptible/supercronic/
# Download dependencies in a separate layer to optimize
# the use of the layers cache even when the project code changes
COPY glide.yaml .
RUN glide install

COPY . .
RUN make build

# Build final image straight from alpine
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /go/src/github.com/aptible/supercronic/supercronic .
ENTRYPOINT ["./supercronic"]
