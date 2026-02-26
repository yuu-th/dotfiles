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
    ];
  };
}
