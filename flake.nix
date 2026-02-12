{
  description = "RepLog — self-hosted workout tracking";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "replog";
          version = "0.1.0";
          src = ./.;
          vendorHash = null; # Update after first `go mod tidy`
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
            sqlite
          ];
        };
      }
    );
}
