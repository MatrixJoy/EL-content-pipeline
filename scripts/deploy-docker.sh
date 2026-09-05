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
if ! ssh "$REMOTE_HOST" "cd '$REMOTE_DIR' && docker compose build crawler aligner"; then
  echo "registry build unavailable; falling back to a locally compiled Linux crawler"
  remote_machine=$(ssh "$REMOTE_HOST" uname -m)
  case "$remote_machine" in
    x86_64) remote_goarch=amd64 ;;
    aarch64|arm64) remote_goarch=arm64 ;;
    *) echo "unsupported remote architecture: $remote_machine" >&2; exit 1 ;;
  esac
  build_dir=$(mktemp -d)
  trap 'rm -rf "$build_dir"' EXIT
  CGO_ENABLED=0 GOOS=linux GOARCH="$remote_goarch" go build -trimpath -ldflags="-s -w" -o "$build_dir/library-sync" ./cmd/library-sync
  ssh "$REMOTE_HOST" "mkdir -p '$REMOTE_DIR/.deploy'"
  rsync -az "$build_dir/library-sync" "$REMOTE_HOST:$REMOTE_DIR/.deploy/"
  ssh "$REMOTE_HOST" "cd '$REMOTE_DIR' && docker build --pull=false -f Dockerfile.prebuilt -t voa-content-library-crawler ."
fi
ssh "$REMOTE_HOST" "cd '$REMOTE_DIR' && docker compose up -d minio"
