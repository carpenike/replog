# nix/tests/module-baseurl.nix
#
# Regression test for nix/lib/derive-baseurl.nix — the pure function
# that maps the single `services.replog.baseUrl` option to
# REPLOG_BASE_URL.
#
# History: this file used to also assert the WebAuthn env trio
# (REPLOG_WEBAUTHN_RPID / REPLOG_WEBAUTHN_ORIGINS) that baseUrl once
# derived. ADR 019 retired passkeys for PocketID OIDC and the Go binary
# no longer reads any REPLOG_WEBAUTHN_* var, so those cases were dropped
# along with the derivation. What remains is the base-URL passthrough:
# null yields no env, a set value passes through verbatim.
#
# Wired into the flake's `checks` output, so `nix flake check` (and
# every CI run) refuses any change that mis-derives.
{ lib, pkgs }:

let
  derive = import ../lib/derive-baseurl.nix lib;

  cases = [
    {
      name = "https-bare-hostname";
      baseUrl = "https://replog.example.com";
      expect = {
        REPLOG_BASE_URL = "https://replog.example.com";
      };
    }
    {
      name = "http-localhost-with-port";
      baseUrl = "http://localhost:5008";
      expect = {
        REPLOG_BASE_URL = "http://localhost:5008";
      };
    }
    {
      name = "https-with-port";
      baseUrl = "https://replog.example.com:8443";
      expect = {
        REPLOG_BASE_URL = "https://replog.example.com:8443";
      };
    }
    {
      name = "null-baseurl-yields-empty";
      baseUrl = null;
      expect = { };
      expectAbsent = [ "REPLOG_BASE_URL" ];
    }
  ];

  checkCase = case:
    let
      got = derive case.baseUrl;
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
