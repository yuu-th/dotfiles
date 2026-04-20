{
  description = "My dev environment (macOS + NixOS) with Nix + nix-darwin + Home Manager";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    nix-darwin = {
      url = "github:LnL7/nix-darwin";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    disko = {
      url = "github:nix-community/disko";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    zen-browser = {
      url = "github:0xc000022070/zen-browser-flake";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.home-manager.follows = "home-manager";
    };

    # VS Code Insiders feeds - tracked in flake.lock
    vsci-feed-darwin-arm64 = { url = "https://update.code.visualstudio.com/api/update/darwin-arm64/insider/latest"; flake = false; };
    vsci-feed-darwin-x64   = { url = "https://update.code.visualstudio.com/api/update/darwin/insider/latest"; flake = false; };
    vsci-feed-linux-arm64  = { url = "https://update.code.visualstudio.com/api/update/linux-arm64/insider/latest"; flake = false; };
    vsci-feed-linux-x64    = { url = "https://update.code.visualstudio.com/api/update/linux-x64/insider/latest"; flake = false; };

  };

  outputs = inputs@{ self, nixpkgs, nix-darwin, home-manager, ... }: {

    # ── macOS (nix-darwin + Home Manager) ──────────────────────────────────────
    darwinConfigurations.yuta = nix-darwin.lib.darwinSystem {
      specialArgs = { inherit inputs; };
      modules = [ ./hosts/darwin/default.nix ];
    };

    # ── NixOS Server (GCP e2-micro) ───────────────────────────────────────────
    nixosConfigurations.server = nixpkgs.lib.nixosSystem {
      specialArgs = { inherit inputs; };
      modules = [ ./hosts/server/default.nix ];
    };

    # ── NixOS Box: AI coding agents (OrbStack VM) ─────────────────────────────
    nixosConfigurations.box-ai = nixpkgs.lib.nixosSystem {
      specialArgs = { inherit inputs; };
      modules = [ ./boxes/box-ai/default.nix ];
    };
  };
}
