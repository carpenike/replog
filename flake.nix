{
  description = "RepLog — self-hosted workout tracking";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      # NixOS module — importable in your system configuration.
      nixosModule = { config, lib, pkgs, ... }:
        let
          cfg = config.services.replog;
        in
        {
          options.services.replog = {
            enable = lib.mkEnableOption "RepLog workout tracker";

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.system}.default;
              description = "The RepLog package to use.";
            };

            port = lib.mkOption {
              type = lib.types.port;
              default = 8080;
              description = "TCP port to listen on.";
            };

            dataDir = lib.mkOption {
              type = lib.types.path;
              default = "/var/lib/replog";
              description = "Directory for database and avatar storage.";
            };

            environment = lib.mkOption {
              type = lib.types.attrsOf lib.types.str;
              default = { };
              description = "Extra environment variables for RepLog.";
              example = {
                REPLOG_BASE_URL = "https://replog.example.com";
                REPLOG_WEBAUTHN_RPID = "replog.example.com";
                REPLOG_WEBAUTHN_ORIGINS = "https://replog.example.com";
              };
            };

            environmentFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "File containing secret environment variables (e.g., REPLOG_SECRET_KEY).";
            };
          };

          config = lib.mkIf cfg.enable {
            systemd.services.replog = {
              description = "RepLog workout tracker";
              after = [ "network.target" ];
              wantedBy = [ "multi-user.target" ];

              environment = {
                REPLOG_DB_PATH = "${cfg.dataDir}/replog.db";
                REPLOG_AVATAR_DIR = "${cfg.dataDir}/avatars";
                REPLOG_ADDR = ":${toString cfg.port}";
              } // cfg.environment;

              serviceConfig = {
                Type = "simple";
                ExecStart = "${cfg.package}/bin/replog";
                Restart = "on-failure";
                RestartSec = 5;

                # Hardening.
                DynamicUser = true;
                StateDirectory = "replog";
                StateDirectoryMode = "0750";
                ProtectSystem = "strict";
                ProtectHome = true;
                PrivateTmp = true;
                NoNewPrivileges = true;
                ReadWritePaths = [ cfg.dataDir ];
              } // lib.optionalAttrs (cfg.environmentFile != null) {
                EnvironmentFile = cfg.environmentFile;
              };
            };
          };
        };
    in
    (flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "replog";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-DI5kP09H/IrMMioqtCA3E5Wv9gqY0GoNchalBpYP8AU=";
          subPackages = [ "cmd/replog" ];

          meta = with pkgs.lib; {
            description = "Self-hosted workout tracking";
            license = licenses.mit;
            mainProgram = "replog";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
            goreleaser
            sqlite
          ];
        };
      }
    )) // {
      nixosModules.default = nixosModule;
    };
}
