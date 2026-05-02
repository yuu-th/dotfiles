# ── アプリルール（AeroSpace の on-window-detected を 1:1 移植） ────────────
# OmniWM の [[appRules]] にマップする。
# AeroSpace の `move-node-to-workspace X` → `assignToWorkspace = "X"`
# AeroSpace の `layout floating`         → `layout = "float"`
# id (UUID) は OmniWM が省略時に自動生成するため未指定。
#
# NOTE: OmniWM のワークスペース rawID は数値のみ受理されるため、AeroSpace の
# 名前付き WS (M, B, E) は内部的に rawID 10, 11, 12 にマップしている
# （displayName で M/B/E 表示は維持）。assignToWorkspace は rawID で指定する。
{ ... }:
[
  # ── Browsers → WS B (rawID 11) ───────────────────────────────────────────
  { bundleId = "com.google.Chrome";            assignToWorkspace = "11"; }
  { bundleId = "org.mozilla.firefox";          assignToWorkspace = "11"; }
  { bundleId = "com.apple.Safari";             assignToWorkspace = "11"; }
  { bundleId = "company.thebrowser.dia";       assignToWorkspace = "11"; }
  { bundleId = "app.zen-browser.zen";          assignToWorkspace = "11"; }

  # ── Editors → WS E (rawID 12) ────────────────────────────────────────────
  { bundleId = "com.microsoft.VSCodeInsiders"; assignToWorkspace = "12"; }
  { bundleId = "com.microsoft.VSCode";         assignToWorkspace = "12"; }
  { bundleId = "dev.zed.Zed";                  assignToWorkspace = "12"; }

  # ── AI / Agent → WS 1 ────────────────────────────────────────────────────
  { bundleId = "com.google.antigravity";       assignToWorkspace = "1"; }

  # ── Media / 常駐 → WS M (rawID 10) ───────────────────────────────────────
  { bundleId = "com.spotify.client";           assignToWorkspace = "10"; }
  { bundleId = "com.hnc.Discord";              assignToWorkspace = "10"; }
  { bundleId = "com.apple.iCal";               assignToWorkspace = "10"; }

  # ── Terminals → WS 3 ─────────────────────────────────────────────────────
  { bundleId = "com.googlecode.iterm2";        assignToWorkspace = "3"; }
  { bundleId = "com.apple.Terminal";           assignToWorkspace = "3"; }

  # ── Floating（ダイアログ系・現 WS に留まる）────────────────────────────
  # AeroSpace は app-name-regex で複数指定だったが OmniWM は substring/regex 別立て
  # → bundleId で個別指定（regex 不可なので確実な ID 指定に振る）
  { bundleId = "com.apple.finder";             layout = "float"; }
  { bundleId = "com.apple.systempreferences";  layout = "float"; }
  { bundleId = "com.apple.calculator";         layout = "float"; }
  { bundleId = "com.apple.Dictionary";         layout = "float"; }
  { bundleId = "com.mojang.minecraftlauncher"; layout = "float"; }

  # ── Ghostty ────────────────────────────────────────────────────────────
  # AeroSpace では on-window-detected で自動移動しないルールだったので OmniWM でも
  # assignToWorkspace を付けない。ただし最小サイズだけ与えて極小ウィンドウ防止。
  { bundleId = "com.mitchellh.ghostty"; minWidth = 480.0; minHeight = 240.0; }
]
