{ ... }: {
  # ── LinearMouse の機能設定 ────────────────────────────────────
  xdg.configFile."linearmouse/linearmouse.json".source = ./linearmouse.json;

  # ── Karabiner-Elements の機能設定 ─────────────────────────────
  # Note: karabiner.json 全体をNixで管理することで、設定を自動的に有効化します。
  # GUIで設定を変更した場合、次回Nixのビルド時に上書きされる可能性があることに注意してください。
  xdg.configFile."karabiner/karabiner.json" = {
    text = builtins.readFile ./karabiner/karabiner.json;
    force = true;
  };
}
