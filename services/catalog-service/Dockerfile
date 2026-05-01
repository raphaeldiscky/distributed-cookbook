# catalog-service container image. Build context is the repo root so
# the Dockerfile can reach pkg/ and the shared go.mod.
#
# Build:
#   docker build -t cookbook/catalog-service:dev \
#     -f services/catalog-service/Dockerfile .

ARG GO_VERSION=1.26.2
ARG ALPINE_VERSION=3.22

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY pkg/ pkg/
COPY services/catalog-service/ services/catalog-service/

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
        -o /out/server ./services/catalog-service/cmd/server

FROM alpine:${ALPINE_VERSION}

# hadolint ignore=DL3018  # alpine ca-certificates is not version-pinned upstream; image tag pinning provides reproducibility.
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /out/server /app/server

RUN addgroup -S app && adduser -S -G app app
USER app

EXPOSE 8080
ENTRYPOINT ["/app/server"]
