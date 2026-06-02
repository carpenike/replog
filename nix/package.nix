# nix/package.nix
#
# RepLog package definition.
#
# Two-stage build:
#   1. `frontend` — buildNpmPackage from ./web/ producing dist/ (the
#      static React bundle that gets embedded into the Go binary).
#   2. `buildGoModule` — copies the frontend output into web/dist/
#      before running `go build`, so the //go:embed in web/embed.go
#      sees a populated tree at compile time.
#
# Hashes that must be refreshed when the corresponding lockfile changes:
#   * npmDepsHash  — when web/package-lock.json changes. Refresh with:
#       nix-shell -p prefetch-npm-deps --run \
#         "prefetch-npm-deps ./web/package-lock.json"
#   * vendorHash   — when go.sum changes. Refresh by setting it to
#       lib.fakeHash and running `nix build`; nix prints the correct
#       value on the mismatch error.
#
# Build-time identity baked into the binary via -ldflags so /api/me,
# logs, and any future build-info endpoint can report the deployed
# commit. cmd/replog/main.go already declares the `version`, `commit`,
# and `date` vars; the flake passes self.rev / self.lastModifiedDate
# in for us.
{ lib
, buildGoModule
, buildNpmPackage
, nodejs_22
, gitRev ? "unknown"
, buildTime ? "unknown"
}:

let
  # Pin the frontend build to web/ only — keeps the npm closure stable
  # when Go-only changes happen, and prevents the .git tree (heavy)
  # from invalidating the buildNpmPackage cache.
  frontend = buildNpmPackage {
    pname = "replog-frontend";
    version = "0.1.0";

    src = lib.cleanSourceWith {
      src = ../web;
      filter = name: type:
        let base = baseNameOf (toString name); in
        # Drop noise that would invalidate the input-addressed hash on
        # every developer machine.
        base != "node_modules"
        && base != "dist"
        && base != ".vite";
    };

    npmDepsHash = "sha256-fPJQfmDjcRpCz320hFJa9EY08Qo02LkUe3EavvC3O5I=";

    nodejs = nodejs_22;

    # `npm run build` runs `tsc -b && vite build` and writes to dist/.
    npmBuildScript = "build";

    # buildNpmPackage's default installPhase would ship node_modules.
    # We only want the static dist/ tree — the Go binary embeds it and
    # everything else is dev-only.
    installPhase = ''
      runHook preInstall
      mkdir -p $out
      cp -r dist/. $out/
      runHook postInstall
    '';

    meta = with lib; {
      description = "RepLog frontend bundle (consumed by the Go binary via //go:embed)";
      license = licenses.mit;
    };
  };
in
buildGoModule {
  pname = "replog";
  version = "0.1.0";

  src = lib.cleanSource ../.;

  # Bump this when go.sum changes (set to lib.fakeHash to discover the
  # new value on the next build).
  vendorHash = "sha256-gvgFOIpWwY63acjiPAsb6+3+tjHIkzovc/o/1vE/rFc=";

  subPackages = [ "cmd/replog" ];

  # Drop the frontend bundle into web/dist/ before `go build`. The
  # `//go:embed all:dist` directive in web/embed.go reads from that
  # path at compile time; without this step the binary would ship an
  # empty embedded FS and the SPA fallback would 404.
  preBuild = ''
    mkdir -p web/dist
    cp -r ${frontend}/. web/dist/
  '';

  ldflags = [
    "-s"
    "-w"
    "-X=main.version=${gitRev}"
    "-X=main.commit=${gitRev}"
    "-X=main.date=${buildTime}"
  ];

  # CI (`just qa`, race-enabled) is the authoritative test gate on
  # every push. Re-running the suite in the Nix sandbox would add
  # ~20s to every `nix build` for zero additional signal and would
  # slow hash-fix iteration (mismatched vendorHash → rebuild → test).
  # `nix flake check` still verifies the binary actually compiles
  # for every supported system, which is the unique value of the
  # Nix build path here.
  doCheck = false;

  meta = with lib; {
    description = "Self-hosted workout tracking — kids on tier-based progression, adults on percentage programs";
    homepage = "https://github.com/carpenike/replog";
    license = licenses.mit;
    mainProgram = "replog";
    # modernc.org/sqlite is pure-Go, so this works on every supported
    # Go target. Linux is the deployment target; the macOS entries are
    # for developer-machine builds via `nix build`.
    platforms = platforms.linux ++ [ "aarch64-darwin" "x86_64-darwin" ];
  };
}
