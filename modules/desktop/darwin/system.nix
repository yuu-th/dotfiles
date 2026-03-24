{ ... }: {
  homebrew = {
    enable = true;
    onActivation = {
      autoUpdate = true;
      upgrade = true;
      cleanup = "zap";
    };
    taps = [
      "FelixKratz/formulae" # JankyBorders (フォーカスボーダー)
    ];
    brews = [
      "borders" # JankyBorders: フォーカス中ウィンドウに色付きボーダーを表示
    ];
    casks = [
      "raycast"
      "jordanbaird-ice"
      "alt-tab"
      "discord"
      "spotify"
      "linearmouse"
      "obsidian"
      "karabiner-elements"
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
