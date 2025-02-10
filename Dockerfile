FROM golang:1.23.6-alpine AS builder

ENV GOBIN=/usr/local/bin
RUN mkdir /src
WORKDIR /src

COPY . /src
RUN go install ./cmd/harald

FROM alpine:3.21.2

LABEL org.opencontainers.image.source = "https://github.com/maxmoehl/harald"
LABEL org.opencontainers.image.licenses = MIT

COPY --from=builder /usr/local/bin/harald /usr/local/bin

ENTRYPOINT [ "/usr/local/bin/harald" ]
