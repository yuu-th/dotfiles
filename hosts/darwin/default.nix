{ pkgs, inputs, ... }: {
  system.primaryUser = "yuta";
  imports = [
    ../../modules/desktop/darwin.nix
  ];
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

    # ── Homebrew の自動インストール（新規環境対応）──────────────────────
    if ! /opt/homebrew/bin/brew --version > /dev/null 2>&1; then
      echo "Homebrew not found. Installing..." >&2
      sudo -u yuta /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)" < /dev/null
    fi
  '';

  # ── Spotlight 検索への対応 ──────────────────────────────────────────────────
  # Home Manager で入れたアプリ（~/Applications）を Spotlight で検索可能にするため、
  # /Applications/Nix Apps に「エイリアス（Alias）」を作成する。
  # シンボリックリンクではなくエイリアスを使うのが、Spotlight に認識させるためのベストプラクティス。
  system.activationScripts.postActivation.text = ''
    # ── Home Manager アプリを Spotlight で検索可能にする（エイリアス作成）──
    echo "Setting up Spotlight visibility for Home Manager apps..." >&2
    rm -rf "/Applications/Nix Apps"
    mkdir -p "/Applications/Nix Apps"
    for app in "/Users/yuta/Applications/Home Manager Apps/"*.app; do
      if [ -e "$app" ]; then
        app_name=$(basename "$app")
        actual_path=$(readlink -f "$app")
        echo "Creating alias for $app_name..." >&2
        ${pkgs.mkalias}/bin/mkalias "$actual_path" "/Applications/Nix Apps/$app_name"
      fi
    done

    # ── Homebrew Cask の自己修復 ──────────────────────────────────────
    # Caskroom にあるのに /Applications に配置されていないアプリを検知し、
    # brew reinstall --cask で自動修復する。
    BREW="/opt/homebrew/bin/brew"
    if [ -x "$BREW" ] && [ -d /opt/homebrew/Caskroom ]; then
      for cask_dir in /opt/homebrew/Caskroom/*/; do
        cask_name=$(basename "$cask_dir")
        app_path=$(find "$cask_dir" -maxdepth 2 -name "*.app" -type d -print -quit 2>/dev/null)
        if [ -n "$app_path" ]; then
          app_name=$(basename "$app_path")
          if [ ! -e "/Applications/$app_name" ]; then
            echo "Broken Cask detected: $cask_name ($app_name not in /Applications). Reinstalling..." >&2
            sudo -u yuta "$BREW" reinstall --cask "$cask_name"
          fi
        fi
      done
    fi
  '';

  # Used for backwards compatibility
  system.stateVersion = 6;
}
