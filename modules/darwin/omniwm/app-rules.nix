# ── アプリルール（layout / 最小サイズのみ） ─────────────────────────────
# v3 (OmniWM 0.4.9 対応):
# - OmniWM 0.4.9 から各 [[appRules]] エントリに `id` フィールドが必須となった。
#   無いと settings.toml が `.corrupt` rename されて全 nix-config が捨てられる。
# - id は bundleId + index から md5 で決定的に生成。再ビルドで安定。
#
# このファイルでは以下のみ扱う:
#   - layout = "float"          (現 WS に float で居続けるダイアログ系)
#   - minWidth / minHeight      (極小ウィンドウ防止)
#   - titleRegex                (title で対象を絞り込む場合)
{ lib ? (import <nixpkgs> { }).lib, ... }:
let
  # ── id 生成 ──────────────────────────────────────────────────────────────
  # md5(bundleId + "#" + index) → 32 hex → UUID 形式 (8-4-4-4-12) に整形。
  # OmniWM 0.4.9 は `id` の形式チェックを UUID 風に行うので 8-4-4-4-12 が安全。
  mkId = bundleId: index:
    let
      h = builtins.hashString "md5" "${bundleId}#${toString index}";
      part = start: len: builtins.substring start len h;
    in
      lib.toUpper "${part 0 8}-${part 8 4}-${part 12 4}-${part 16 4}-${part 20 12}";

  # ── 元のルール定義（layout / titleRegex / minWidth / minHeight） ─────────
  rules = [
    # ── Editors: 極小防止 ─────────────────────────────────────────────────
    { bundleId = "com.todesktop.230313mzl4w4u92"; minWidth = 800.0; minHeight = 500.0; }  # Cursor
    { bundleId = "com.jetbrains.intellij";        minWidth = 800.0; minHeight = 500.0; }
    { bundleId = "com.jetbrains.pycharm";         minWidth = 800.0; minHeight = 500.0; }
    { bundleId = "com.jetbrains.WebStorm";        minWidth = 800.0; minHeight = 500.0; }
    { bundleId = "com.jetbrains.goland";          minWidth = 800.0; minHeight = 500.0; }

    # ── Chat: 極小防止 ────────────────────────────────────────────────────
    { bundleId = "com.tinyspeck.slackmacgap";     minWidth = 800.0; minHeight = 500.0; }
    { bundleId = "com.microsoft.teams2";          minWidth = 800.0; minHeight = 500.0; }
    { bundleId = "com.microsoft.teams";           minWidth = 800.0; minHeight = 500.0; }

    # ── Notes: 極小防止 ───────────────────────────────────────────────────
    { bundleId = "notion.id";                     minWidth = 700.0; minHeight = 500.0; }
    { bundleId = "md.obsidian";                   minWidth = 700.0; minHeight = 500.0; }

    # ── Music: 極小防止 ───────────────────────────────────────────────────
    { bundleId = "com.apple.Music";               minWidth = 600.0; minHeight = 400.0; }

    # ── Floating（ダイアログ系・現 WS に留まる）─────────────────────────
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

    # Ghostty 一般 (最小サイズだけ確保)
    { bundleId  = "com.mitchellh.ghostty";
      minWidth  = 480.0;
      minHeight = 240.0; }
  ];
in
  # 各 rule に id を付与（0.4.9 必須）。index は重複 bundleId を区別するため。
  lib.imap0 (i: rule: rule // { id = mkId rule.bundleId i; }) rules
