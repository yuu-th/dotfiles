{
  description = "My dev environment (macOS + NixOS) with Nix + nix-darwin + Home Manager";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    # wrangler 専用 pin。unstable の wrangler 4.90.0/4.93.0 は aarch64-darwin で
    # tsup ビルドが EBADF(Node fstat) で壊れているため(nixpkgs#423082 系)、
    # ビルド確認済みの 4.62.0 を持つ rev (4.90.0 bump 直前) に固定する。
    # 上流が修正されたら本 input ごと削除して nixpkgs 由来に戻すこと。
    nixpkgs-wrangler.url = "github:NixOS/nixpkgs/cfcca11389b00c161e78b87d867955defe076966";

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

    firefox-addons = {
      url = "gitlab:rycee/nur-expressions?dir=pkgs/firefox-addons";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    # Helium browser (imput 製、Chromium ベース、プライバシー重視)
    # - 公式に nixpkgs 未収録、amaanq の community flake で供給
    # - 対応: x86_64/aarch64 × linux/darwin
    # - 自動更新: 上流リリース後 15 分以内に GitHub Actions が hash 更新
    # - 設定 / policy は modules/common/helium.nix で管理
    helium-flake = {
      url = "github:amaanq/helium-flake";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    # GitHub Copilot CLI - updated 4x/day by numtide bot
    llm-agents.url = "github:numtide/llm-agents.nix";

    # vast.ai CLI - upstream の最新 master を darwin-up で追従
    # uv.lock は modules/common/vast-cli/uv.lock に commit、darwin-up が再生成
    vast-cli-src = {
      url = "github:vast-ai/vast-cli";
      flake = false;
    };
    pyproject-nix = {
      url = "github:pyproject-nix/pyproject.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    uv2nix = {
      url = "github:pyproject-nix/uv2nix";
      inputs.pyproject-nix.follows = "pyproject-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    pyproject-build-systems = {
      url = "github:pyproject-nix/build-system-pkgs";
      inputs.pyproject-nix.follows = "pyproject-nix";
      inputs.uv2nix.follows = "uv2nix";
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
