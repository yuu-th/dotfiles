# ── キーバインド（niri 流に最適化） ────────────────────────────────────────
# OmniWM の canonical デフォルトを尊重しつつ、ユーザの哲学（多数 WS、アプリ別
# ショートカット）は維持。AeroSpace 由来の擬似機能は削除済み。
#
# Karabiner レイヤ（karabiner-rules.nix）で実装するもの:
#   - alt-s / alt-c / alt-a       (シェル実行マクロ: WS M + アプリ起動)
#   - alt-ctrl-m                  (setup-media-workspace)
#   - alt-m / alt-b / alt-e       (名前指定 WS 切替: omniwmctl workspace focus-name)
#   - alt-shift-m / b / e         (名前指定 WS への送り)
#   - alt-ctrl-h/j/k/l            (方向ベース focus-monitor)
#   - cmd-h                       (macOS Hide ブロック)
{ ... }:
[
  # ── 数字 WS 切替 (alt-1〜9) ─────────────────────────────────────────────
  { binding = "Option+1"; id = "switchWorkspace.0"; }
  { binding = "Option+2"; id = "switchWorkspace.1"; }
  { binding = "Option+3"; id = "switchWorkspace.2"; }
  { binding = "Option+4"; id = "switchWorkspace.3"; }
  { binding = "Option+5"; id = "switchWorkspace.4"; }
  { binding = "Option+6"; id = "switchWorkspace.5"; }
  { binding = "Option+7"; id = "switchWorkspace.6"; }
  { binding = "Option+8"; id = "switchWorkspace.7"; }
  { binding = "Option+9"; id = "switchWorkspace.8"; }

  # ── 数字 WS への送り (alt-shift-1〜9) ────────────────────────────────────
  { binding = "Option+Shift+1"; id = "moveToWorkspace.0"; }
  { binding = "Option+Shift+2"; id = "moveToWorkspace.1"; }
  { binding = "Option+Shift+3"; id = "moveToWorkspace.2"; }
  { binding = "Option+Shift+4"; id = "moveToWorkspace.3"; }
  { binding = "Option+Shift+5"; id = "moveToWorkspace.4"; }
  { binding = "Option+Shift+6"; id = "moveToWorkspace.5"; }
  { binding = "Option+Shift+7"; id = "moveToWorkspace.6"; }
  { binding = "Option+Shift+8"; id = "moveToWorkspace.7"; }
  { binding = "Option+Shift+9"; id = "moveToWorkspace.8"; }

  # ── フォーカス h/j/k/l (alt-h/j/k/l) ──────────────────────────────────
  { binding = "Option+H"; id = "focus.left"; }
  { binding = "Option+J"; id = "focus.down"; }
  { binding = "Option+K"; id = "focus.up"; }
  { binding = "Option+L"; id = "focus.right"; }
  # 直前のフォーカスウィンドウに戻る（Option+Tab の WS 版とは別、ウィンドウ単位）
  { binding = "Option+P"; id = "focusPrevious"; }

  # ── ウィンドウ移動 (alt-shift-h/j/k/l) ───────────────────────────────────
  # 左右 move は隣 column への consume 効果も持つ（旧 join-with 相当）
  { binding = "Option+Shift+H"; id = "move.left"; }
  { binding = "Option+Shift+J"; id = "move.down"; }
  { binding = "Option+Shift+K"; id = "move.up"; }
  { binding = "Option+Shift+L"; id = "move.right"; }

  # ── Column 直接フォーカス（niri 流の超強力ナビ）────────────────────────
  # 現在の WS 内で N 番目の column に一発ジャンプ
  { binding = "Control+Option+1"; id = "focusColumn.0"; }
  { binding = "Control+Option+2"; id = "focusColumn.1"; }
  { binding = "Control+Option+3"; id = "focusColumn.2"; }
  { binding = "Control+Option+4"; id = "focusColumn.3"; }
  { binding = "Control+Option+5"; id = "focusColumn.4"; }
  { binding = "Control+Option+6"; id = "focusColumn.5"; }
  { binding = "Control+Option+7"; id = "focusColumn.6"; }
  { binding = "Control+Option+8"; id = "focusColumn.7"; }
  { binding = "Control+Option+9"; id = "focusColumn.8"; }
  { binding = "Option+Home";      id = "focusColumnFirst"; }
  { binding = "Option+End";       id = "focusColumnLast"; }

  # ── Column 単位の移動（ウィンドウじゃなく column ごと動かす）──────────
  { binding = "Control+Option+Shift+H"; id = "moveColumn.left"; }
  { binding = "Control+Option+Shift+L"; id = "moveColumn.right"; }

  # ── レイアウト切替・サイズ調整 ────────────────────────────────────────
  # canonical 準拠: , / . で column width 巡回、T で tabbed 切替
  { binding = "Option+,";       id = "cycleColumnWidthBackward"; }
  { binding = "Option+.";       id = "cycleColumnWidthForward"; }
  { binding = "Option+T";       id = "toggleColumnTabbed"; }
  { binding = "Option+Shift+F"; id = "toggleColumnFullWidth"; }
  { binding = "Option+Shift+B"; id = "balanceSizes"; }
  { binding = "Option+/";       id = "toggleWorkspaceLayout"; }   # niri ⇄ dwindle

  # ── フルスクリーン・floating ─────────────────────────────────────────
  { binding = "Option+Return";       id = "toggleFullscreen"; }
  { binding = "Option+Shift+Space";  id = "toggleFocusedWindowFloating"; }

  # ── WS 行き来 ────────────────────────────────────────────────────────
  { binding = "Option+Tab"; id = "workspaceBackAndForth"; }

  # ── UI / discoverability ────────────────────────────────────────────
  { binding = "Option+Shift+O";       id = "toggleOverview"; }            # 全 WS 俯瞰
  { binding = "Option+Shift+R";       id = "raiseAllFloatingWindows"; }
  { binding = "Control+Option+Space"; id = "openCommandPalette"; }        # 全コマンド検索
  { binding = "Control+Option+M";     id = "openMenuAnywhere"; }          # context menu 召喚

  # ── Quake terminal（OmniWM 内蔵 libghostty） ────────────────────────
  { binding = "Option+`"; id = "toggleQuakeTerminal"; }
]
