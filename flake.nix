{
  description = "CLI for searching packages on search.nixos.org";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";

    flake-utils.url = "github:numtide/flake-utils";

    flake-compat.url = "github:edolstra/flake-compat";
    flake-compat.flake = false;

    gomod2nix.url = "github:nix-community/gomod2nix";
    gomod2nix.inputs.nixpkgs.follows = "nixpkgs";
    gomod2nix.inputs.utils.follows = "flake-utils";
  };

  outputs = inputs @ {...}:
    inputs.flake-utils.lib.eachDefaultSystem
    (
      system: let
        overlays = [
          inputs.gomod2nix.overlays.default
        ];
        pkgs = import inputs.nixpkgs {
          inherit system overlays;
        };
        # warnToUpgrade = pkgs.lib.warn "Please upgrade Nix to 2.7 or later.";
      in rec {
        packages = rec {
          nix-search = pkgs.buildGoApplication {
            pname = "nix-search-cli";
            version = "0.1.0";
            src = ./.;
            modules = ./gomod2nix.toml;
          };
          default = nix-search;
        };
        defaultPackage = packages.default;

        apps = rec {
          nix-search = {
            type = "app";
            program = "${packages.nix-search}/bin/nix-search";
          };
          default = nix-search;
        };
        defaultApp = apps.default;

        devShells = rec {
          default = pkgs.mkShell {
            packages = with pkgs; [
              ## golang
              delve
              go-outline
              go
              golangci-lint
              gopkgs
              gopls
              gotools
              ## nix
              gomod2nix
              rnix-lsp
              nixpkgs-fmt
              ## other tools
              just
            ];

            shellHook = ''
              # The path to this repository
              if [ -z $WORKSPACE_ROOT ]; then
                shell_nix="''${IN_LORRI_SHELL:-$(pwd)/shell.nix}"
                workspace_root=$(dirname "$shell_nix")
                export WORKSPACE_ROOT="$workspace_root"
              fi

              # We put the $GOPATH/$GOCACHE/$GOENV in $TOOLCHAIN_ROOT,
              # and ensure that the GOPATH's bin dir is on our PATH so tools
              # can be installed with `go install`.
              #
              # Any tools installed explicitly with `go install` will take precedence
              # over versions installed by Nix due to the ordering here.
              export TOOLCHAIN_ROOT="$WORKSPACE_ROOT/.toolchain"
              export GOROOT=
              export GOCACHE="$TOOLCHAIN_ROOT/go/cache"
              export GOENV="$TOOLCHAIN_ROOT/go/env"
              export GOPATH="$TOOLCHAIN_ROOT/go/path"
              export GOMODCACHE="$GOPATH/pkg/mod"
              export PATH=$(go env GOPATH)/bin:$PATH
              # This project is pure go and does not need CGO. We disable it
              # here as well as in the Dockerfile and nix build scripts.
              export CGO_ENABLED=0
            '';

            # Need to disable fortify hardening because GCC is not built with -oO,
            # which means that if CGO_ENABLED=1 (which it is by default) then the golang
            # debugger fails.
            # see https://github.com/NixOS/nixpkgs/pull/12895/files
            hardeningDisable = ["fortify"];
          };
        };
        devShell = devShells.default;
      }
    );
}
