FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bill-sharing ./cmd/server

FROM alpine:3.20

RUN addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app \
    && mkdir -p /app /data/uploads \
    && chown -R app:app /app /data

WORKDIR /app

COPY --from=build /out/bill-sharing /app/bill-sharing
COPY --chown=app:app web /app/web

ENV PORT=8080 \
    WEB_DIR=/app/web \
    UPLOAD_DIR=/data/uploads \
    COOKIE_SECURE=true

EXPOSE 8080

VOLUME ["/data/uploads"]

USER app

ENTRYPOINT ["/app/bill-sharing"]
