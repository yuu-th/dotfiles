# ── キーバインド（AeroSpace の main.binding を OmniWM action ID にマップ） ──
# OmniWM ホットキーは「事前定義 action ID にバインド文字列を割当てる」モデル。
# OmniWM がネイティブ対応する操作はここで TOML に直接書く。
# OmniWM が action として持っていない/シェル実行/モーダル/名前指定 WS 切替等は
# Karabiner レイヤ（karabiner-rules.nix）で実装する。
#
# AeroSpace では割当ていたが OmniWM で「action 不在 / 命名なし」のため
# Karabiner 側に逃がしているもの:
#   - alt-s / alt-c / alt-a / alt-ctrl-m  (シェル実行マクロ)
#   - alt-m / alt-b / alt-e               (名前指定 WS 切替 → workspace focus-name)
#   - alt-shift-m / b / e                 (名前指定 WS への移動)
#   - alt-ctrl-h/j/k/l                    (方向ベース focus-monitor)
#   - alt-r                               (resize モード入口、set_variable)
#   - alt-shift-semicolon                 (service モード — 廃止)
#   - cmd-h / cmd-alt-h                   (macOS Hide ブロック)
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

  # ── フォーカス移動 h/j/k/l (alt-h/j/k/l) ────────────────────────────────
  { binding = "Option+H"; id = "focus.left"; }
  { binding = "Option+J"; id = "focus.down"; }
  { binding = "Option+K"; id = "focus.up"; }
  { binding = "Option+L"; id = "focus.right"; }

  # ── ウィンドウ移動 (alt-shift-h/j/k/l) ───────────────────────────────────
  # OmniWM の move.left/right は隣 column への consume 効果も持つため
  # AeroSpace の "join-with" に相当する操作（alt-ctrl-shift-h/j/k/l）と統合される。
  { binding = "Option+Shift+H"; id = "move.left"; }
  { binding = "Option+Shift+J"; id = "move.down"; }
  { binding = "Option+Shift+K"; id = "move.up"; }
  { binding = "Option+Shift+L"; id = "move.right"; }

  # ── レイアウト切替 ───────────────────────────────────────────────────────
  # AeroSpace alt-slash    = "layout tiles ..."     → toggleWorkspaceLayout (niri⇄dwindle)
  # AeroSpace alt-comma    = "layout accordion ..." → toggleColumnTabbed (column 内タブ表示)
  # NOTE: OmniWM の KeySymbolMapper は記号キーを "/", ",", "-", "=", "[", "]" 等
  # の生記号で認識する。"Slash"/"Hyphen"/"Equal" のような名前は受け付けず
  # KeyBinding decode が失敗 → 設定ファイル全体が corrupt 扱いになる。
  { binding = "Option+/"; id = "toggleWorkspaceLayout"; }
  { binding = "Option+,"; id = "toggleColumnTabbed"; }

  # ── フルスクリーン (alt-enter) ───────────────────────────────────────────
  { binding = "Option+Return"; id = "toggleFullscreen"; }

  # ── floating⇔tiling (alt-shift-space) ─────────────────────────────────
  { binding = "Option+Shift+Space"; id = "toggleFocusedWindowFloating"; }

  # ── リサイズ簡易 (alt-minus / alt-equal) ─────────────────────────────────
  # AeroSpace の "resize smart ±50" は OmniWM の column width preset 巡回で代替
  # 記号は生キー記号で指定（KeySymbolMapper の制約）
  { binding = "Option+-"; id = "cycleColumnWidthBackward"; }
  { binding = "Option+="; id = "cycleColumnWidthForward"; }

  # ── WS 行き来 (alt-tab) ──────────────────────────────────────────────────
  { binding = "Option+Tab"; id = "workspaceBackAndForth"; }

  # ── サービスモード相当の単発ショートカット ───────────────────────────────
  # AeroSpace では mode service 内で `alt-shift-d = enable toggle` 等を使っていた
  # OmniWM でモード概念がないため、よく使うものを直接バインドにフォールバック
  { binding = "Option+Shift+R"; id = "raiseAllFloatingWindows"; }
  { binding = "Option+Shift+B"; id = "balanceSizes"; }
  { binding = "Option+Shift+F"; id = "toggleColumnFullWidth"; }
  { binding = "Option+Shift+O"; id = "toggleOverview"; }

  # ── command palette / overview ──────────────────────────────────────────
  { binding = "Control+Option+Space"; id = "openCommandPalette"; }
  { binding = "Control+Option+M";     id = "openMenuAnywhere"; }

  # ── Quake terminal — 無効化（既存 Ghostty 運用と独立） ──────────────────
  { binding = "Unassigned"; id = "toggleQuakeTerminal"; }

  # ── 残りの action は明示的に Unassigned にして TOML 全網羅 ───────────────
  { binding = "Unassigned"; id = "switchWorkspace.next"; }
  { binding = "Unassigned"; id = "switchWorkspace.previous"; }
  { binding = "Unassigned"; id = "focusPrevious"; }
  { binding = "Unassigned"; id = "toggleNativeFullscreen"; }
  { binding = "Unassigned"; id = "moveToRoot"; }
  { binding = "Unassigned"; id = "toggleSplit"; }
  { binding = "Unassigned"; id = "swapSplit"; }
  { binding = "Unassigned"; id = "resizeGrow.left"; }
  { binding = "Unassigned"; id = "resizeGrow.right"; }
  { binding = "Unassigned"; id = "resizeGrow.up"; }
  { binding = "Unassigned"; id = "resizeGrow.down"; }
  { binding = "Unassigned"; id = "resizeShrink.left"; }
  { binding = "Unassigned"; id = "resizeShrink.right"; }
  { binding = "Unassigned"; id = "resizeShrink.up"; }
  { binding = "Unassigned"; id = "resizeShrink.down"; }
  { binding = "Unassigned"; id = "preselect.left"; }
  { binding = "Unassigned"; id = "preselect.right"; }
  { binding = "Unassigned"; id = "preselect.up"; }
  { binding = "Unassigned"; id = "preselect.down"; }
  { binding = "Unassigned"; id = "preselectClear"; }
  { binding = "Unassigned"; id = "rescueOffscreenWindows"; }
  { binding = "Unassigned"; id = "assignFocusedWindowToScratchpad"; }
  { binding = "Unassigned"; id = "toggleScratchpadWindow"; }
  { binding = "Unassigned"; id = "toggleWorkspaceBarVisibility"; }
  { binding = "Unassigned"; id = "toggleHiddenBar"; }
  { binding = "Unassigned"; id = "moveColumn.left"; }
  { binding = "Unassigned"; id = "moveColumn.right"; }
  { binding = "Unassigned"; id = "focusColumnFirst"; }
  { binding = "Unassigned"; id = "focusColumnLast"; }
  { binding = "Unassigned"; id = "focusColumn.0"; }
  { binding = "Unassigned"; id = "focusColumn.1"; }
  { binding = "Unassigned"; id = "focusColumn.2"; }
  { binding = "Unassigned"; id = "focusColumn.3"; }
  { binding = "Unassigned"; id = "focusColumn.4"; }
  { binding = "Unassigned"; id = "focusColumn.5"; }
  { binding = "Unassigned"; id = "focusColumn.6"; }
  { binding = "Unassigned"; id = "focusColumn.7"; }
  { binding = "Unassigned"; id = "focusColumn.8"; }
]
