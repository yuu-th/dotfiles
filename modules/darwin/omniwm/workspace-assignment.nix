# ── bundleId → ワークスペース rawID マッピング ─────────────────────────────
# OmniWM 起動時の one-shot sort スクリプト (scripts/startup-sort.sh) が
# このマップに従って既存ウィンドウを正しい WS に整列させる。
#
# 設計思想:
# - appRules.assignToWorkspace は使わない（再検知で window が引き戻される問題を回避）
# - 起動時 1 回だけ整列。それ以降の新ウィンドウは開いた WS に留まる
# - 手動で WS を跨いで動かしても **絶対に戻されない**
#
# rawID:
#   1〜9 = 数値 WS / 10 = M (Media) / 11 = B (Browser)
#   12 = E (旧 Editor、projwm では AI slot 3 として再利用)
#   13 = A (projwm AI Viewer) / 14-22 = projwm AI slots Q/W/R/T/Y/U/I/O/P
{
  # ── Browsers → WS B (11) ───────────────────────────────────────────────
  "com.google.Chrome"      = "11";
  "org.mozilla.firefox"    = "11";
  "com.apple.Safari"       = "11";
  "company.thebrowser.dia" = "11";
  "app.zen-browser.zen"    = "11";

  # ── Editors → 起動時 WS 固定は廃止（projwm 導入に伴う、queue/projwm-design.md §4.3 / FR-24） ─
  # WS E は projwm の AI slot 3 として再利用されるため、editor を一律 WS 12 (E) に
  # 集めると AI ワークスペースに editor が混ざってしまう。
  # 代替運用:
  #   - Zed: projwm が per-project に slot 配置（projwm reconcile で動的）
  #   - VSCode/Cursor/JetBrains: ad-hoc 起動なので開いた WS に留める。
  #     必要なら手動で `alt+shift+<letter>` で送る（NR-01「動的 appRule で固定しない」）
  # 削除前: "com.microsoft.VSCodeInsiders" = "12"; "com.microsoft.VSCode" = "12";
  #         "com.todesktop.230313mzl4w4u92" = "12"; (Cursor)
  #         "com.jetbrains.intellij" = "12"; .pycharm / .WebStorm / .goland 等
  # （いずれも v11.3 で削除）

  # ── AI / Agent → WS 1 ──────────────────────────────────────────────────
  "com.google.antigravity" = "1";

  # ── Media / 常駐 → WS M (10) ───────────────────────────────────────────
  "com.spotify.client" = "10";
  "com.hnc.Discord"    = "10";
  "com.apple.iCal"     = "10";
  "com.apple.Music"    = "10";

  # ── Terminals → WS 3 ───────────────────────────────────────────────────
  "com.googlecode.iterm2" = "3";
  "com.apple.Terminal"    = "3";

  # ── Chat → WS 4 ────────────────────────────────────────────────────────
  "com.tinyspeck.slackmacgap" = "4";
  "com.microsoft.teams2"      = "4";
  "com.microsoft.teams"       = "4";

  # ── Notes → WS 5 ───────────────────────────────────────────────────────
  "notion.id"   = "5";
  "md.obsidian" = "5";
}
