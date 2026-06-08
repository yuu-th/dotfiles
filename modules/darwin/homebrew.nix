# modules/darwin/homebrew.nix
# Homebrew base + preActivation + caskRepair
{ config, lib, ... }:
let cfg = config.myConfig.darwin.homebrew; in {
  options.myConfig.darwin.homebrew.enable = lib.mkEnableOption "Homebrew base setup";

  config = lib.mkIf cfg.enable {
    environment.systemPath = [ "/opt/homebrew/bin" ];

    homebrew = {
      enable = true;
      onActivation = {
        autoUpdate = true;
        upgrade = true;
        # cleanup "zap" を一時的に "none" に。Homebrew PR #22395 (2026-05-24) で
        # `brew bundle cleanup` が破壊的化(Brewfile に無い MAS アプリまで無差別削除)
        # + `--force`/`$HOMEBREW_ASK` ゲート化され、nix-darwin が非対話で `--cleanup`
        # を呼ぶと activation が失敗する (Homebrew#22450 / nix-darwin#1787)。
        # nix-darwin の修正 (PR #1774, `cleanup --force --zap` 化) が merge されたら
        # "zap" に戻す。現状 drift=0(全インストールが宣言済)+ mas 未導入なので
        # "none" による実損失は無し(dry-run で uninstall 対象 0 件を確認済)。
        cleanup = "none";
      };
      taps = [ "FelixKratz/formulae" ];
      brews = [ "pake" ];
    };

    system.activationScripts.preActivation.text = ''
      echo "Checking for conflicting system files..."
      for file in /etc/bashrc /etc/zshrc; do
        if [ -f "$file" ] && [ ! -L "$file" ]; then
          echo "Auto-backing up $file to $file.before-nix-darwin"
          mv "$file" "$file.before-nix-darwin"
        fi
      done

      if ! /opt/homebrew/bin/brew --version > /dev/null 2>&1; then
        echo "Homebrew not found. Installing..." >&2
        sudo -u ${config.myConfig.primaryUser} /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)" < /dev/null
      fi
    '';

    system.activationScripts.caskRepair.text = ''
      BREW="/opt/homebrew/bin/brew"
      if [ -x "$BREW" ] && [ -d /opt/homebrew/Caskroom ]; then
        for cask_dir in /opt/homebrew/Caskroom/*/; do
          cask_name=$(basename "$cask_dir")
          app_path=$(find "$cask_dir" -maxdepth 2 -name "*.app" -type d -print -quit 2>/dev/null)
          if [ -n "$app_path" ]; then
            app_name=$(basename "$app_path")
            if [ ! -e "/Applications/$app_name" ]; then
              echo "Broken Cask detected: $cask_name ($app_name not in /Applications). Reinstalling..." >&2
              sudo -u ${config.myConfig.primaryUser} "$BREW" reinstall --cask "$cask_name"
            fi
          fi
        done
      fi
    '';
  };
}
