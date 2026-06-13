ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /out/turn-broker ./cmd/turn-broker

FROM alpine:latest
LABEL org.opencontainers.image.source="https://github.com/turn-proxy/turn-broker"
LABEL org.opencontainers.image.description="HTTP broker serving TURN credentials"
LABEL org.opencontainers.image.licenses="MIT"
RUN apk add --no-cache ca-certificates
COPY --from=build /out/turn-broker /usr/local/bin/turn-broker
ENTRYPOINT ["turn-broker"]
CMD ["-config", "/etc/turn-broker/turn-broker.json"]
