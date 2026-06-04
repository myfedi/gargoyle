#!/bin/sh
set -eu

CONFIG_PATH="${GARGOYLE_CONFIG:-/config/config.yml}"
USERNAME="${GARGOYLE_USERNAME:-alice}"
EMAIL="${GARGOYLE_EMAIL:-alice@gargoyle.test}"
PASSWORD="${GARGOYLE_PASSWORD:-Str0ngP@ssword!}"

mkdir -p /data /media

gargoyle-migrate db --config "$CONFIG_PATH" init
gargoyle-migrate db --config "$CONFIG_PATH" migrate

# The integration stack uses throw-away volumes, but tolerate restarts during debugging.
if ! gargoyle-admin register --config "$CONFIG_PATH" --email "$EMAIL" --username "$USERNAME" --password "$PASSWORD"; then
  echo "gargoyle user registration failed; continuing in case the user already exists" >&2
fi

exec gargoyle-server "$CONFIG_PATH"
