#!/bin/sh
set -eu

gotenberg --api-port=3000 --api-bind-ip=127.0.0.1 &

i=0
while [ "$i" -lt 60 ]; do
  if curl -sf http://127.0.0.1:3000/health >/dev/null; then
    break
  fi
  i=$((i + 1))
  sleep 1
done

if ! curl -sf http://127.0.0.1:3000/health >/dev/null; then
  echo "gotenberg failed to become healthy" >&2
  exit 1
fi

exec /app/pdf-go-server
