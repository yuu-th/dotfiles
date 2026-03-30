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
        cleanup = "zap";
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
