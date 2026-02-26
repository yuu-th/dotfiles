{ pkgs, inputs, ... }: {
  # ── Nix 設定 ──────────────────────────────────────────────────────────────────
  nix.settings.experimental-features = [ "nix-command" "flakes" ];

  # nixpkgs の設定
  nixpkgs.config.allowUnfree = true;

  # ── macOS システム設定 ─────────────────────────────────────────────────────────
  # 将来的にここに Dock、Finder、キーバインド等の設定を追加できる
  # 例:
  # system.defaults.dock.autohide = true;
  # system.defaults.finder.AppleShowAllExtensions = true;

  # ── ネットワーク & VPN ──────────────────────────────────────────────────────────
  services.tailscale.enable = true;

  # ── セキュリティ ──────────────────────────────────────────────────────────────
  security.pam.services.sudo_local.touchIdAuth = true;

  # ── 既存ファイルの自動バックアップ ───────────────────────────────────────────
  # nix-darwin が /etc/bashrc 等を作成する前に、既存のファイルがあれば自動で退避させる
  system.activationScripts.preActivation.text = ''
    echo "Checking for conflicting system files..."
    for file in /etc/bashrc /etc/zshrc; do
      if [ -f "$file" ] && [ ! -L "$file" ]; then
        echo "Auto-backing up $file to $file.before-nix-darwin"
        mv "$file" "$file.before-nix-darwin"
      fi
    done
  '';

  # ── Spotlight 検索への対応 ──────────────────────────────────────────────────
  # Home Manager で入れたアプリ（~/Applications）を Spotlight で検索可能にするため、
  # /Applications/Nix Apps に「エイリアス（Alias）」を作成する。
  # シンボリックリンクではなくエイリアスを使うのが、Spotlight に認識させるためのベストプラクティス。
  system.activationScripts.postActivation.text = ''
    echo "Setting up Spotlight visibility for Home Manager apps..." >&2
    rm -rf "/Applications/Nix Apps"
    mkdir -p "/Applications/Nix Apps"
    for app in "/Users/yuta/Applications/Home Manager Apps/"*.app; do
      if [ -e "$app" ]; then
        app_name=$(basename "$app")
        # If it was a symlink, $app_name is correct, but we want the actual store path for mkalias
        actual_path=$(readlink -f "$app")
        echo "Creating alias for $app_name..." >&2
        ${pkgs.mkalias}/bin/mkalias "$actual_path" "/Applications/Nix Apps/$app_name"
      fi
    done
  '';

  # Used for backwards compatibility
  system.stateVersion = 6;
}
