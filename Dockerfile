ARG GO_VERSION=1.20.4
ARG ALPINE_VERSION=3.18
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

ENV GOBIN=/usr/local/bin
RUN mkdir /src
WORKDIR /src

COPY . /src
RUN go install ./cmd/harald

FROM alpine:${ALPINE_VERSION}

COPY --from=builder /usr/local/bin/harald /usr/local/bin

ENTRYPOINT [ "/usr/local/bin/harald" ]
