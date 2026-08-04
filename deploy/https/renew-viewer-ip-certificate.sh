#!/bin/sh
set -eu

SERVER_IP="${VIEWER_SERVER_IP:-135.106.130.37}"
PROJECT_DIR="${VIEWER_PROJECT_DIR:-/opt/viewer/viewer_backend}"
CERTBOT_ROOT="${VIEWER_CERTBOT_ROOT:-/opt/viewer/certbot}"
TLS_DIR="${VIEWER_TLS_DIR:-/opt/viewer/tls}"
CERTBOT_IMAGE="${CERTBOT_IMAGE:-certbot/certbot:latest}"

mkdir -p "$CERTBOT_ROOT/etc" "$CERTBOT_ROOT/lib" "$CERTBOT_ROOT/www" "$TLS_DIR"

docker run --rm \
  -v "$CERTBOT_ROOT/etc:/etc/letsencrypt" \
  -v "$CERTBOT_ROOT/lib:/var/lib/letsencrypt" \
  -v "$CERTBOT_ROOT/www:/var/www/certbot" \
  "$CERTBOT_IMAGE" renew --quiet

LIVE_DIR="$CERTBOT_ROOT/etc/live/$SERVER_IP"
if [ ! -r "$LIVE_DIR/fullchain.pem" ] || [ ! -r "$LIVE_DIR/privkey.pem" ]; then
  echo "Certificate for $SERVER_IP is not available" >&2
  exit 1
fi

changed=false
if ! cmp -s "$LIVE_DIR/fullchain.pem" "$TLS_DIR/fullchain.pem" 2>/dev/null; then
  install -o 101 -g 101 -m 0640 "$LIVE_DIR/fullchain.pem" "$TLS_DIR/fullchain.pem"
  changed=true
fi
if ! cmp -s "$LIVE_DIR/privkey.pem" "$TLS_DIR/privkey.pem" 2>/dev/null; then
  install -o 101 -g 101 -m 0640 "$LIVE_DIR/privkey.pem" "$TLS_DIR/privkey.pem"
  changed=true
fi

if [ "$changed" = true ]; then
  cd "$PROJECT_DIR"
  docker compose up -d --force-recreate frontend
fi
