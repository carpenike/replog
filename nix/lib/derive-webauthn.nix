# nix/lib/derive-webauthn.nix
#
# Pure function that derives the WebAuthn env-var trio from a single
# baseUrl. Extracted from the NixOS module so it can be unit-tested
# without dragging in the full NixOS module system.
#
# Inputs:
#   lib       — nixpkgs lib
#   baseUrl   — null | string (e.g. "https://replog.example.com:8443")
#   settings  — attrs of user-supplied env overrides; any of
#               REPLOG_WEBAUTHN_RPID / REPLOG_WEBAUTHN_ORIGINS present
#               here suppresses the corresponding derivation.
#
# Output:
#   attrs to merge into the systemd unit's `environment`. Empty when
#   baseUrl is null. Keys derived:
#     REPLOG_BASE_URL          — always (when baseUrl != null)
#     REPLOG_WEBAUTHN_RPID     — bare hostname; suppressed by override
#     REPLOG_WEBAUTHN_ORIGINS  — scheme://host[:port]; suppressed by override
#
# Why this exists as a separate file: the original inline derivation
# in nix/module.nix computed ORIGINS via
#   lib.head (lib.splitString "/" cfg.baseUrl)
# which returns "https:" for "https://x" because the "//" yields two
# empty list elements. The bug shipped, broke passkey registration on
# Forge, and nothing caught it because the only previous gate was
# `nix build` (which doesn't evaluate the module). Moving the logic
# here lets `nix/tests/module-baseurl.nix` evaluate it with no NixOS
# scaffolding and assert every derived value byte-for-byte.

lib: baseUrl: settings:

if baseUrl == null then
  { }
else
  let
    hasHttps = lib.hasPrefix "https://" baseUrl;
    scheme = if hasHttps then "https" else "http";
    stripped = lib.removePrefix "https://" (lib.removePrefix "http://" baseUrl);
    # hostPort keeps an explicit ":port" if the operator included one
    # (e.g. "http://localhost:5008"). The WebAuthn spec requires ports
    # in the Origin header but forbids them in RPID, so RPID drops the
    # ":port" suffix below.
    hostPort = lib.head (lib.splitString "/" stripped);
    host = lib.head (lib.splitString ":" hostPort);
    origin = "${scheme}://${hostPort}";
  in
  {
    REPLOG_BASE_URL = baseUrl;
  } // lib.optionalAttrs (! (settings ? REPLOG_WEBAUTHN_RPID)) {
    REPLOG_WEBAUTHN_RPID = host;
  } // lib.optionalAttrs (! (settings ? REPLOG_WEBAUTHN_ORIGINS)) {
    REPLOG_WEBAUTHN_ORIGINS = origin;
  }
