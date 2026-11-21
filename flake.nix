{
  description = "RepLog — self-hosted workout tracking for a single family";

  inputs = {
    # Pinned to nixos-25.11 to match the consumer channel
    # (carpenike/nix-config). Override with
    #   inputs.replog.inputs.nixpkgs.follows = "nixpkgs";
    # if your consumer is on a different channel.
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
    # Go toolchain only. nixos-25.11's `go` stops on the 1.25 line.
    # nixos-26.05 carries go 1.27.4. Keep this input independent of
    # `nixpkgs.follows` so a consumer on 25.11 still builds with 1.27.4.
    nixpkgs-go.url = "github:NixOS/nixpkgs/nixos-26.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, nixpkgs-go, flake-utils }:
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
            pkgsGo = import nixpkgs-go { inherit system; };
          });

      mkPackage = pkgs: pkgsGo: pkgs.callPackage ./nix/package.nix {
        # Bake the flake's revision into the binary so `replog --version`
        # (and any future build-info endpoint) report the deployed
        # commit. Falls through to `dirtyRev` for uncommitted source
        # trees, then "unknown" when neither is available.
        gitRev = self.rev or self.dirtyRev or "unknown";
        buildTime = self.lastModifiedDate or "unknown";
        # nixos-25.11's buildGoModule, driven by go_1_27 from
        # nixpkgs-go. The default `go` on that channel is still the
        # 1.26 line. GOTOOLCHAIN=local inside buildGoModule, so the
        # go.mod version cannot download a different compiler.
        buildGoModule = pkgs.buildGoModule.override { go = pkgsGo.go_1_27; };
      };
    in
    {
      packages = forAllSystems ({ pkgs, pkgsGo, ... }: {
        default = mkPackage pkgs pkgsGo;
        replog = mkPackage pkgs pkgsGo;
      });

      # `nix run github:carpenike/replog`
      apps = forAllSystems ({ system, ... }: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/replog";
        };
      });

      # `nix flake check` will build the package on every supported
      # system and run the lightweight module eval-checks. Go tests are
      # NOT re-run here — `just qa` in CI is the authoritative test gate
      # (see nix/package.nix → doCheck).
      checks = forAllSystems ({ pkgs, system, ... }: {
        package = self.packages.${system}.default;
        module-baseurl = import ./nix/tests/module-baseurl.nix {
          inherit (pkgs) lib;
          inherit pkgs;
        };
      });

      devShells = forAllSystems ({ pkgs, pkgsGo, ... }: {
        default = pkgs.mkShell {
          packages = [
            # go_1_27 is 1.27.4 on nixos-26.05. The default `go`
            # attribute there is still 1.26, which cannot be the
            # toolchain this module requires. gopls and golangci-lint
            # from the same channel understand the 1.27 module.
            pkgsGo.go_1_27
            pkgsGo.gopls
            pkgsGo.golangci-lint
            pkgs.gotools
            pkgs.go-swag
            pkgs.goreleaser
            pkgs.sqlite
            pkgs.nodejs_22
            pkgs.just
            # Refresh web/package-lock.json hash in nix/package.nix with:
            #   nix-shell -p prefetch-npm-deps --run \
            #     "prefetch-npm-deps ./web/package-lock.json"
            pkgs.prefetch-npm-deps
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
