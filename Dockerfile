FROM golang:1.23-alpine AS builder

COPY ./ ./

RUN go build -trimpath -ldflags="-s -w" -o /wst ./server/main.go

FROM alpine:latest

ENV PUID=0 PGID=0 UMASK=022

COPY --from=builder /wst /wst

RUN apk add --no-cache bash ca-certificates su-exec tzdata && \
    rm -rf /var/cache/apk/*

EXPOSE 8080/tcp

ENV LISTEN=0.0.0.0:8080

ENTRYPOINT [ "/wst" ]
