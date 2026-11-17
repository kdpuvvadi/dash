# syntax=docker/dockerfile:1
# Runtime image. Prefer a prebuilt linux binary at dist/dash when present
# so the LXC does not need a full Go/Node build (1GB RAM friendly).

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates openssh-client \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY dist/dash /app/dash
COPY apps /app/apps
COPY schemas /app/schemas
RUN chmod +x /app/dash
ENV DASH_CONFIG=/config
EXPOSE 8080
VOLUME ["/config"]
ENTRYPOINT ["/app/dash", "serve", "-config", "/config"]
