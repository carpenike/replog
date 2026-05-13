# Operations Guide

Operational runbook for a deployed RepLog instance. Covers first-time
deployment, backups, restores, secret-key handling, log review, upgrades,
and disaster recovery.

For day-to-day development commands see the [Justfile](../Justfile) and
the top-level [README](../README.md). For the env-var reference, see the
README's *Configuration* section. This guide is about running the binary
in production.

---

## Table of contents

1. [Operational model](#operational-model)
2. [First-time deployment](#first-time-deployment)
3. [Backups](#backups)
4. [Restoring from backup](#restoring-from-backup)
5. [`REPLOG_SECRET_KEY`: bootstrap, rotation, and recovery](#replog_secret_key-bootstrap-rotation-and-recovery)
6. [Upgrades](#upgrades)
7. [Logs and monitoring](#logs-and-monitoring)
8. [Reverse proxy notes](#reverse-proxy-notes)
9. [Disaster recovery](#disaster-recovery)
10. [Routine maintenance](#routine-maintenance)
11. [Security checklist](#security-checklist)

---

## Operational model

RepLog is a **single static binary** that owns:

- **A SQLite database file** (default `replog.db`, override with
  `REPLOG_DB_PATH`) — opened in WAL mode, single writer.
- **An avatar directory** (default sibling of the DB, override with
  `REPLOG_AVATAR_DIR`) — small image files keyed by user ID + random
  suffix. Served from a public no-auth route, so don't put anything here
  except the binary's own writes.
- **An optional secret key** in `app_settings._internal.secret_key` used
  to AES-256-GCM-encrypt sensitive settings (LLM API keys, SMTP
  password). Either supplied via `REPLOG_SECRET_KEY` or auto-generated on
  first run.
- **Sessions** in the same SQLite DB (table `sessions`).
- **A background scheduler** that prunes expired login tokens and old
  notifications (interval and retention controlled by app settings;
  defaults: 24h / 90d).

**There is no external state.** No Redis, no second database, no message
queue. Backing up the DB file plus the avatar directory captures
everything.

---

## First-time deployment

The reference deployment is the NixOS module shipped in
[`flake.nix`](../flake.nix) — a `services.replog.enable = true` block in
your nix-config. The flow below is environment-agnostic.

### 1. Provision the data directory

```bash
sudo mkdir -p /var/lib/replog/avatars
sudo chown -R replog:replog /var/lib/replog
sudo chmod 750 /var/lib/replog
```

(On NixOS the systemd unit creates this for you via `StateDirectory`.)

### 2. Decide on a `REPLOG_SECRET_KEY` strategy

You have two options. **Pick one before first launch — switching later
is non-trivial** (see [secret key section](#replog_secret_key-bootstrap-rotation-and-recovery)).

- **Auto-generate** (no env var set). The binary generates a 32-byte
  random key on first launch and stores it in the DB. The DB backup is
  then sufficient to restore — no separate secret to manage.
- **Provide via env var.** Generate a key (`openssl rand -base64 32`)
  and inject it via `REPLOG_SECRET_KEY` (`EnvironmentFile=` in systemd,
  `--env-file` in Docker, sealed-secrets in k8s, etc.). The env var
  wins on every launch and is also persisted to the DB row, so backups
  remain self-sufficient.

For a small self-hosted family deployment, **auto-generate is fine** —
the key only matters if you ever expose the DB file to someone you
don't trust with the LLM API keys it encrypts.

### 3. Set the bootstrap admin credentials

The binary refuses to start when the DB is empty unless these are set:

| Var | Required on first run | Notes |
|-----|----------------------|-------|
| `REPLOG_ADMIN_USER` | yes | Created as `is_admin = is_coach = 1`. |
| `REPLOG_ADMIN_PASS` | yes | Use a real passphrase; this account has full rights. |
| `REPLOG_ADMIN_EMAIL` | recommended | Used for display and password-reset UX (when added). |

These are read **only when the `users` table is empty**. After the first
successful boot they are ignored — feel free to remove them from the
service environment.

### 4. Set the production env vars

Minimum useful production set:

```bash
REPLOG_DB_PATH=/var/lib/replog/replog.db
REPLOG_AVATAR_DIR=/var/lib/replog/avatars
REPLOG_ADDR=127.0.0.1:8080            # bind loopback; expose via reverse proxy
REPLOG_BASE_URL=https://replog.example.com
REPLOG_WEBAUTHN_RPID=replog.example.com
REPLOG_WEBAUTHN_ORIGINS=https://replog.example.com
REPLOG_TRUSTED_PROXIES=127.0.0.1/32   # whatever your proxy is
```

Setting `REPLOG_BASE_URL` to an `https://` URL automatically:

- flips session cookies to `Secure`
- enables the `Strict-Transport-Security` response header (1 year,
  `includeSubDomains`)

You can override either explicitly with `REPLOG_SECURE_COOKIES=true|false`
if needed.

### 5. First boot and smoke test

```bash
sudo systemctl start replog
sudo journalctl -u replog -f
```

Expected log lines on a clean boot:

```
Database ready: /var/lib/replog/replog.db
Secret key generated and stored in database         # OR: loaded from database / env
Bootstrapped admin user: admin (id=1)
Seeded catalog: N equipment, N exercises, N programs ...
Background scheduler started
WebAuthn enabled: RPID=replog.example.com, Origins=[https://replog.example.com]
RepLog 0.x.y (commit, date) listening on :8080
```

Then through the proxy:

```bash
curl -fsS https://replog.example.com/healthz   # -> "ok"
curl -fsS https://replog.example.com/readyz    # -> "ok"   (DB ping)
```

Open the URL in a browser, log in with the bootstrap admin, change the
password, register a passkey, and (optionally) clear `REPLOG_ADMIN_PASS`
from the systemd EnvironmentFile.

---

## Backups

RepLog state is **DB file + avatar directory**. Everything else
(`web/dist/`, the binary, the seed catalog) is reproducible from the
source tree.

### What you must back up

| Path | What's in it | Critical? |
|------|--------------|-----------|
| `$REPLOG_DB_PATH` | All user data, sessions, encrypted settings, secret key | **Yes** |
| `$REPLOG_AVATAR_DIR` | User avatar images | No (cosmetic) |

### How to take a SQLite backup safely

**Never `cp` a live WAL-mode database.** A naïve copy taken while the
binary is running will miss writes that live in the `-wal` sidecar and
can produce a corrupt restore. Always use SQLite's online backup API:

```bash
sqlite3 /var/lib/replog/replog.db ".backup '/var/backups/replog/replog-$(date -u +%Y%m%dT%H%M%SZ).db'"
```

This works on a hot DB — the backup is consistent at the moment the
command starts. The `.backup` command holds a read lock briefly per
page, so it does not block writers for noticeable time on a small
family-scale DB.

### How to back up avatars

A plain copy is fine — they are immutable once written:

```bash
rsync -a --delete /var/lib/replog/avatars/ /var/backups/replog/avatars/
```

### Suggested cron / timer

```bash
#!/usr/bin/env bash
# /usr/local/sbin/replog-backup
set -euo pipefail
ts=$(date -u +%Y%m%dT%H%M%SZ)
dest=/var/backups/replog
mkdir -p "$dest"
sqlite3 /var/lib/replog/replog.db ".backup '$dest/replog-$ts.db'"
rsync -a --delete /var/lib/replog/avatars/ "$dest/avatars/"
# Keep 14 daily snapshots; keep weeklies forever (or rotate as you wish).
find "$dest" -maxdepth 1 -name 'replog-*.db' -mtime +14 -delete
```

Run nightly via systemd timer or cron. Then ship `$dest` somewhere
off-host (Restic, rclone to S3, borg, etc.) — RepLog has no opinion.

### Verify backups occasionally

A backup you have never restored is a wish, not a backup. At least once
per quarter:

```bash
sqlite3 /var/backups/replog/replog-LATEST.db "PRAGMA integrity_check;"
# Expected output: ok
```

For a fuller test, run the [restore drill](#restoring-from-backup) into
a scratch directory and start a binary against it.

---

## Restoring from backup

Stop the running instance first — SQLite single-writer means concurrent
writes during a swap will corrupt state.

```bash
sudo systemctl stop replog

# Move the live files aside (don't delete until you've verified the restore).
sudo mv /var/lib/replog/replog.db{,.broken}
sudo rm -f /var/lib/replog/replog.db-{wal,shm}    # discard live WAL sidecars
sudo cp /var/backups/replog/replog-GOOD.db /var/lib/replog/replog.db
sudo chown replog:replog /var/lib/replog/replog.db

# (Optional) restore avatars.
sudo rsync -a --delete /var/backups/replog/avatars/ /var/lib/replog/avatars/

sudo systemctl start replog
sudo journalctl -u replog -n 50
```

What you should see in the logs:

- `Database ready: /var/lib/replog/replog.db`
- `Secret key loaded from database` (or `from REPLOG_SECRET_KEY environment variable`)
- **No** `Bootstrapped admin user` line — that only fires on an empty DB.
- **No** `Seeded catalog` line — same reason.

If the binary tries to bootstrap an admin from a restore, the DB is
empty / wrong file. Stop and investigate before any user logs in (a new
admin would shadow the originals).

### Restoring to a different host

If `REPLOG_SECRET_KEY` is auto-generated, **the key lives in the DB
row**. A restore on a fresh host just works: the binary finds the row
and uses it.

If `REPLOG_SECRET_KEY` is supplied via env var, **you must inject the
same value** on the new host. The encrypted LLM keys / SMTP password in
`app_settings` are unrecoverable without it. (See the next section.)

---

## `REPLOG_SECRET_KEY`: bootstrap, rotation, and recovery

### What it does

Sensitive `app_settings` values (anything with `Sensitive: true` in the
registry — currently LLM API keys, SMTP password, notification URLs that
contain secrets) are encrypted with AES-256-GCM. The key is derived
from `REPLOG_SECRET_KEY` via HKDF-SHA256 (`info = "aes-256-gcm"`,
`salt = "replog-settings-v1"`).

The key is **the same value used for every encrypt and decrypt** — it is
not a password, it is the key material. Treat it like one.

### Bootstrap order (read this before changing anything)

On every launch, `models.GetOrCreateSecretKey` resolves the key in this
order:

1. `REPLOG_SECRET_KEY` env var, if non-empty. The value is also written
   to the `_internal.secret_key` row in `app_settings` so the DB stays
   self-sufficient for backups.
2. Otherwise, the `_internal.secret_key` row in `app_settings`, if
   present.
3. Otherwise, a freshly-generated 32-byte random key, written to
   `_internal.secret_key` and used as the key going forward.

The startup log line tells you which path was taken
(`Secret key generated and stored in database` /
`loaded from database` / `loaded from REPLOG_SECRET_KEY environment variable`).

### "I lost the key, can I recover the encrypted settings?"

**No.** AES-GCM with a lost key is not recoverable. Your options are:

1. Restore a DB backup from before the key was lost (the key row
   travels with the backup, assuming you didn't separately set the env
   var to something different).
2. Re-enter the lost values through the admin Settings page. They will
   be re-encrypted with the current key.

This is why the recommended posture is auto-generate-and-back-up, not
manage-the-key-out-of-band.

### Rotating the key

There is currently **no automated key-rotation tool**. The encrypted
ciphertexts in `app_settings` cannot be re-encrypted in place without
the old key. The supported rotation procedure is:

1. **Take a backup.** This is your rollback.
2. From the admin Settings page, copy any sensitive values you still
   know (LLM keys, SMTP password) somewhere safe.
3. Stop the service.
4. In the DB, clear the encrypted settings and the existing key row:
   ```sql
   DELETE FROM app_settings WHERE key = '_internal.secret_key';
   DELETE FROM app_settings WHERE value LIKE 'enc:%';
   ```
5. Set the new `REPLOG_SECRET_KEY` env var (or unset it to
   auto-generate).
6. Start the service. Re-enter the sensitive values via the admin UI.

If the values are not recoverable (lost LLM key etc.), rotation is the
same as recovery — re-enter what you can, regenerate what you can't.

A future enhancement (no ETA) would add a `replog rotate-secret-key`
admin command that takes both old and new keys and rewrites every `enc:`
ciphertext in a transaction.

---

## Upgrades

### Standard upgrade flow

1. **Check for breaking changes.** Read the release notes / commit log
   between your current and target version. Pay attention to:
   - Migration files in `internal/database/migrations/` — anything
     beyond the highest version you've already applied will run
     automatically on next start.
   - ADRs in `docs/adr/` — high-level architecture decisions land here.
2. **Take a fresh backup** (see above). Always immediately before an
   upgrade.
3. **Deploy the new binary.** On NixOS, `nixos-rebuild switch` after
   bumping the flake input. On Docker, pull the new image and restart
   the container. The binary is fully self-contained — no separate
   frontend deploy step.
4. **Watch the logs.** A successful upgrade looks like:
   ```
   Database ready: /var/lib/replog/replog.db
   Secret key loaded from database
   RepLog 0.X.Y (commit, date) listening on :8080
   ```
   If you see migration output (`OK 000N_*.sql`), that's expected for
   any version bump that ships new migrations.
5. **Smoke test.** Hit `/healthz`, `/readyz`, log in, load the
   dashboard, log a workout.

### Migration policy

Per [ADR 002](adr/002-migrations.md): once an instance is in
production, all schema changes are **additive** — new files
(`0002_*.sql`, `0003_*.sql`, ...) appended to
`internal/database/migrations/`, never edits to existing files.
`pressly/goose` runs them in order on startup. There is no manual
migration step.

### Rolling back

If a release misbehaves:

1. Stop the service.
2. Restore the pre-upgrade DB backup (see [restore](#restoring-from-backup)).
3. Redeploy the previous binary.
4. Start.

Note: rolling back **without** a DB restore is risky if the new version
ran a forward migration. If the new schema added a column the old
binary reads strictly, the old binary may panic. Always pair a binary
rollback with a DB rollback from backup.

---

## Logs and monitoring

### What to watch

The binary writes structured-ish access logs and event logs to stdout.
With systemd:

```bash
journalctl -u replog -f                    # follow
journalctl -u replog --since '1 hour ago'  # recent
journalctl -u replog -p err                # errors only
```

Lines worth alerting on:

- `Failed to run migrations:` — fatal at startup.
- `Failed to bootstrap admin:` — fatal at startup, usually means
  `REPLOG_ADMIN_*` is missing on an empty DB.
- `api: session renew error` / `api: ensure preferences` — non-fatal
  but indicates DB pressure or a race.
- A burst of `429 Too Many Requests` on `/api/login` — possible
  password-spray. Check the source IP and the `REPLOG_TRUSTED_PROXIES`
  config.
- Anything matching `Warning:` at startup.

### Health endpoints

| Path | Purpose | Use |
|------|---------|-----|
| `GET /healthz` | Liveness | Container/orchestrator restart probe. Returns `200 ok` if the process is up. |
| `GET /readyz` | Readiness | Pre-traffic check. Returns `200 ok` only if the DB ping succeeds. |

Both are **public** (no auth). Don't expose them through the public
proxy if you don't need to — bind them to an internal listener instead
(future enhancement).

### Sensitive paths are redacted

The access logger redacts magic-link login tokens. A request to
`/api/auth/token/abc123def...` is logged as
`/api/auth/token/<redacted>`. If you add a new route that carries a
secret in the path, append the prefix to `redactedPaths` in
`internal/middleware/logging.go`.

### Metrics

There is no Prometheus endpoint yet. If/when added it will live at
`/metrics` behind an internal-only listener.

---

## Reverse proxy notes

The binary is intended to live behind a TLS-terminating reverse proxy
(Caddy, nginx, Traefik). It does not handle TLS itself.

### Required headers

The proxy must forward (and the binary must trust) at least:

- `Host` — used for absolute URL generation when `REPLOG_BASE_URL` is
  not set.
- `X-Forwarded-Proto` — informational; does not toggle Secure cookies
  (use `REPLOG_BASE_URL` or `REPLOG_SECURE_COOKIES` for that).
- `X-Forwarded-For` — the rate limiter uses this **only when** the
  immediate peer's IP is in `REPLOG_TRUSTED_PROXIES` (CIDR or bare IP).
  Without that allow-list, the binary uses the direct `RemoteAddr` so
  upstream clients cannot spoof their IP to evade rate limiting.

### Caddy example

```caddy
replog.example.com {
    reverse_proxy 127.0.0.1:8080 {
        header_up X-Forwarded-For {remote}
        header_up X-Forwarded-Proto {scheme}
    }
}
```

### nginx example

```nginx
server {
    listen 443 ssl http2;
    server_name replog.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Then on the binary side: `REPLOG_TRUSTED_PROXIES=127.0.0.1/32`.

### WebAuthn caveat

`REPLOG_WEBAUTHN_RPID` and `REPLOG_WEBAUTHN_ORIGINS` are validated
strictly by the WebAuthn library against what the browser sends. If
your reverse proxy serves a different hostname than the configured
RPID, passkey registration will fail with an opaque
`origin not allowed` error. Rule of thumb: `RPID` is the bare host
(`replog.example.com`), `ORIGINS` is the full scheme+host
(`https://replog.example.com`).

---

## Disaster recovery

The minimum-viable recovery story, ordered from "lost the binary" to
"lost everything":

| What you lost | Recovery |
|--------------|----------|
| The binary | Rebuild from source (`nix build` or `go build`) or pull a release. State is on disk. |
| `web/dist/` | Same — embedded in the binary, rebuilt with the binary. |
| The avatar directory | Cosmetic. Restore from rsync backup if you took one; otherwise users re-upload. |
| The DB file | Restore from a SQLite `.backup` snapshot (see [restore](#restoring-from-backup)). All app state is here. |
| The host | Provision a new one, restore DB + avatars, point DNS at it. If `REPLOG_SECRET_KEY` is supplied via env var, set it on the new host first. |
| The `REPLOG_SECRET_KEY` env var (and not in DB) | Restore a DB backup taken when the env var was set, OR clear encrypted settings and re-enter via admin UI (see [secret key recovery](#replog_secret_key-bootstrap-rotation-and-recovery)). |
| Everything | Cry, then restore from off-host backups. Provision host, install binary, copy DB and avatars into place, start service. |

### Recovery time objective

For a single-family deployment: provisioning a fresh host + restoring
DB + restarting service is ~10 minutes assuming you have off-host
backups and an Ansible/NixOS recipe. Without those, longer.

---

## Routine maintenance

### Background scheduler

The binary runs a background scheduler (`internal/scheduler`) that wakes
on the configured interval (default 24 h) and:

- Deletes login tokens whose `expires_at` is in the past (also: tokens
  consumed by `ValidateLoginToken` are deleted immediately as part of
  single-use semantics).
- Prunes notification rows older than the configured retention (default
  90 d).

Both interval and retention are admin-tunable in the Settings page; the
defaults are fine.

### Clearing a per-account login lockout

Per [ADR 014](adr/014-login-lockout.md), an account is temporarily
locked for **15 minutes** after **5 consecutive wrong-password
attempts**. Locked logins return `429 Too Many Requests` with a
`Retry-After` header. The lockout slides on each additional attempt.

Most users should just wait 15 minutes. To clear a lockout manually
(e.g. an admin needs in immediately and the password is known):

```bash
sqlite3 /var/lib/replog/replog.db <<'SQL'
UPDATE users
   SET failed_login_count = 0,
       locked_until       = NULL
 WHERE username = 'admin';   -- adjust as needed
SQL
```

A successful password change via the admin UI also clears the lockout
(`UpdatePassword` resets both columns), so the supported recovery path
without DB access is "have an admin reset the password."

### `PRAGMA optimize`

The binary runs `PRAGMA optimize` on graceful shutdown so the next
startup benefits from accurate query-planner statistics. If your
deployment crashes / kills the binary frequently, run it manually
occasionally:

```bash
sqlite3 /var/lib/replog/replog.db 'PRAGMA optimize;'
```

### Vacuum

WAL-mode SQLite reclaims space lazily. For a small family-scale DB this
is unlikely to matter, but if `replog.db` ever grows surprisingly large
relative to actual content:

```bash
sudo systemctl stop replog
sqlite3 /var/lib/replog/replog.db 'VACUUM;'
sudo systemctl start replog
```

`VACUUM` rewrites the entire DB and requires a brief exclusive lock —
do it during downtime.

### Log rotation

If you use journald, it handles rotation. If you redirect to a flat
file, plug in `logrotate`:

```
/var/log/replog/*.log {
    daily
    rotate 14
    compress
    missingok
    notifempty
    copytruncate
}
```

(`copytruncate` because the binary holds the log fd open and there is
no SIGHUP-reopen handler today.)

---

## Security checklist

Run through this once after first deployment and again after any infra
change.

- [ ] `REPLOG_BASE_URL` is `https://...` (or `REPLOG_SECURE_COOKIES=true`
      is set) so session cookies are flagged `Secure` and HSTS is sent.
- [ ] `REPLOG_ADDR` binds to a non-public interface
      (`127.0.0.1:8080`) and only the reverse proxy can reach it.
- [ ] `REPLOG_TRUSTED_PROXIES` is set to your proxy's address(es) — and
      **only** to those — so `X-Forwarded-For` is trusted there and
      nowhere else.
- [ ] The bootstrap admin password has been rotated to something real,
      and `REPLOG_ADMIN_PASS` no longer appears in the systemd
      EnvironmentFile.
- [ ] The admin user has registered at least one passkey.
- [ ] `REPLOG_WEBAUTHN_RPID` matches the bare hostname in
      `REPLOG_BASE_URL`; `REPLOG_WEBAUTHN_ORIGINS` matches the full
      scheme+host. Passkey registration succeeds end-to-end.
- [ ] `REPLOG_SECRET_KEY` strategy is decided and documented (auto-gen
      vs env-supplied), and any env-supplied value is in a backed-up
      vault.
- [ ] DB backups are running on a timer, integrity-checked at least
      quarterly, and copied off-host.
- [ ] Reverse proxy is forwarding `Host`, `X-Forwarded-For`, and
      `X-Forwarded-Proto`.
- [ ] Health endpoints `/healthz` and `/readyz` are reachable from
      whatever monitors them, but ideally not from the public internet.
- [ ] The data directory is owned by the service user and chmod 750.
- [ ] systemd unit (or container) sets `NoNewPrivileges`, `PrivateTmp`,
      `ProtectSystem=strict`, `ProtectHome` — the shipped NixOS module
      does this for you.
