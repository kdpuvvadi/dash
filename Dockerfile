# syntax=docker/dockerfile:1

FROM node:22-alpine AS ui
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /src/web/dist ./internal/embedui/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/dash ./cmd/dash

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates openssh-client \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/dash /app/dash
COPY apps /app/apps
COPY schemas /app/schemas
ENV DASH_CONFIG=/config
EXPOSE 8080
VOLUME ["/config"]
ENTRYPOINT ["/app/dash", "serve", "-config", "/config"]
