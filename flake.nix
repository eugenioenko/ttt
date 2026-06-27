{
  description = "TTT Editor: Terminal Text Tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = self.shortRev or self.dirtyShortRev or "dev";
      in
      {
        packages = {
          ttt = pkgs.buildGoModule {
            pname = "ttt";
            inherit version;
            src = self;
            vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
            ];
            subPackages = [ "cmd/ttt" ];
            meta = with pkgs.lib; {
              description = "Terminal Text Tool — an IDE that lives in your terminal";
              homepage = "https://github.com/eugenioenko/ttt";
              license = licenses.mit;
              mainProgram = "ttt";
            };
          };
          default = self.packages.${system}.ttt;
        };
      }
    );
}
