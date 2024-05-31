FROM docker.io/golang:alpine3.20 as builder

COPY . /src
WORKDIR /src

RUN apk add --no-cache --update go gcc g++
RUN go build -o /src/yggmail ./cmd/yggmail

FROM docker.io/alpine:3.18

LABEL org.opencontainers.image.source=https://github.com/neilalexander/yggmail
LABEL org.opencontainers.image.description=Yggmail
LABEL org.opencontainers.image.licenses=MPL-2.0

COPY --from=builder /src/yggmail /usr/bin/yggmail

EXPOSE 1143/tcp
EXPOSE 1025/tcp
VOLUME /etc/yggmail

ENTRYPOINT ["/usr/bin/yggmail", "-smtp=:1025", "-imap=:1143", "-database=/etc/yggmail/yggmail.db"]
