ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /out/turn-broker ./cmd/turn-broker

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.source="https://github.com/turn-proxy/turn-broker"
LABEL org.opencontainers.image.description="HTTP broker serving TURN credentials"
LABEL org.opencontainers.image.licenses="MIT"
COPY --from=build /out/turn-broker /turn-broker
EXPOSE 8787
ENTRYPOINT ["/turn-broker"]
CMD ["-config", "/etc/turn-broker/turn-broker.json"]
