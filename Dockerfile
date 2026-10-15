# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM node:26-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26.8-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=web /src/internal/ui/dist/ internal/ui/dist/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /textdock ./cmd/textdock

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S textdock && adduser -S -G textdock textdock && mkdir /data && chown textdock:textdock /data
COPY --from=build /textdock /usr/local/bin/textdock
USER textdock
ENV TEXTDOCK_LISTEN=0.0.0.0:18257 TEXTDOCK_DB=/data/textdock.db
VOLUME ["/data"]
EXPOSE 18257
ENTRYPOINT ["textdock"]
