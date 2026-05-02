# ── アプリルール（layout / 最小サイズのみ） ─────────────────────────────
# v2: assignToWorkspace は削除。OmniWM の rule 再適用で手動移動した window が
# 引き戻される問題を避けるため、起動時の整列は scripts/startup-sort.sh に移譲した。
# bundleId → WS マッピングは workspace-assignment.nix で一元管理。
#
# このファイルでは以下のみ扱う:
#   - layout = "float"          (現 WS に float で居続けるダイアログ系)
#   - minWidth / minHeight      (極小ウィンドウ防止)
{ ... }:
[
  # ── Editors: 極小防止 ─────────────────────────────────────────────────────
  { bundleId = "com.todesktop.230313mzl4w4u92"; minWidth = 800.0; minHeight = 500.0; }  # Cursor
  { bundleId = "com.jetbrains.intellij";        minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.jetbrains.pycharm";         minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.jetbrains.WebStorm";        minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.jetbrains.goland";          minWidth = 800.0; minHeight = 500.0; }

  # ── Chat: 極小防止 ─────────────────────────────────────────────────────────
  { bundleId = "com.tinyspeck.slackmacgap";     minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.microsoft.teams2";          minWidth = 800.0; minHeight = 500.0; }
  { bundleId = "com.microsoft.teams";           minWidth = 800.0; minHeight = 500.0; }

  # ── Notes: 極小防止 ────────────────────────────────────────────────────────
  { bundleId = "notion.id";                     minWidth = 700.0; minHeight = 500.0; }
  { bundleId = "md.obsidian";                   minWidth = 700.0; minHeight = 500.0; }

  # ── Music: 極小防止 ────────────────────────────────────────────────────────
  { bundleId = "com.apple.Music";               minWidth = 600.0; minHeight = 400.0; }

  # ── Floating（ダイアログ系・現 WS に留まる）────────────────────────────
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
  { bundleId = "com.1password.1password";      layout = "float"; }
  { bundleId = "com.agilebits.onepassword7";   layout = "float"; }
  { bundleId = "com.apple.MobileSMS";          layout = "float"; minWidth = 600.0; minHeight = 500.0; }
  { bundleId = "com.tinkoffsystems.utm";       layout = "float"; }

  # ── Ghostty ────────────────────────────────────────────────────────────
  { bundleId = "com.mitchellh.ghostty";        minWidth = 480.0; minHeight = 240.0; }
]
