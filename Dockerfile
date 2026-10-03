# Build stage
FROM golang:alpine AS builder

WORKDIR /src

# Copy go workspace and module manifests
COPY go.work ./
COPY lib/go.mod lib/
COPY server/go.mod server/
COPY client/native/go.mod client/native/
COPY keypair-generator/go.mod keypair-generator/

# Copy source code needed for the server
COPY lib/ lib/
COPY server/ server/

ARG VERSION=0.1.3

# Build static binary
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X 'wake-on-wan-relay/lib.VERSION=${VERSION}'" -o /out/server ./server

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/server /app/server

# Server listens on 8443 by default
EXPOSE 8443

# Wake-on-LAN packets require host networking (--net=host) to reach the physical LAN.
ENTRYPOINT ["/app/server"]
