# nix/module.nix
#
# NixOS module for RepLog — self-hosted workout tracking.
#
# Typical consumer wiring (e.g. in carpenike/nix-config):
#
#   # flake.nix
#   inputs.replog = {
#     url = "github:carpenike/replog";
#     inputs.nixpkgs.follows = "nixpkgs";
#   };
#
#   # hosts/forge/services/replog.nix
#   { config, inputs, pkgs, ... }:
#   {
#     imports = [ inputs.replog.nixosModules.default ];
#
#     services.replog = {
#       enable = true;
#       package = inputs.replog.packages.${pkgs.system}.default;
#
#       baseUrl = "https://replog.holthome.net";   # auto-derives WebAuthn
#       port    = 5008;
#
#       # Secrets — root-readable EnvironmentFile (sops-nix / agenix).
#       # Required keys:
#       #   REPLOG_ADMIN_USER=...      (only required on first boot;
#       #   REPLOG_ADMIN_PASS=...       safe to remove after the first
#       #   REPLOG_ADMIN_EMAIL=...      successful login)
#       #
#       # Optional:
#       #   REPLOG_SECRET_KEY=...      (auto-generated and persisted to
#       #                               the DB on first boot if absent;
#       #                               only set if you want to control
#       #                               the value yourself, e.g. for
#       #                               cross-host DB restores)
#       environmentFile = config.sops.templates."replog-env".path;
#     };
#   }
#
# Design notes — where this module deliberately differs from the sister
# `whiskey-whiskey-whiskey` module:
#
# * `environmentFile` is OPTIONAL. RepLog's only true secret
#   (`REPLOG_SECRET_KEY`) auto-generates on first boot and persists into
#   the SQLite DB itself, so a minimal "run it on a NixOS VM" smoke test
#   doesn't need any sops/agenix plumbing at all.
# * `baseUrl` is a convenience option that auto-derives
#   `REPLOG_WEBAUTHN_RPID` + `REPLOG_WEBAUTHN_ORIGINS`. Three coupled
#   env vars behind one option — passkeys silently break at the browser
#   layer if these three drift apart, and there's no useful log line
#   when it happens.
# * No `logLevel` option — replog uses stdlib `log.Printf`, no levels.
# * No maintenance-script template unit — daily token cleanup +
#   notification pruning runs in-process via the embedded scheduler.
# * `MemoryDenyWriteExecute = true` — Go has no JIT, stricter sandbox
#   is free.

{ config, lib, pkgs, ... }:

let
  cfg = config.services.replog;

  # If baseUrl is set and the user hasn't pinned the WebAuthn settings
  # themselves, derive them. Keeps the three coupled env vars in lock
  # step without taking control away.
  baseUrlAttrs =
    if cfg.baseUrl == null then
      { }
    else
      let
        # Parse out the host part (everything between the scheme and the
        # first '/' or ':port'). RPID must be a bare hostname, no scheme,
        # no port. Origins is the full origin (scheme + host + optional
        # port), no trailing slash.
        stripped = lib.removePrefix "https://" (lib.removePrefix "http://" cfg.baseUrl);
        host = lib.head (lib.splitString "/" (lib.head (lib.splitString ":" stripped)));
        origin = lib.head (lib.splitString "/" cfg.baseUrl);
      in
      {
        REPLOG_BASE_URL = cfg.baseUrl;
      } // lib.optionalAttrs (! (cfg.settings ? REPLOG_WEBAUTHN_RPID)) {
        REPLOG_WEBAUTHN_RPID = host;
      } // lib.optionalAttrs (! (cfg.settings ? REPLOG_WEBAUTHN_ORIGINS)) {
        REPLOG_WEBAUTHN_ORIGINS = origin;
      };

  # Final environment merged into the systemd unit. Order matters:
  # bake-in defaults < baseUrl-derived < user-supplied settings.
  finalEnv = {
    REPLOG_DB_PATH = "${cfg.dataDir}/replog.db";
    REPLOG_AVATAR_DIR = "${cfg.dataDir}/avatars";
    REPLOG_ADDR = "${cfg.host}:${toString cfg.port}";
  } // baseUrlAttrs // lib.mapAttrs (_n: v: toString v) cfg.settings;
in
{
  options.services.replog = {
    enable = lib.mkEnableOption "RepLog — self-hosted workout tracking";

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.callPackage ./package.nix { };
      defaultText = lib.literalExpression "pkgs.callPackage ./nix/package.nix { }";
      description = ''
        The replog package to run. Consumers using the flake's overlay
        can leave this at default; otherwise set it to
        `inputs.replog.packages.<system>.default` so the binary built
        by upstream CI is what actually deploys.
      '';
    };

    host = lib.mkOption {
      type = lib.types.str;
      default = "127.0.0.1";
      description = ''
        Interface to bind. Default `127.0.0.1` — the standard topology
        is a reverse proxy on the same box (Caddy / nginx) forwarding
        to localhost. Override only if you need LAN access without a
        proxy in front.
      '';
    };

    port = lib.mkOption {
      type = lib.types.port;
      default = 8080;
      description = "TCP port to bind on `host`.";
    };

    dataDir = lib.mkOption {
      type = lib.types.path;
      default = "/var/lib/replog";
      description = ''
        Where the SQLite database (`replog.db` + WAL/SHM sidecars) and
        the uploaded avatars live. Defaults to the systemd
        StateDirectory; override only if you bind-mount a ZFS dataset
        somewhere else.
      '';
    };

    baseUrl = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      example = "https://replog.holthome.net";
      description = ''
        The external URL the SPA is served from. Convenience option
        that:

          * sets `REPLOG_BASE_URL` (used to generate absolute URLs for
            login-token magic links and to auto-derive whether the
            session cookie should be flagged `Secure`)
          * auto-derives `REPLOG_WEBAUTHN_RPID` (bare hostname) and
            `REPLOG_WEBAUTHN_ORIGINS` (scheme + host) so passkeys
            actually work without the operator manually keeping three
            env vars in sync. Set either of those in `settings`
            explicitly to override the derivation.

        Leave null if the deployment is reached at multiple hostnames
        or if you want to pin all three values yourself in `settings`.
      '';
    };

    settings = lib.mkOption {
      type = with lib.types; attrsOf (oneOf [ str int bool ]);
      default = { };
      example = lib.literalExpression ''
        {
          # Trusted reverse-proxy CIDRs (so X-Forwarded-For is honored
          # for rate-limit accounting).
          REPLOG_TRUSTED_PROXIES = "127.0.0.1,10.0.0.0/8";

          # Override the auto-derived WebAuthn settings if the
          # deployment is reachable at more than one origin (e.g. LAN
          # + tunnel):
          # REPLOG_WEBAUTHN_ORIGINS = "https://replog.example.com,https://replog.lan";
        }
      '';
      description = ''
        Non-secret environment variables passed to the service. Use
        this for any `REPLOG_*` knob that isn't covered by a top-level
        option above.

        Values appear in the world-readable Nix store — never put
        secrets here. Use `environmentFile` for `REPLOG_SECRET_KEY`,
        admin-bootstrap credentials, or anything else sensitive.
      '';
    };

    environmentFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      description = ''
        Optional path to a root-readable EnvironmentFile carrying
        secret config. systemd reads it before dropping to the
        DynamicUser, so it must be `0400 root:root` (sops-nix /
        agenix defaults are correct).

        Optional keys:

          REPLOG_ADMIN_USER=admin
          REPLOG_ADMIN_PASS=<initial password>
          REPLOG_ADMIN_EMAIL=admin@example.com

            Required ONLY on the very first boot (when no users exist
            in the DB). Safe to remove from the env file after the
            first successful login — replog ignores them once a user
            row exists.

          REPLOG_SECRET_KEY=<32+ random bytes, base64 or hex>

            Encrypts sensitive settings stored in the DB (LLM API
            keys, etc.). Auto-generated and persisted to the DB on
            first boot if absent. Only set this if you want to
            control the value yourself — e.g. you're restoring a
            DB backup onto a fresh host and the original key lived
            elsewhere.

        Leaving this option `null` is fine for greenfield deploys:
        the very first run will auto-generate the secret key and you
        can bootstrap admin via `settings` if you don't mind the
        plaintext-in-Nix-store trade-off for first-boot credentials.
      '';
    };

    openFirewall = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = ''
        Open `port` in the host firewall. Default off — the assumed
        topology is reverse-proxy on the same box, so opening the
        port only invites trouble.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.replog = {
      description = "RepLog — self-hosted workout tracking";
      wantedBy = [ "multi-user.target" ];
      after = [ "network.target" ];

      environment = finalEnv;

      serviceConfig = {
        ExecStart = lib.getExe cfg.package;
        Restart = "on-failure";
        RestartSec = "5s";

        # Sandboxing. DynamicUser allocates an ephemeral UID per boot;
        # StateDirectory creates /var/lib/replog and chowns it to that
        # UID. The explicit `User = "replog";` lets us share the same
        # identity slot if we ever add maintenance oneshots later
        # (systemd.exec(5): "this name is shared by other units using
        # the same User= setting, allowing simple sharing of state").
        DynamicUser = true;
        User = "replog";
        StateDirectory = "replog";
        StateDirectoryMode = "0700";

        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
        PrivateDevices = true;
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectControlGroups = true;
        ProtectClock = true;
        ProtectHostname = true;
        ProtectKernelLogs = true;
        ProtectProc = "invisible";
        NoNewPrivileges = true;
        RestrictRealtime = true;
        RestrictSUIDSGID = true;
        RestrictNamespaces = true;
        LockPersonality = true;
        # Go has no JIT — unlike Node we can refuse W+X mappings
        # outright.
        MemoryDenyWriteExecute = true;
        SystemCallFilter = [ "@system-service" "~@privileged" "~@resources" ];
        SystemCallArchitectures = "native";
        RestrictAddressFamilies = [ "AF_INET" "AF_INET6" "AF_UNIX" ];
        CapabilityBoundingSet = [ "" ];
        AmbientCapabilities = [ "" ];
      } // lib.optionalAttrs (cfg.environmentFile != null) {
        EnvironmentFile = cfg.environmentFile;
      };
    };

    networking.firewall.allowedTCPPorts = lib.mkIf cfg.openFirewall [ cfg.port ];
  };
}
