FROM golang:1-alpine

ADD . "${GOPATH}/src/github.com/aptible/supercronic"

RUN apk add --no-cache git \
    && go get -u github.com/golang/dep/cmd/dep \
    && cd "${GOPATH}/src/github.com/golang/dep/cmd/dep" \
    && dep init \
    && cd "${GOPATH}/src/github.com/aptible/supercronic" \
    && dep ensure \
    && go install \
    && apk del git \
    && rm "${GOPATH}/bin/dep" \
    && rm -Rf "${GOPATH}/src" "${GOPATH}/pkg"

# Create a default crontab - this should be overwritten
RUN echo "*/5 * * * * * * echo \"hello from Supercronic\"" >> "${GOPATH}/crontab"

CMD supercronic "${GOPATH}/crontab"