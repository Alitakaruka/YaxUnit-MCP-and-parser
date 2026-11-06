FROM golang:1.26.5-alpine AS build

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY src ./src
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/yaxbase ./src

FROM alpine:3.23

RUN apk add --no-cache ca-certificates \
    && addgroup -g 10001 app \
    && adduser -D -u 10001 -G app app \
    && mkdir -p /app \
    && chown app:app /app

WORKDIR /app
COPY --from=build --chown=app:app /out/yaxbase ./yaxbase
USER app

ENTRYPOINT ["/app/yaxbase"]