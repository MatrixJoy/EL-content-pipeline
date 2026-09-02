#!/bin/sh
set -eu

REMOTE_HOST=${REMOTE_HOST:-oldj@10.10.1.4}
REMOTE_DIR=${REMOTE_DIR:-/home/oldj/Application/voa-content-pipeline}

ssh "$REMOTE_HOST" "mkdir -p '$REMOTE_DIR'"
rsync -az --delete \
  --exclude .git \
  --exclude data \
  --exclude .DS_Store \
  ./ "$REMOTE_HOST:$REMOTE_DIR/"
ssh "$REMOTE_HOST" "cd '$REMOTE_DIR' && docker compose build crawler aligner && docker compose up -d minio"
