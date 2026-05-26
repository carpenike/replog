{
  description = "RepLog — self-hosted workout tracking for a single family";

  inputs = {
    # Pinned to nixos-25.11 to match the consumer channel
    # (carpenike/nix-config). Override with
    #   inputs.replog.inputs.nixpkgs.follows = "nixpkgs";
    # if your consumer is on a different channel.
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      # The deployment target is NixOS on x86_64-linux (Forge). The
      # aarch64-linux and aarch64-darwin entries exist so dev machines
      # (M-series Macs, rydev/nixpi) can `nix run` for smoke testing.
      # Adding more systems is cheap — modernc.org/sqlite is pure Go,
      # so the binary builds on every Go target without CGO.
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin" ];
      forAllSystems = f:
        nixpkgs.lib.genAttrs supportedSystems
          (system: f {
            inherit system;
            pkgs = import nixpkgs { inherit system; };
          });

      mkPackage = pkgs: pkgs.callPackage ./nix/package.nix {
        # Bake the flake's revision into the binary so `replog --version`
        # (and any future build-info endpoint) report the deployed
        # commit. Falls through to `dirtyRev` for uncommitted source
        # trees, then "unknown" when neither is available.
        gitRev = self.rev or self.dirtyRev or "unknown";
        buildTime = self.lastModifiedDate or "unknown";
      };
    in
    {
      packages = forAllSystems ({ pkgs, ... }: {
        default = mkPackage pkgs;
        replog = mkPackage pkgs;
      });

      # `nix run github:carpenike/replog`
      apps = forAllSystems ({ system, ... }: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/replog";
        };
      });

      # `nix flake check` will build the package on every supported
      # system. Tests are NOT re-run here — `just qa` in CI is the
      # authoritative test gate (see nix/package.nix → doCheck).
      checks = forAllSystems ({ system, ... }: {
        package = self.packages.${system}.default;
      });

      devShells = forAllSystems ({ pkgs, ... }: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            gotools
            go-swag
            golangci-lint
            goreleaser
            sqlite
            nodejs_22
            just
            # Refresh web/package-lock.json hash in nix/package.nix with:
            #   nix-shell -p prefetch-npm-deps --run \
            #     "prefetch-npm-deps ./web/package-lock.json"
            prefetch-npm-deps
          ];
        };
      });

      # NixOS module — import via:
      #   imports = [ inputs.replog.nixosModules.default ];
      # Then enable + configure under `services.replog`.
      nixosModules.default = import ./nix/module.nix;

      # Convenience overlay so callers can do `pkgs.replog` after
      # adding `inputs.replog.overlays.default` to their nixpkgs
      # config.
      overlays.default = final: _prev: {
        replog = mkPackage final;
      };
    };
}
