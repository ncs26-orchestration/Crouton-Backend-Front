# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/simulator .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/simulator /simulator
USER nonroot:nonroot
ENTRYPOINT ["/simulator"]
