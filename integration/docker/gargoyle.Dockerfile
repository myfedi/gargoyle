# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

# docker:S6470 is intentionally accepted here: this integration build needs the repo
# context for the Go binaries, and the root .dockerignore excludes local secrets/data.
COPY . .
RUN go build -o /out/gargoyle-server ./cmd/web/server.go \
 && go build -o /out/gargoyle-migrate ./cmd/cli/migrate/main.go \
 && go build -o /out/gargoyle-admin ./cmd/cli/admin/main.go

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates sqlite3 wget \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/gargoyle-server /usr/local/bin/gargoyle-server
COPY --from=build /out/gargoyle-migrate /usr/local/bin/gargoyle-migrate
COPY --from=build /out/gargoyle-admin /usr/local/bin/gargoyle-admin
COPY integration/docker/gargoyle-entrypoint.sh /usr/local/bin/gargoyle-entrypoint.sh
RUN chmod +x /usr/local/bin/gargoyle-entrypoint.sh \
 && addgroup --system gargoyle \
 && adduser --system --ingroup gargoyle --home /nonexistent --no-create-home gargoyle \
 && mkdir -p /data /media \
 && chown -R gargoyle:gargoyle /data /media
USER gargoyle
ENTRYPOINT ["/usr/local/bin/gargoyle-entrypoint.sh"]
