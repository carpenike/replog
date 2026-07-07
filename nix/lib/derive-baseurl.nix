# nix/lib/derive-baseurl.nix
#
# Pure function that maps the single `services.replog.baseUrl` option to
# the REPLOG_BASE_URL env var. Extracted from the NixOS module so it can
# be unit-tested without dragging in the full NixOS module system.
#
# Inputs:
#   lib      — nixpkgs lib (unused today; kept for signature stability
#              and any future URL parsing/validation)
#   baseUrl  — null | string (e.g. "https://replog.example.com")
#
# Output:
#   attrs to merge into the systemd unit's `environment`. Empty when
#   baseUrl is null; otherwise:
#     REPLOG_BASE_URL — the URL verbatim
#
# History: this file used to also derive REPLOG_WEBAUTHN_RPID /
# REPLOG_WEBAUTHN_ORIGINS from baseUrl. ADR 019 retired passkeys in
# favour of PocketID OIDC, the Go binary no longer reads any
# REPLOG_WEBAUTHN_* var, so that derivation was removed and the file
# renamed. The regression suite in nix/tests/module-baseurl.nix still
# gates `nix flake check` against the surviving base-URL passthrough.

lib: baseUrl:

if baseUrl == null then
  { }
else
  { REPLOG_BASE_URL = baseUrl; }
