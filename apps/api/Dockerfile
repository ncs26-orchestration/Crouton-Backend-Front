# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS base
WORKDIR /app
RUN apk add --no-cache git

FROM base AS deps
COPY apps/api/go.mod apps/api/go.sum* ./
RUN go mod download || true

FROM deps AS build
COPY apps/api/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/server /server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]

# dev stage: mount source, use air for hot reload
FROM base AS dev
ARG DBMATE_VERSION=v2.21.0
RUN apk add --no-cache curl \
    && for i in $(seq 1 5); do go install github.com/air-verse/air@latest && break || sleep 5; done \
    && curl --retry 3 --http1.1 -fsSL -o /usr/local/bin/dbmate \
        "https://github.com/amacneil/dbmate/releases/download/${DBMATE_VERSION}/dbmate-linux-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" \
    && chmod +x /usr/local/bin/dbmate
WORKDIR /app
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]
