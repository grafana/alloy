{
  description = "Grafana Alloy development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

    # Helm 4 renders the golden chart tests in operations/helm/tests differently, and
    # unstable only ships Helm 4. Take Helm 3 from this channel until the chart moves.
    nixpkgs-helm3.url = "github:NixOS/nixpkgs/nixos-26.05";
  };

  # Tool versions are pinned by flake.lock. To bump them, run:
  #
  #   nix flake update
  outputs =
    { self, nixpkgs, nixpkgs-helm3 }:
    let
      # x86_64-darwin (Intel Macs) is not supported: nixpkgs-unstable dropped it after 26.05.
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      devShells = forAllSystems (
        pkgs:
        let
          # Tool groups. Each CI job selects the smallest shell that covers its needs
          # (`nix develop .#<name>`), so that jobs don't have to fetch unrelated tools.
          # Node.js follows internal/web/ui/.nvmrc (which Renovate bumps). Only the major
          # version is honored, the patch version comes from nixpkgs.
          nodeMajor = builtins.head (
            pkgs.lib.splitString "." (
              pkgs.lib.removePrefix "v" (pkgs.lib.removeSuffix "\n" (builtins.readFile ./internal/web/ui/.nvmrc))
            )
          );

          groups = rec {
            go = [ pkgs.go ];
            task = go ++ [ pkgs.go-task ];
            lint = task ++ [
              pkgs.golangci-lint
              pkgs.govulncheck
              pkgs.shellcheck
            ];
            ui = [ pkgs.${"nodejs_" + nodeMajor} ];
            release = go ++ ui;
            # Helm chart generation and testing.
            helm = go ++ [
              nixpkgs-helm3.legacyPackages.${pkgs.stdenv.hostPlatform.system}.kubernetes-helm
              pkgs.helm-docs
              pkgs.chart-testing
              pkgs.yamllint
            ];
            k8s = helm ++ [ pkgs.kind ];
            all = pkgs.lib.unique (lint ++ ui ++ k8s);
          };

          # The shells provide tools only, not a C toolchain: cgo builds keep using the
          # system compiler and headers (e.g. libsystemd-dev).
          mkShell =
            packages:
            pkgs.mkShellNoCC {
              inherit packages;
              # Bin directories of just the tools above (no stdenv utilities). CI puts
              # these on PATH, see .github/actions/setup-nix.
              TOOLS_PATH = pkgs.lib.makeBinPath packages;
              # Never let Go download a different toolchain than the one provided here.
              GOTOOLCHAIN = "local";
            };
        in
        (builtins.mapAttrs (_: mkShell) groups) // { default = mkShell groups.all; }
      );

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
