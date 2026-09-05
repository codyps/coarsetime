{
  inputs = {
    nixpkgs.url = "nixpkgs/nixpkgs-26.05-darwin";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, flake-utils, nixpkgs }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = ((import nixpkgs) {
          inherit system;
        });
        lib = pkgs.lib;
        stdenv = pkgs.stdenv;
      in
      {
        formatter = pkgs.nixpkgs-fmt;

        devShells.default = pkgs.mkShell {
          nativeBuildInputs = with pkgs; [
            go
            gopls
          ];
        };
      }
    );
}
