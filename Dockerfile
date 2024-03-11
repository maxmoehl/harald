FROM golang:1.22.1-alpine AS builder

ENV GOBIN=/usr/local/bin
RUN mkdir /src
WORKDIR /src

COPY . /src
RUN go install ./cmd/harald

FROM alpine:3.19.1

LABEL org.opencontainers.image.source = "https://github.com/maxmoehl/harald"
LABEL org.opencontainers.image.licenses = MIT

COPY --from=builder /usr/local/bin/harald /usr/local/bin

ENTRYPOINT [ "/usr/local/bin/harald" ]
