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
  { bundleId = "com.todesktop.230313mzl4w4u92"; assignToWorkspace = "12"; minWidth = 800.0; minHeight = 500.0; } # Cursor
  { bundleId = "com.jetbrains.intellij";       assignToWorkspace = "12"; minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.jetbrains.pycharm";        assignToWorkspace = "12"; minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.jetbrains.WebStorm";       assignToWorkspace = "12"; minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.jetbrains.goland";         assignToWorkspace = "12"; minWidth = 800.0; minHeight = 500.0; }

  # ── AI / Agent → WS 1 ────────────────────────────────────────────────────
  { bundleId = "com.google.antigravity";       assignToWorkspace = "1"; }

  # ── Media / 常駐 → WS M (rawID 10) ───────────────────────────────────────
  { bundleId = "com.spotify.client";           assignToWorkspace = "10"; }
  { bundleId = "com.hnc.Discord";              assignToWorkspace = "10"; }
  { bundleId = "com.apple.iCal";               assignToWorkspace = "10"; }

  # ── Terminals → WS 3 ─────────────────────────────────────────────────────
  { bundleId = "com.googlecode.iterm2";        assignToWorkspace = "3"; }
  { bundleId = "com.apple.Terminal";           assignToWorkspace = "3"; }

  # ── Chat / Communication → WS 4 ──────────────────────────────────────────
  { bundleId = "com.tinyspeck.slackmacgap";    assignToWorkspace = "4"; minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.microsoft.teams2";         assignToWorkspace = "4"; minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.microsoft.teams";          assignToWorkspace = "4"; minWidth = 800.0; minHeight = 500.0; }

  # ── Notes / Reference → WS 5 ─────────────────────────────────────────────
  { bundleId = "notion.id";                    assignToWorkspace = "5"; minWidth = 700.0; minHeight = 500.0; }
  { bundleId = "md.obsidian";                  assignToWorkspace = "5"; minWidth = 700.0; minHeight = 500.0; }

  # ── Music （Spotify と並べる）──────────────────────────────────────────
  { bundleId = "com.apple.Music";              assignToWorkspace = "10"; minWidth = 600.0; minHeight = 400.0; }

  # ── Floating（ダイアログ系・現 WS に留まる）────────────────────────────
  # OmniWM は appNameSubstring/titleRegex 個別フィールドで regex を扱う。
  # AeroSpace の `app-name-regex` 一括指定は使えないので bundleId で個別に指定。
  { bundleId = "com.apple.finder";             layout = "float"; }
  { bundleId = "com.apple.systempreferences";  layout = "float"; }
  { bundleId = "com.apple.calculator";         layout = "float"; }
  { bundleId = "com.apple.Dictionary";         layout = "float"; }
  { bundleId = "com.apple.ActivityMonitor";    layout = "float"; }
  { bundleId = "com.apple.Console";            layout = "float"; }
  { bundleId = "com.apple.QuickTimePlayerX";   layout = "float"; }
  { bundleId = "com.apple.PhotoBooth";         layout = "float"; }
  { bundleId = "com.apple.iWork.Keynote";      layout = "float"; minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.apple.iWork.Pages";        layout = "float"; minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.apple.iWork.Numbers";      layout = "float"; minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.mojang.minecraftlauncher"; layout = "float"; }
  { bundleId = "com.raycast.macos";            layout = "float"; }
  { bundleId = "com.knollsoft.Hookshot";       layout = "float"; }
  # 1Password など modal 性質のアプリ
  { bundleId = "com.1password.1password";      layout = "float"; }
  { bundleId = "com.agilebits.onepassword7";   layout = "float"; }
  # 設定系・ユーティリティ
  { bundleId = "com.apple.MobileSMS";          layout = "float"; minWidth = 600.0; minHeight = 500.0; }
  { bundleId = "com.tinkoffsystems.utm";       layout = "float"; }

  # ── Ghostty ────────────────────────────────────────────────────────────
  # AeroSpace では on-window-detected で自動移動しないルールだったので OmniWM でも
  # assignToWorkspace を付けない。ただし最小サイズだけ与えて極小ウィンドウ防止。
  { bundleId = "com.mitchellh.ghostty"; minWidth = 480.0; minHeight = 240.0; }
]
