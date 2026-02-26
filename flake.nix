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

    # VS Code Insiders feeds - tracked in flake.lock
    vsci-feed-darwin-arm64 = { url = "https://update.code.visualstudio.com/api/update/darwin-arm64/insider/latest"; flake = false; };
    vsci-feed-darwin-x64   = { url = "https://update.code.visualstudio.com/api/update/darwin/insider/latest"; flake = false; };
    vsci-feed-linux-arm64  = { url = "https://update.code.visualstudio.com/api/update/linux-arm64/insider/latest"; flake = false; };
    vsci-feed-linux-x64    = { url = "https://update.code.visualstudio.com/api/update/linux-x64/insider/latest"; flake = false; };
  };

  outputs = inputs@{ self, nixpkgs, nix-darwin, home-manager, ... }: {

    # ── macOS (nix-darwin + Home Manager) ──────────────────────────────────────
    darwinConfigurations.yuta = nix-darwin.lib.darwinSystem {
      system = "aarch64-darwin";
      modules = [
        ./hosts/darwin/default.nix
        home-manager.darwinModules.home-manager
        {
          home-manager.useGlobalPkgs = true;
          home-manager.useUserPackages = true;
          home-manager.backupFileExtension = "before-nix-darwin";
          home-manager.users.yuta = import ./hosts/darwin/home.nix;
          home-manager.extraSpecialArgs = { inherit inputs; };
        }
      ];
      specialArgs = { inherit inputs; };
    };

    # ── NixOS Server (GCP e2-micro) ───────────────────────────────────────────
    nixosConfigurations.server = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        inputs.disko.nixosModules.disko
        ./hosts/server/default.nix
      ];
      specialArgs = { inherit inputs; };
    };
  };
}
