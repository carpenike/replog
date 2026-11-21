#!/usr/bin/env bash
# Smoke-test a local RepLog image. Does not push.
#
#   scripts/image-smoke.sh <image>            # runtime checks, then secret scan
#   scripts/image-smoke.sh runtime <image>    # read-only root, writable /data, /healthz
#   scripts/image-smoke.sh secrets <image>    # no secret in the image filesystem or history
#
# The runtime check starts the image with a read-only root and a writable
# /data bind mount. It requires /healthz to answer 200 "ok", the nonroot
# process to create /data/replog.db, a write under /data to succeed, and a
# write to / to fail. A canary REPLOG_SECRET_KEY and admin password are
# injected only at runtime and must not show up in the logs or the database.
set -euo pipefail

usage() {
  echo "usage: scripts/image-smoke.sh [runtime|secrets] <image>" >&2
  exit 2
}

mode="all"
if [ "$#" -eq 1 ]; then
  image="$1"
elif [ "$#" -eq 2 ]; then
  mode="$1"
  image="$2"
else
  usage
fi

case "$mode" in
  all | runtime | secrets) ;;
  *) usage ;;
esac

if [ -z "$image" ]; then
  usage
fi

work="${RUNNER_TEMP:-$(mktemp -d)}"
mkdir -p "$work"
name="replog-smoke-$$"
inspect="replog-smoke-inspect-$$"

cleanup() {
  docker rm -f "$name" "$inspect" >/dev/null 2>&1 || true
}
trap cleanup EXIT

rand() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 16
  else
    python3 -c 'import secrets; print(secrets.token_hex(16))'
  fi
}

nonroot_ids() {
  docker rm -f "$inspect" >/dev/null 2>&1 || true
  docker create --name "$inspect" "$image" >/dev/null
  docker cp "$inspect":/etc/passwd "$work/passwd"
  docker rm "$inspect" >/dev/null
  uid="$(awk -F: '$1=="nonroot" {print $3; exit}' "$work/passwd")"
  gid="$(awk -F: '$1=="nonroot" {print $4; exit}' "$work/passwd")"
  if [ -z "$uid" ] || [ -z "$gid" ]; then
    echo "image passwd has no nonroot user" >&2
    exit 1
  fi
}

refuse_logged_secrets() {
  if grep -q -F "$canary" "$work/smoke.log"; then
    echo "secret key reached the container log" >&2
    exit 1
  fi
  if grep -q -F "$admin_pass" "$work/smoke.log"; then
    echo "admin password reached the container log" >&2
    exit 1
  fi
}

runtime_check() {
  canary="CANARY-$(rand)"
  admin_pass="ci-$(rand)"
  nonroot_ids

  # Bind-mount /data instead of tmpfs. docker cp does not see tmpfs
  # contents, and the host directory is how the check proves the process
  # wrote the database. 1777 so uid nonroot can create files without a
  # chown (the runner user owns the directory).
  mkdir -p "$work/data"
  chmod 1777 "$work/data"

  docker rm -f "$name" >/dev/null 2>&1 || true
  docker run -d --name "$name" --read-only \
    -v "$work/data:/data" \
    -p "127.0.0.1:${port}:8080" \
    -e REPLOG_ADMIN_USER=ci \
    -e REPLOG_ADMIN_PASS="$admin_pass" \
    -e REPLOG_ADMIN_EMAIL=ci@example.com \
    -e REPLOG_SECRET_KEY="$canary" \
    "$image" >/dev/null

  ok=0
  for _ in $(seq 1 90); do
    code="$(curl -s -o "$work/healthz" -w '%{http_code}' "http://127.0.0.1:${port}/healthz" || true)"
    if [ "$code" = "200" ] && grep -qx 'ok' "$work/healthz"; then
      ok=1
      break
    fi
    sleep 1
  done

  docker logs "$name" >"$work/smoke.log" 2>&1 || true
  refuse_logged_secrets
  if [ "$ok" != 1 ]; then
    cat "$work/smoke.log" >&2
    echo "/healthz did not return 200 ok" >&2
    exit 1
  fi

  if [ "$(docker inspect -f '{{.HostConfig.ReadonlyRootfs}}' "$name")" != "true" ]; then
    echo "container root is not read-only" >&2
    exit 1
  fi

  # The process created the database, so uid nonroot can write /data.
  # The canary is runtime-only; it must not be stored in the database either.
  test -s "$work/data/replog.db"
  db_uid="$(stat -c '%u' "$work/data/replog.db")"
  if [ "$db_uid" != "$uid" ]; then
    echo "database owner is ${db_uid}, expected nonroot ${uid}" >&2
    exit 1
  fi
  if grep -a -F -q "$canary" "$work/data/replog.db" || grep -a -F -q "$admin_pass" "$work/data/replog.db"; then
    echo "runtime secret was written into /data/replog.db" >&2
    exit 1
  fi
  if [ -f "$work/data/replog.db-wal" ]; then
    if grep -a -F -q "$canary" "$work/data/replog.db-wal" || grep -a -F -q "$admin_pass" "$work/data/replog.db-wal"; then
      echo "runtime secret was written into /data/replog.db-wal" >&2
      exit 1
    fi
  fi

  printf 'probe\n' >"$work/data/data-probe"
  test -s "$work/data/data-probe"
  printf 'probe\n' >"$work/root-probe"

  if docker cp "$work/root-probe" "$name":/root-write-probe 2>"$work/root-write.err"; then
    echo "root filesystem accepted a write" >&2
    exit 1
  fi
  if ! grep -qi 'read-only' "$work/root-write.err"; then
    echo "write to / failed for an unexpected reason:" >&2
    cat "$work/root-write.err" >&2
    exit 1
  fi

  docker rm -f "$name" >/dev/null
}

secrets_check() {
  docker rm -f "$inspect" >/dev/null 2>&1 || true
  docker create --name "$inspect" "$image" >/dev/null
  docker export "$inspect" -o "$work/fs.tar"
  docker rm "$inspect" >/dev/null

  names="$(tar -tf "$work/fs.tar")"
  if grep -E '(^|/)(\.env|id_rsa|github_token)($|/)' <<<"$names"; then
    echo "secret-shaped file in the image" >&2
    exit 1
  fi
  # The runtime stage copies only the binary. Source and git metadata
  # must not come along.
  if grep -E '(^|/)(go\.mod|\.git/|Dockerfile)($|/)' <<<"$names"; then
    echo "build context leaked into the runtime image" >&2
    exit 1
  fi

  tar -xOf "$work/fs.tar" >"$work/fs.bin"
  if grep -a -F -q 'BEGIN PRIVATE KEY' "$work/fs.bin"; then
    echo "private key in the image filesystem" >&2
    exit 1
  fi
  if grep -a -E -q 'ghp_|ghs_|github_pat_' "$work/fs.bin"; then
    echo "token in the image filesystem" >&2
    exit 1
  fi

  history="$(docker history --no-trunc "$image")"
  if grep -E 'ghp_|ghs_|github_pat_|REPLOG_SECRET_KEY=.+|REPLOG_ADMIN_PASS=.+' <<<"$history"; then
    echo "secret in image history" >&2
    exit 1
  fi
}

port="${SMOKE_PORT:-18080}"

case "$mode" in
  runtime) runtime_check ;;
  secrets) secrets_check ;;
  all)
    runtime_check
    secrets_check
    ;;
esac
