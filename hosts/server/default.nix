{ config, pkgs, lib, inputs, ... }: {
  # ── ホスト固有の基本定義 ──────────────────────────────────────────────────────
  nixpkgs.hostPlatform = "x86_64-linux";

  imports = [
    inputs.disko.nixosModules.disko
    ./hardware-configuration.nix
    ./disk-config.nix
    ../../modules/nixos/flake-autoupdate.nix
  ];

  myModules.nixos.flakeAutoupdate.enable = true;

  # ── Networking & Tailscale ────────────────────────────────────────────────
  networking = { hostName = "dotfiles-bot"; firewall.allowedTCPPorts = [ ]; };
  services.tailscale.enable = true;

  # ── Resource Optimization (e2-micro) ────────────────────────────────────────
  swapDevices = [{ device = "/var/lib/swapfile"; size = 2048; }];
  nix.settings = {
    max-jobs = 1;
    cores = 1;
    auto-optimise-store = true;
    experimental-features = [ "nix-command" "flakes" ];
  };
  nix.gc = { automatic = true; dates = "weekly"; options = "--delete-older-than 7d"; };

  # ── Base Packages ──────────────────────────────────────────────────────────
  environment.systemPackages = with pkgs; [ git gh curl jq bash ];

  # ── SSH ─────────────────────────────────────────────────────────────────────
  services.openssh = {
    enable = true;
    openFirewall = false;
    settings.PermitRootLogin = "prohibit-password";
    settings.PasswordAuthentication = false;
    authorizedKeysFiles = lib.mkForce [ "/var/lib/authorized_keys" ];
  };
  networking.firewall.trustedInterfaces = [ "tailscale0" ];

  users.users.nixos = {
    isNormalUser = true;
    extraGroups = [ "wheel" ];
  };

  system.stateVersion = lib.mkDefault "24.11";
}
