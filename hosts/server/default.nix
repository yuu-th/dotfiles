{ config, pkgs, lib, inputs, ... }: {
  imports = [
    ./hardware-configuration.nix
    ./disk-config.nix
    ../../modules/server/automation.nix
  ];

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
    openFirewall = false; # ポート22を全世界に開くのをやめる
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
