# ── キーバインド（OmniWM 0.4.9 完全対応） ────────────────────────────────
# OmniWM 0.4.9 は [[hotkeys]] 配列に **すべての 144 hotkey ID** を要求する。
# 一部でも欠けると settings.toml が `.corrupt` rename されて全 nix-config が捨てられる。
# よって defaults (144 個) + 我々の overrides を merge する構造とする。
#
# Karabiner レイヤ（karabiner-rules.nix）で実装するもの:
#   - alt-s / alt-c / alt-a       (シェル実行マクロ: WS M + アプリ起動)
#   - alt-ctrl-m                  (setup-media-workspace)
#   - alt-m / alt-b / alt-e       (名前指定 WS 切替: omniwmctl workspace focus-name)
#   - alt-shift-m / b / e         (名前指定 WS への送り)
#   - alt-ctrl-h/j/k/l            (方向ベース focus-monitor)
#   - cmd-h                       (macOS Hide ブロック)
{ ... }:
let
  # ── OmniWM 0.4.9 の 144 hotkey デフォルト ─────────────────────────────
  # `omniwmctl` が settings.toml に書き戻すと alphabetical 順で全 ID を出力する。
  # この一覧はその write-back を元にしている。OmniWM が新しい hotkey を追加した場合は
  # この list を更新する必要あり（旧 0.4.9 で動く間は問題なし）。
  defaults = [
    { id = "assignFocusedWindowToScratchpad"; binding = "Unassigned"; }
    { id = "balanceSizes"; binding = "Option+Shift+B"; }
    { id = "centerColumn"; binding = "Unassigned"; }
    { id = "centerVisibleColumns"; binding = "Unassigned"; }
    { id = "consumeOrExpelWindowLeft"; binding = "Unassigned"; }
    { id = "consumeOrExpelWindowRight"; binding = "Unassigned"; }
    { id = "consumeWindowIntoColumn"; binding = "Unassigned"; }
    { id = "cycleColumnWidthBackward"; binding = "Option+,"; }
    { id = "cycleColumnWidthForward"; binding = "Option+."; }
    { id = "cycleWindowHeightBackward"; binding = "Unassigned"; }
    { id = "cycleWindowHeightForward"; binding = "Unassigned"; }
    { id = "cycleWindowWidthBackward"; binding = "Unassigned"; }
    { id = "cycleWindowWidthForward"; binding = "Unassigned"; }
    { id = "expandColumnToAvailableWidth"; binding = "Control+Option+F"; }
    { id = "expelWindowFromColumn"; binding = "Unassigned"; }
    { id = "focus.down"; binding = "Option+Down Arrow"; }
    { id = "focus.left"; binding = "Option+Left Arrow"; }
    { id = "focus.right"; binding = "Option+Right Arrow"; }
    { id = "focus.up"; binding = "Option+Up Arrow"; }
    { id = "focusColumn.0"; binding = "Control+Option+1"; }
    { id = "focusColumn.1"; binding = "Control+Option+2"; }
    { id = "focusColumn.2"; binding = "Control+Option+3"; }
    { id = "focusColumn.3"; binding = "Control+Option+4"; }
    { id = "focusColumn.4"; binding = "Control+Option+5"; }
    { id = "focusColumn.5"; binding = "Control+Option+6"; }
    { id = "focusColumn.6"; binding = "Control+Option+7"; }
    { id = "focusColumn.7"; binding = "Control+Option+8"; }
    { id = "focusColumn.8"; binding = "Control+Option+9"; }
    { id = "focusColumnFirst"; binding = "Option+Home"; }
    { id = "focusColumnLast"; binding = "Option+End"; }
    { id = "focusDownOrLeft"; binding = "Unassigned"; }
    { id = "focusMonitorLast"; binding = "Control+Command+`"; }
    { id = "focusMonitorNext"; binding = "Control+Command+Tab"; }
    { id = "focusMonitorPrevious"; binding = "Unassigned"; }
    { id = "focusPrevious"; binding = "Option+Tab"; }
    { id = "focusUpOrRight"; binding = "Unassigned"; }
    { id = "focusWindowBottom"; binding = "Unassigned"; }
    { id = "focusWindowDownOrTop"; binding = "Unassigned"; }
    { id = "focusWindowInColumn.1"; binding = "Unassigned"; }
    { id = "focusWindowInColumn.2"; binding = "Unassigned"; }
    { id = "focusWindowInColumn.3"; binding = "Unassigned"; }
    { id = "focusWindowInColumn.4"; binding = "Unassigned"; }
    { id = "focusWindowInColumn.5"; binding = "Unassigned"; }
    { id = "focusWindowInColumn.6"; binding = "Unassigned"; }
    { id = "focusWindowInColumn.7"; binding = "Unassigned"; }
    { id = "focusWindowInColumn.8"; binding = "Unassigned"; }
    { id = "focusWindowInColumn.9"; binding = "Unassigned"; }
    { id = "focusWindowOrWorkspaceDown"; binding = "Unassigned"; }
    { id = "focusWindowOrWorkspaceUp"; binding = "Unassigned"; }
    { id = "focusWindowTop"; binding = "Unassigned"; }
    { id = "focusWindowUpOrBottom"; binding = "Unassigned"; }
    { id = "move.down"; binding = "Option+Shift+Down Arrow"; }
    { id = "move.left"; binding = "Option+Shift+Left Arrow"; }
    { id = "move.right"; binding = "Option+Shift+Right Arrow"; }
    { id = "move.up"; binding = "Option+Shift+Up Arrow"; }
    { id = "moveColumn.left"; binding = "Control+Option+Shift+Left Arrow"; }
    { id = "moveColumn.right"; binding = "Control+Option+Shift+Right Arrow"; }
    { id = "moveColumnToFirst"; binding = "Control+Option+Home"; }
    { id = "moveColumnToIndex.1"; binding = "Unassigned"; }
    { id = "moveColumnToIndex.2"; binding = "Unassigned"; }
    { id = "moveColumnToIndex.3"; binding = "Unassigned"; }
    { id = "moveColumnToIndex.4"; binding = "Unassigned"; }
    { id = "moveColumnToIndex.5"; binding = "Unassigned"; }
    { id = "moveColumnToIndex.6"; binding = "Unassigned"; }
    { id = "moveColumnToIndex.7"; binding = "Unassigned"; }
    { id = "moveColumnToIndex.8"; binding = "Unassigned"; }
    { id = "moveColumnToIndex.9"; binding = "Unassigned"; }
    { id = "moveColumnToLast"; binding = "Control+Option+End"; }
    { id = "moveColumnToWorkspace.0"; binding = "Unassigned"; }
    { id = "moveColumnToWorkspace.1"; binding = "Unassigned"; }
    { id = "moveColumnToWorkspace.2"; binding = "Unassigned"; }
    { id = "moveColumnToWorkspace.3"; binding = "Unassigned"; }
    { id = "moveColumnToWorkspace.4"; binding = "Unassigned"; }
    { id = "moveColumnToWorkspace.5"; binding = "Unassigned"; }
    { id = "moveColumnToWorkspace.6"; binding = "Unassigned"; }
    { id = "moveColumnToWorkspace.7"; binding = "Unassigned"; }
    { id = "moveColumnToWorkspace.8"; binding = "Unassigned"; }
    { id = "moveColumnToWorkspaceDown"; binding = "Control+Option+Shift+Page Down"; }
    { id = "moveColumnToWorkspaceUp"; binding = "Control+Option+Shift+Page Up"; }
    { id = "moveToRoot"; binding = "Unassigned"; }
    { id = "moveToWorkspace.0"; binding = "Option+Shift+1"; }
    { id = "moveToWorkspace.1"; binding = "Option+Shift+2"; }
    { id = "moveToWorkspace.2"; binding = "Option+Shift+3"; }
    { id = "moveToWorkspace.3"; binding = "Option+Shift+4"; }
    { id = "moveToWorkspace.4"; binding = "Option+Shift+5"; }
    { id = "moveToWorkspace.5"; binding = "Option+Shift+6"; }
    { id = "moveToWorkspace.6"; binding = "Option+Shift+7"; }
    { id = "moveToWorkspace.7"; binding = "Option+Shift+8"; }
    { id = "moveToWorkspace.8"; binding = "Option+Shift+9"; }
    { id = "moveWindowDown"; binding = "Unassigned"; }
    { id = "moveWindowDownOrToWorkspaceDown"; binding = "Unassigned"; }
    { id = "moveWindowToWorkspaceDown"; binding = "Control+Option+Shift+Down Arrow"; }
    { id = "moveWindowToWorkspaceUp"; binding = "Control+Option+Shift+Up Arrow"; }
    { id = "moveWindowUp"; binding = "Unassigned"; }
    { id = "moveWindowUpOrToWorkspaceUp"; binding = "Unassigned"; }
    { id = "openCommandPalette"; binding = "Control+Option+Space"; }
    { id = "openMenuAnywhere"; binding = "Control+Option+M"; }
    { id = "preselect.down"; binding = "Unassigned"; }
    { id = "preselect.left"; binding = "Unassigned"; }
    { id = "preselect.right"; binding = "Unassigned"; }
    { id = "preselect.up"; binding = "Unassigned"; }
    { id = "preselectClear"; binding = "Unassigned"; }
    { id = "raiseAllFloatingWindows"; binding = "Option+Shift+R"; }
    { id = "rescueOffscreenWindows"; binding = "Unassigned"; }
    { id = "resetWindowHeight"; binding = "Control+Option+R"; }
    { id = "resizeGrow.down"; binding = "Unassigned"; }
    { id = "resizeGrow.left"; binding = "Unassigned"; }
    { id = "resizeGrow.right"; binding = "Unassigned"; }
    { id = "resizeGrow.up"; binding = "Unassigned"; }
    { id = "resizeShrink.down"; binding = "Unassigned"; }
    { id = "resizeShrink.left"; binding = "Unassigned"; }
    { id = "resizeShrink.right"; binding = "Unassigned"; }
    { id = "resizeShrink.up"; binding = "Unassigned"; }
    { id = "setColumnWidth.decrease10Percent"; binding = "Option+-"; }
    { id = "setColumnWidth.increase10Percent"; binding = "Option+="; }
    { id = "setWindowHeight.decrease10Percent"; binding = "Option+Shift+-"; }
    { id = "setWindowHeight.increase10Percent"; binding = "Option+Shift+="; }
    { id = "setWindowWidth.decrease10Percent"; binding = "Unassigned"; }
    { id = "setWindowWidth.increase10Percent"; binding = "Unassigned"; }
    { id = "swapSplit"; binding = "Unassigned"; }
    { id = "switchWorkspace.0"; binding = "Option+1"; }
    { id = "switchWorkspace.1"; binding = "Option+2"; }
    { id = "switchWorkspace.2"; binding = "Option+3"; }
    { id = "switchWorkspace.3"; binding = "Option+4"; }
    { id = "switchWorkspace.4"; binding = "Option+5"; }
    { id = "switchWorkspace.5"; binding = "Option+6"; }
    { id = "switchWorkspace.6"; binding = "Option+7"; }
    { id = "switchWorkspace.7"; binding = "Option+8"; }
    { id = "switchWorkspace.8"; binding = "Option+9"; }
    { id = "switchWorkspace.next"; binding = "Unassigned"; }
    { id = "switchWorkspace.previous"; binding = "Unassigned"; }
    { id = "toggleColumnFullWidth"; binding = "Option+Shift+F"; }
    { id = "toggleColumnTabbed"; binding = "Option+T"; }
    { id = "toggleFocusedWindowFloating"; binding = "Unassigned"; }
    { id = "toggleFullscreen"; binding = "Option+Return"; }
    { id = "toggleHiddenBar"; binding = "Unassigned"; }
    { id = "toggleNativeFullscreen"; binding = "Unassigned"; }
    { id = "toggleOverview"; binding = "Option+Shift+O"; }
    { id = "toggleQuakeTerminal"; binding = "Option+`"; }
    { id = "toggleScratchpadWindow"; binding = "Unassigned"; }
    { id = "toggleSplit"; binding = "Unassigned"; }
    { id = "toggleWorkspaceBarVisibility"; binding = "Unassigned"; }
    { id = "toggleWorkspaceLayout"; binding = "Option+Shift+L"; }
    { id = "workspaceBackAndForth"; binding = "Control+Option+Tab"; }
  ];

  # ── 我々の意図的なバインド（defaults を上書き、または明示的にデフォルト維持）─
  # 哲学: niri 流に最適化、AeroSpace 由来の擬似機能は削除済み。
  # 暗黙のデフォルト依存をなくすため、運用上意味があるキーは値が同じでも明示する。
  # OmniWM 上流がデフォルトを変えても、ここに書いた値は維持される。
  overrides = {
    # ━━━ SPACE-leader 移行済み — すべて Unassigned (§11.3 v2.6) ━━━━━━━━━
    # 位置/focus/move 系は space+leader で実装。OmniWM 内蔵 hotkey は解除。

    # 数字 WS 切替: space+1..9 で代替
    "switchWorkspace.0" = "Unassigned";
    "switchWorkspace.1" = "Unassigned";
    "switchWorkspace.2" = "Unassigned";
    "switchWorkspace.3" = "Unassigned";
    "switchWorkspace.4" = "Unassigned";
    "switchWorkspace.5" = "Unassigned";
    "switchWorkspace.6" = "Unassigned";
    "switchWorkspace.7" = "Unassigned";
    "switchWorkspace.8" = "Unassigned";

    # WS next / prev / back-and-forth: space+]/[ / space+tab で代替
    "switchWorkspace.next"     = "Unassigned";
    "switchWorkspace.previous" = "Unassigned";
    "workspaceBackAndForth"    = "Unassigned";

    # 数字 WS への送り: space+shift+1..9 で代替
    "moveToWorkspace.0" = "Unassigned";
    "moveToWorkspace.1" = "Unassigned";
    "moveToWorkspace.2" = "Unassigned";
    "moveToWorkspace.3" = "Unassigned";
    "moveToWorkspace.4" = "Unassigned";
    "moveToWorkspace.5" = "Unassigned";
    "moveToWorkspace.6" = "Unassigned";
    "moveToWorkspace.7" = "Unassigned";
    "moveToWorkspace.8" = "Unassigned";

    # WS up/down への送り: space+shift+]/[ で代替
    "moveWindowToWorkspaceDown" = "Unassigned";
    "moveWindowToWorkspaceUp"   = "Unassigned";

    # フォーカス方向: space+hjkl で代替
    "focus.left"  = "Unassigned";
    "focus.down"  = "Unassigned";
    "focus.up"    = "Unassigned";
    "focus.right" = "Unassigned";

    # 直前ウィンドウ: space+; で代替
    "focusPrevious" = "Unassigned";

    # ウィンドウ移動: space+shift+hjkl で代替
    "move.left"  = "Unassigned";
    "move.down"  = "Unassigned";
    "move.up"    = "Unassigned";
    "move.right" = "Unassigned";

    # Column focus: space+ctrl+1..9 / space+ctrl+[/] で代替
    "focusColumn.0" = "Unassigned";
    "focusColumn.1" = "Unassigned";
    "focusColumn.2" = "Unassigned";
    "focusColumn.3" = "Unassigned";
    "focusColumn.4" = "Unassigned";
    "focusColumn.5" = "Unassigned";
    "focusColumn.6" = "Unassigned";
    "focusColumn.7" = "Unassigned";
    "focusColumn.8" = "Unassigned";
    "focusColumnFirst" = "Unassigned";
    "focusColumnLast"  = "Unassigned";

    # Column 移動: space+ctrl+shift+h/l で代替
    "moveColumn.left"  = "Unassigned";
    "moveColumn.right" = "Unassigned";

    # Column → WS: space+ctrl+shift+1..9 / [/] で代替
    "moveColumnToWorkspace.0" = "Unassigned";
    "moveColumnToWorkspace.1" = "Unassigned";
    "moveColumnToWorkspace.2" = "Unassigned";
    "moveColumnToWorkspace.3" = "Unassigned";
    "moveColumnToWorkspace.4" = "Unassigned";
    "moveColumnToWorkspace.5" = "Unassigned";
    "moveColumnToWorkspace.6" = "Unassigned";
    "moveColumnToWorkspace.7" = "Unassigned";
    "moveColumnToWorkspace.8" = "Unassigned";
    "moveColumnToWorkspaceDown" = "Unassigned";
    "moveColumnToWorkspaceUp"   = "Unassigned";

    # monitor focus last/prev: space+ctrl+tab (next のみ) で代替
    "focusMonitorLast"     = "Unassigned";
    "focusMonitorNext"     = "Unassigned";
    "focusMonitorPrevious" = "Unassigned";

    # その他 廃止 / Unassigned (spec §11 Unassigned リスト)
    "toggleQuakeTerminal"          = "Unassigned";  # §11.4 廃止
    "openMenuAnywhere"             = "Unassigned";  # space+ctrl+m と衝突
    "toggleScratchpadWindow"       = "Unassigned";
    "assignFocusedWindowToScratchpad" = "Unassigned";
    "moveToRoot"                   = "Unassigned";
    "toggleNativeFullscreen"       = "Unassigned";
    "toggleWorkspaceBarVisibility" = "Unassigned";
    "toggleHiddenBar"              = "Unassigned";
    "toggleSplit"                  = "Unassigned";
    "swapSplit"                    = "Unassigned";
    "centerColumn"                 = "Unassigned";
    "centerVisibleColumns"         = "Unassigned";
    "consumeOrExpelWindowLeft"     = "Unassigned";
    "consumeOrExpelWindowRight"    = "Unassigned";
    "consumeWindowIntoColumn"      = "Unassigned";
    "expelWindowFromColumn"        = "Unassigned";
    "cycleWindowWidthForward"      = "Unassigned";
    "cycleWindowWidthBackward"     = "Unassigned";
    "cycleWindowHeightForward"     = "Unassigned";
    "cycleWindowHeightBackward"    = "Unassigned";
    "setWindowWidth.decrease10Percent" = "Unassigned";
    "setWindowWidth.increase10Percent" = "Unassigned";
    "resizeGrow.down"   = "Unassigned";
    "resizeGrow.left"   = "Unassigned";
    "resizeGrow.right"  = "Unassigned";
    "resizeGrow.up"     = "Unassigned";
    "resizeShrink.down"  = "Unassigned";
    "resizeShrink.left"  = "Unassigned";
    "resizeShrink.right" = "Unassigned";
    "resizeShrink.up"    = "Unassigned";
    "focusUpOrRight"            = "Unassigned";
    "focusDownOrLeft"           = "Unassigned";
    "focusWindowDownOrTop"      = "Unassigned";
    "focusWindowUpOrBottom"     = "Unassigned";
    "focusWindowOrWorkspaceDown" = "Unassigned";
    "focusWindowOrWorkspaceUp"   = "Unassigned";
    "focusWindowInColumn.1" = "Unassigned";
    "focusWindowInColumn.2" = "Unassigned";
    "focusWindowInColumn.3" = "Unassigned";
    "focusWindowInColumn.4" = "Unassigned";
    "focusWindowInColumn.5" = "Unassigned";
    "focusWindowInColumn.6" = "Unassigned";
    "focusWindowInColumn.7" = "Unassigned";
    "focusWindowInColumn.8" = "Unassigned";
    "focusWindowInColumn.9" = "Unassigned";
    "focusWindowTop"    = "Unassigned";
    "focusWindowBottom" = "Unassigned";
    "moveWindowDown"    = "Unassigned";
    "moveWindowUp"      = "Unassigned";
    "moveWindowDownOrToWorkspaceDown" = "Unassigned";
    "moveWindowUpOrToWorkspaceUp"     = "Unassigned";
    "moveColumnToFirst" = "Unassigned";
    "moveColumnToLast"  = "Unassigned";
    "moveColumnToIndex.1" = "Unassigned";
    "moveColumnToIndex.2" = "Unassigned";
    "moveColumnToIndex.3" = "Unassigned";
    "moveColumnToIndex.4" = "Unassigned";
    "moveColumnToIndex.5" = "Unassigned";
    "moveColumnToIndex.6" = "Unassigned";
    "moveColumnToIndex.7" = "Unassigned";
    "moveColumnToIndex.8" = "Unassigned";
    "moveColumnToIndex.9" = "Unassigned";
    "preselect.down"  = "Unassigned";
    "preselect.left"  = "Unassigned";
    "preselect.right" = "Unassigned";
    "preselect.up"    = "Unassigned";
    "preselectClear"  = "Unassigned";

    # ━━━ OPTION-base 維持 — size/構造/UI 系 (§11 OPTION-base 維持リスト) ━━━
    "cycleColumnWidthBackward" = "Option+,";
    "cycleColumnWidthForward"  = "Option+.";
    "setColumnWidth.decrease10Percent" = "Option+-";
    "setColumnWidth.increase10Percent" = "Option+=";
    "setWindowHeight.decrease10Percent" = "Option+Shift+-";
    "setWindowHeight.increase10Percent" = "Option+Shift+=";
    "expandColumnToAvailableWidth" = "Control+Option+F";
    "toggleColumnFullWidth"    = "Option+Shift+F";
    "toggleColumnTabbed"       = "Option+T";
    "toggleFullscreen"         = "Option+Return";
    "toggleFocusedWindowFloating" = "Option+Shift+Space";
    # balanceSizes → Option+/ に変更 (spec v2.6)
    "balanceSizes" = "Option+/";
    # toggleWorkspaceLayout → Option+L に変更 (spec v2.6、Option+Shift+L から)
    "toggleWorkspaceLayout"   = "Option+L";
    "toggleOverview"          = "Option+Shift+O";
    "raiseAllFloatingWindows" = "Option+Shift+R";
    "rescueOffscreenWindows"  = "Control+Option+Shift+R";
    "resetWindowHeight"       = "Control+Option+R";
    "openCommandPalette"      = "Control+Option+Space";
  };

in
  # 各 default に対して override があれば binding を上書き
  map (h:
    if overrides ? ${h.id}
    then h // { binding = overrides.${h.id}; }
    else h
  ) defaults
