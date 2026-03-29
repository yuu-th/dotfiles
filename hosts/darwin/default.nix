{ inputs, ... }: {
  # ── ホスト固有の基本定義 ──────────────────────────────────────────────────────
  nixpkgs.hostPlatform = "aarch64-darwin";
  system.primaryUser   = "yuta";
  system.stateVersion  = 6;

  imports = [
    inputs.home-manager.darwinModules.home-manager
    ../../profiles/darwin.nix
  ];

  home-manager = {
    useGlobalPkgs       = true;
    useUserPackages     = true;
    backupFileExtension = "before-nix-darwin";
    extraSpecialArgs    = { inherit inputs; };
  };

  services.tailscale.enable = true;
}
