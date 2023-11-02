FROM docker.io/golang:alpine as builder

COPY . /src
WORKDIR /src

RUN apk add --no-cache --update go gcc g++
RUN go build -o /src/yggmail ./cmd/yggmail

FROM docker.io/alpine
COPY --from=builder /src/yggmail /usr/bin/yggmail

EXPOSE 1143/tcp
EXPOSE 1025/tcp
VOLUME /etc/yggmail

ENTRYPOINT ["/usr/bin/yggmail", "-database=/etc/yggmail/yggmail.db"]