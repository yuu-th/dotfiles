# profiles/darwin.nix
{ config, lib, pkgs, ... }: {
  imports = [
    # darwin modules（homebrew.nix は各アプリが自動 import）
    ../modules/darwin/box.nix
    ../modules/darwin/orbstack.nix
    ../modules/darwin/borders.nix
    ../modules/darwin/raycast.nix
    ../modules/darwin/ice.nix
    ../modules/darwin/alt-tab.nix
    ../modules/darwin/discord.nix
    ../modules/darwin/spotify.nix
    ../modules/darwin/obsidian.nix
    ../modules/darwin/dia.nix
    ../modules/darwin/aerospace
    ../modules/darwin/omniwm
    ../modules/darwin/karabiner
    ../modules/darwin/linearmouse
    ../modules/darwin/pake-webapps
    ../modules/darwin/google-calendar.nix
    ../modules/darwin/ghostty.nix
    ../modules/darwin/cmux.nix
    ../modules/darwin/parsec.nix
    # sub-profiles（常時ON設定を関心ごとに分割）
    ./fav_fonts.nix
    # common modules
    ../modules/common/primary-user.nix
    ../modules/common/cli-tools.nix
    ../modules/common/antigravity.nix
    ../modules/common/zsh.nix
    ../modules/common/git.nix
    ../modules/common/go.nix
    ../modules/common/python.nix
    ../modules/common/claude-code.nix
    ../modules/common/gcloud.nix
    ../modules/common/vscode.nix
    ../modules/common/node.nix
    ../modules/common/rust.nix
    ../modules/common/terraform.nix
    ../modules/common/github-copilot-cli.nix
    ../modules/common/fish.nix
    ../modules/common/zellij.nix
    ../modules/common/neovim.nix
    ../modules/common/teams.nix
    ../modules/common/zen-browser.nix
    ../modules/common/gemini-cli.nix
    ../modules/common/uv.nix
    ../modules/common/browser-use.nix
    ../modules/common/vercel.nix
    ../modules/common/direnv.nix
    ../modules/common/flutter.nix
    ../modules/common/android-tools.nix
    ../modules/darwin/android-studio.nix
    ../modules/darwin/xcode-tools.nix
  ];

  # ── macOS 前提設定（常時ON、トグル不要）────────────────────────────────────
  nixpkgs.config.allowUnfree = true;
  nix.settings.experimental-features = [ "nix-command" "flakes" ];
  security.pam.services.sudo_local.touchIdAuth = true;

  system.defaults = {
    dock = {
      autohide = true;
      autohide-delay = 0.0;
      autohide-time-modifier = 0.15;
      mru-spaces = false;
      show-recents = false;
      orientation = "bottom";
      tilesize = 48;
    };
    NSGlobalDomain = {
      AppleSpacesSwitchOnActivate = false;
    };
  };

  system.activationScripts.systemTweaks.text = ''
    echo "Applying system tweaks..."
    /usr/bin/defaults write -g NSWindowShouldDragOnGesture -bool true
    /usr/bin/defaults write -g NSAutomaticWindowAnimationsEnabled -bool false
    /usr/bin/defaults write com.apple.spaces spans-displays -bool true
    /usr/bin/defaults write com.apple.dock expose-group-apps -bool true
    /usr/bin/defaults write -g NSWindowResizeTime -float 0.001
    /usr/bin/killall Dock 2>/dev/null || true
  '';

  system.activationScripts.postActivation.text = ''
    echo "Setting up Spotlight visibility for Home Manager apps..." >&2
    rm -rf "/Applications/Nix Apps"
    mkdir -p "/Applications/Nix Apps"
    for app in "/Users/${config.myConfig.primaryUser}/Applications/Home Manager Apps/"*.app; do
      if [ -e "$app" ]; then
        app_name=$(basename "$app")
        actual_path=$(readlink -f "$app")
        echo "Creating alias for $app_name..." >&2
        ${pkgs.mkalias}/bin/mkalias "$actual_path" "/Applications/Nix Apps/$app_name"
      fi
    done
  '';

  # ── Darwin modules ────────────────────────────────────────────────────────────
  myConfig.primaryUser = config.system.primaryUser;

  myConfig.darwin.homebrew.enable       = true;
  myConfig.darwin.box.enable            = true;
  myConfig.darwin.orbstack.enable       = true;
  myConfig.darwin.borders.enable        = config.myConfig.darwin.aerospace.enable;  # OmniWM 時は内蔵で代替
  myConfig.darwin.raycast.enable        = true;
  myConfig.darwin.ice.enable            = true;
  myConfig.darwin.altTab.enable         = true;
  myConfig.darwin.discord.enable        = true;
  myConfig.darwin.spotify.enable        = true;
  myConfig.darwin.obsidian.enable       = true;
  myConfig.darwin.dia.enable            = true;
  myConfig.darwin.aerospace.enable      = false;  # OmniWM 移行作業中（一時停止）
  myConfig.darwin.omniwm.enable         = true;
  myConfig.darwin.omniwm.monitorProfile = "auto";  # "auto" = 接続中モニタから自動選択。"<name>" で強制指定可
  myConfig.darwin.karabiner.enable      = true;
  myConfig.darwin.linearmouse.enable    = true;
  myConfig.darwin.pake.enable           = true;
  myConfig.darwin.googleCalendar.enable = true;
  myConfig.darwin.ghostty.enable        = true;
  myConfig.darwin.cmux.enable           = true;
  myConfig.darwin.parsec.enable         = true;

  # ── Common modules ────────────────────────────────────────────────────────────
  myConfig.cliTools.enable    = true;
  myConfig.antigravity.enable = true;
  myConfig.zsh.enable         = true;
  myConfig.git.enable         = true;
  myConfig.go.enable          = true;
  myConfig.python.enable      = true;
  myConfig.claudeCode.enable  = true;
  myConfig.gcloud.enable      = true;
  myConfig.vscode.enable      = true;
  myConfig.node.enable        = true;
  myConfig.rust.enable        = true;
  myConfig.terraform.enable   = true;
  myConfig.githubCopilotCli.enable = true;
  myConfig.fish.enable             = true;
  myConfig.zellij.enable           = true;
  myConfig.neovim.enable           = true;
  myConfig.teams.enable            = true;
  myConfig.zenBrowser.enable       = true;
  myConfig.geminiCli.enable        = true;
  myConfig.uv.enable               = true;
  myConfig.browserUse.enable        = true;
  myConfig.vercel.enable            = true;
  myConfig.direnv.enable            = true;
  myConfig.flutter.enable           = true;
  myConfig.androidTools.enable      = true;
  myConfig.darwin.androidStudio.enable = true;
  myConfig.darwin.xcodeTools.enable    = true;
  # ── Home Manager user facts ───────────────────────────────────────────────────
  home-manager.users.${config.myConfig.primaryUser} = {
    home.username      = config.myConfig.primaryUser;
    home.homeDirectory = lib.mkForce "/Users/${config.myConfig.primaryUser}";
    home.stateVersion  = "24.11";
  };
}
