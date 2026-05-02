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
#   1〜9 = 数値 WS / 10 = M (Media) / 11 = B (Browser) / 12 = E (Editor)
{
  # ── Browsers → WS B (11) ───────────────────────────────────────────────
  "com.google.Chrome"      = "11";
  "org.mozilla.firefox"    = "11";
  "com.apple.Safari"       = "11";
  "company.thebrowser.dia" = "11";
  "app.zen-browser.zen"    = "11";

  # ── Editors → WS E (12) ────────────────────────────────────────────────
  # ⚠️ projwm 導入後、Zed は projwm が per-project に slot 配置するため
  # startup-sort の Zed → 12 ルールは削除する（queue/projwm-design.md FR-24 / 決定 42）。
  # OmniWM 起動時に Zed が走っていれば projwm reconcile が改めて正しい slot に
  # 戻すので、ここで一律 12 に集めない方が望ましい。
  "com.microsoft.VSCodeInsiders"  = "12";
  "com.microsoft.VSCode"          = "12";
  "com.todesktop.230313mzl4w4u92" = "12";  # Cursor
  "com.jetbrains.intellij"        = "12";
  "com.jetbrains.pycharm"         = "12";
  "com.jetbrains.WebStorm"        = "12";
  "com.jetbrains.goland"          = "12";

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
