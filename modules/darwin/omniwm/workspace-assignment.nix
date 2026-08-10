# ── bundleId → ワークスペース rawName マッピング ────────────────────────────
# OmniWM 起動時の one-shot sort スクリプト (scripts/startup-sort.sh) が
# このマップに従って既存ウィンドウを正しい WS に整列させる。
#
# 設計思想:
# - appRules.assignToWorkspace は使わない
#   （0.5.9 では「最初にマッチした窓だけ」に意味が変わったが、それは
#    「既に開いている窓をまとめて整列する」用途には使えない。既存窓に効かせるには
#    `omniwmctl rule apply` を明示的に叩く必要があり、それなら整列スクリプトの方が素直）
# - 起動時 1 回だけ整列。それ以降の新ウィンドウは開いた WS に留まる
# - 手動で WS を跨いで動かしても **絶対に戻されない**
#
# ── rawName 対応表（workspace-builder.nix と一致させること）──
#   1〜9  = 数値 WS（ad-hoc）
#   10 = W / 11 = E / 12 = R   … メイン作業（上段・上のディスプレイ）
#   13 = S / 14 = D / 15 = F   … そのプロジェクトのブラウザ（下段）
#   16 = X（Media）/ 17 = C（Chat）/ 18 = V（予定・ノート） … 常駐（最下段）
{
  # ── ブラウザ → WS S (13) ───────────────────────────────────────────────
  # Helium が既定ブラウザ。他のブラウザを使っても同じ場所に着地するように
  # まとめて S に寄せる（ブラウザを乗り換えても運用が変わらない）。
  "net.imput.helium"       = "13";
  "com.google.Chrome"      = "13";
  "org.mozilla.firefox"    = "13";
  "com.apple.Safari"       = "13";
  "company.thebrowser.dia" = "13";
  "app.zen-browser.zen"    = "13";

  # ── ターミナル / エージェント → WS W (10) ──────────────────────────────
  # メイン作業面の先頭。cmux が主力。
  "com.cmuxterm.app"      = "10";
  "com.googlecode.iterm2" = "10";
  "com.apple.Terminal"    = "10";

  # ── AI クライアント → WS E (11) ────────────────────────────────────────
  "com.anthropic.claudefordesktop" = "11";
  "com.google.antigravity"         = "11";

  # ── Media → WS X (16) ──────────────────────────────────────────────────
  "com.spotify.client" = "16";
  "com.apple.Music"    = "16";

  # ── Chat → WS C (17) ───────────────────────────────────────────────────
  "com.hnc.Discord"           = "17";
  "com.tinyspeck.slackmacgap" = "17";
  "com.microsoft.teams2"      = "17";
  "com.microsoft.teams"       = "17";
  "com.apple.MobileSMS"       = "17";

  # ── 予定 / ノート → WS V (18) ──────────────────────────────────────────
  "com.apple.iCal" = "18";
  "notion.id"      = "18";
  "md.obsidian"    = "18";

  # ── 意図的に登録しないもの ─────────────────────────────────────────────
  # エディタ（VSCode / Cursor / JetBrains / Zed）は「その日どのプロジェクトの
  # ペアに置くか」が変わるので固定しない。開いた WS に留め、必要なら
  # space+shift+<letter> で送る。
}
