# syntax=docker/dockerfile:1.7

FROM golang:1.24 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY webui ./webui
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/xiaohongshu-webui ./webui

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 1000 webui \
    && adduser -S -D -H -u 1000 -G webui webui
COPY --from=builder /out/xiaohongshu-webui /usr/local/bin/xiaohongshu-webui

USER webui
EXPOSE 8080
ENV WEBUI_ADDR=:8080
ENTRYPOINT ["/usr/local/bin/xiaohongshu-webui"]
