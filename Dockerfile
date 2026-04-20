FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/fst-api ./cmd/api

FROM alpine:3.20

RUN addgroup -S fst && adduser -S fst -G fst

WORKDIR /app
COPY --from=build /out/fst-api /app/fst-api

USER fst
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/health >/dev/null || exit 1

ENTRYPOINT ["/app/fst-api"]
