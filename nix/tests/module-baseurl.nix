# nix/tests/module-baseurl.nix
#
# Regression test for nix/lib/derive-webauthn.nix — the pure function
# that derives REPLOG_BASE_URL / REPLOG_WEBAUTHN_RPID /
# REPLOG_WEBAUTHN_ORIGINS from the single `services.replog.baseUrl`
# option.
#
# History: the original inline derivation in nix/module.nix computed
# the origin via `lib.head (lib.splitString "/" cfg.baseUrl)` which
# returns "https:" for the input "https://replog.example" (because
# "//" produces two empty list elements after the colon). The bug
# shipped, broke passkey registration on Forge with "Error validating
# origin", and was caught by Coach during the first deploy smoke test.
# This test asserts every case byte-for-byte so it cannot regress.
#
# Wired into the flake's `checks` output, so `nix flake check` (and
# every CI run) refuses any change that mis-derives.
{ lib, pkgs }:

let
  derive = import ../lib/derive-webauthn.nix lib;

  cases = [
    {
      name = "https-bare-hostname";
      baseUrl = "https://replog.example.com";
      settings = { };
      expect = {
        REPLOG_BASE_URL = "https://replog.example.com";
        REPLOG_WEBAUTHN_ORIGINS = "https://replog.example.com";
        REPLOG_WEBAUTHN_RPID = "replog.example.com";
      };
    }
    {
      name = "http-localhost-with-port";
      baseUrl = "http://localhost:5008";
      settings = { };
      expect = {
        REPLOG_BASE_URL = "http://localhost:5008";
        # WebAuthn: port REQUIRED in Origin, FORBIDDEN in RPID.
        REPLOG_WEBAUTHN_ORIGINS = "http://localhost:5008";
        REPLOG_WEBAUTHN_RPID = "localhost";
      };
    }
    {
      name = "https-with-port";
      baseUrl = "https://replog.example.com:8443";
      settings = { };
      expect = {
        REPLOG_BASE_URL = "https://replog.example.com:8443";
        REPLOG_WEBAUTHN_ORIGINS = "https://replog.example.com:8443";
        REPLOG_WEBAUTHN_RPID = "replog.example.com";
      };
    }
    {
      # User-supplied REPLOG_WEBAUTHN_ORIGINS in `settings` must
      # suppress the derivation's ORIGINS key so the user value wins
      # at unit-environment merge time (settings is merged AFTER
      # baseUrlAttrs in nix/module.nix). RPID still derives because
      # it wasn't overridden.
      name = "user-override-origins-suppressed";
      baseUrl = "https://replog.example.com";
      settings.REPLOG_WEBAUTHN_ORIGINS = "https://a.example,https://b.example";
      expect = {
        REPLOG_BASE_URL = "https://replog.example.com";
        REPLOG_WEBAUTHN_RPID = "replog.example.com";
      };
      expectAbsent = [ "REPLOG_WEBAUTHN_ORIGINS" ];
    }
    {
      name = "user-override-rpid-suppressed";
      baseUrl = "https://replog.example.com";
      settings.REPLOG_WEBAUTHN_RPID = "example.com";
      expect = {
        REPLOG_BASE_URL = "https://replog.example.com";
        REPLOG_WEBAUTHN_ORIGINS = "https://replog.example.com";
      };
      expectAbsent = [ "REPLOG_WEBAUTHN_RPID" ];
    }
    {
      name = "null-baseurl-yields-empty";
      baseUrl = null;
      settings = { };
      expect = { };
      expectAbsent = [ "REPLOG_BASE_URL" "REPLOG_WEBAUTHN_ORIGINS" "REPLOG_WEBAUTHN_RPID" ];
    }
  ];

  checkCase = case:
    let
      got = derive case.baseUrl case.settings;
      missing = lib.concatLists (lib.mapAttrsToList
        (key: want:
          if got ? ${key} && got.${key} == want then [ ] else [
            "  [${case.name}] got.${key} = ${toString (got.${key} or "<unset>")}, want ${toString want}"
          ])
        case.expect);
      absentList = case.expectAbsent or [ ];
      unwanted = lib.concatLists (map
        (key:
          if got ? ${key} then
            [ "  [${case.name}] got.${key} = ${got.${key}}, expected unset" ]
          else
            [ ])
        absentList);
    in
    missing ++ unwanted;

  failures = lib.concatLists (map checkCase cases);
in
if failures == [ ] then
  pkgs.runCommand "replog-module-baseurl-ok" { } "echo OK > $out"
else
  throw ''

    nix/tests/module-baseurl.nix: ${toString (lib.length failures)} assertion(s) failed.
    ${lib.concatStringsSep "\n" failures}
  ''
