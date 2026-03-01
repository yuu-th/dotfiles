{ ... }: {
  homebrew = {
    enable = true;
    onActivation = {
      autoUpdate = true;
      upgrade = true;
      cleanup = "zap";
    };
    taps = [
      "nikitabobko/tap" # Aerospaceのtapを追加
    ];
    casks = [
      "raycast"
      "jordanbaird-ice"
      "alt-tab"
      "nikitabobko/tap/aerospace"
      "discord"
      "spotify"
      "linearmouse"
    ];
  };

  # ── Launchd エージェント (GUIアプリの自動起動) ───────────────────
  launchd.user.agents.linearmouse = {
    serviceConfig = {
      ProgramArguments = [ "/Applications/LinearMouse.app/Contents/MacOS/LinearMouse" ];
      RunAtLoad = true;
      KeepAlive = true;
    };
  };
}
