# syntax=docker/dockerfile:1
# Multi-stage build for ekanited (amd64-focused; adjust --platform for other targets).
# Version metadata is embedded via -ldflags (Issue #43), consistent with local builds.

FROM golang:1.24-alpine AS build
WORKDIR /src

# Cache module metadata; go.mod is present in this repo (not modified by this artifact).
COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build -trimpath \
  -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT}" \
  -o /out/ekanited ./cmd/ekanited

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/ekanited /usr/local/bin/ekanited

# Syslog ingestion (TCP/UDP) and query interfaces (defaults).
EXPOSE 5514/tcp 5514/udp 9950/tcp 8080/tcp 9951/tcp

# Persist the data directory via a volume mount, e.g. -v ekanite-data:/var/lib/ekanite
VOLUME ["/var/lib/ekanite"]

ENTRYPOINT ["/usr/local/bin/ekanited"]
CMD ["-datadir", "/var/lib/ekanite", "-tcp", "0.0.0.0:5514"]
