# ── キーバインド（OmniWM 0.5.9 / 149 action ID 全列挙）──────────────────────
#
# ⚠️ このファイルは生成物に近い。ID 一覧は **実機の OmniWM が書き戻した
# ~/.config/omniwm/settings.toml から機械抽出**したもので、手打ちしていない。
# OmniWM を上げて ID が増減したら、同じ方法で再抽出して差し替える:
#
#   grep -A2 '^\[\[hotkeys\]\]' ~/.config/omniwm/settings.toml \
#     | grep '^id = ' | sed 's/^id = "\(.*\)"/  { id = "\1"; binding = "Unassigned"; }/'
#
# 件数の確認は `omniwmctl query capabilities --format json` の
# payload.appVersion / protocolVersion と併せて行う。
#
# ── 方針 ──
# ウィンドウ操作は **すべて Karabiner の space-leader → omniwmctl** に集約する
# （Option ベースは打鍵しづらいという判断）。よって native hotkey は原則全部
# "Unassigned" にする。
#
# ただし `general.hotkeysEnabled = true` は維持し、**緊急脱出用の 2 つだけ native に
# 二重で残す**。Karabiner が落ちた / IPC が死んだときに、最低限フルスクリーンの
# 解除と画面外ウィンドウの回収ができるようにするため。同じ command を space 側と
# 二重に割り当てても競合しない。
#
# ── 全列挙する理由 ──
# 0.5.9 は欠損キーを default で補うので、実は全列挙しなくても壊れない
# （SettingsTOMLCodec が keyNotFound を捕まえて recovering デコードする）。
# それでも全部書くのは、**upstream が default binding を変えても我々の宣言が勝つ**
# ようにするため。0.5.9 は 16 個の action ID をリネームしており、旧 ID に当てた
# 設定は黙って無視されて upstream default が採用される、という事故が実際に起きていた。
#
# ── binding 文字列の正規形 ──
# 記号は英語名。`Option+,` ではなく `Option+Comma`、`Option+/` は `Option+Slash`。
# 他に Period / Minus / Equal / Return / Space / Left Arrow / Home / End / Page Up 等。
# 未割当は "Unassigned"。
{ ... }:
[
  { id = "switchWorkspace.0"; binding = "Unassigned"; }
  { id = "moveToWorkspace.0"; binding = "Unassigned"; }
  { id = "switchWorkspace.1"; binding = "Unassigned"; }
  { id = "moveToWorkspace.1"; binding = "Unassigned"; }
  { id = "switchWorkspace.2"; binding = "Unassigned"; }
  { id = "moveToWorkspace.2"; binding = "Unassigned"; }
  { id = "switchWorkspace.3"; binding = "Unassigned"; }
  { id = "moveToWorkspace.3"; binding = "Unassigned"; }
  { id = "switchWorkspace.4"; binding = "Unassigned"; }
  { id = "moveToWorkspace.4"; binding = "Unassigned"; }
  { id = "switchWorkspace.5"; binding = "Unassigned"; }
  { id = "moveToWorkspace.5"; binding = "Unassigned"; }
  { id = "switchWorkspace.6"; binding = "Unassigned"; }
  { id = "moveToWorkspace.6"; binding = "Unassigned"; }
  { id = "switchWorkspace.7"; binding = "Unassigned"; }
  { id = "moveToWorkspace.7"; binding = "Unassigned"; }
  { id = "switchWorkspace.8"; binding = "Unassigned"; }
  { id = "moveToWorkspace.8"; binding = "Unassigned"; }
  { id = "workspaceBackAndForth"; binding = "Unassigned"; }
  { id = "switchWorkspace.next"; binding = "Unassigned"; }
  { id = "switchWorkspace.previous"; binding = "Unassigned"; }
  { id = "focus.left"; binding = "Unassigned"; }
  { id = "focus.down"; binding = "Unassigned"; }
  { id = "focus.up"; binding = "Unassigned"; }
  { id = "focus.right"; binding = "Unassigned"; }
  { id = "focusPrevious"; binding = "Unassigned"; }
  { id = "focusDownOrLeft"; binding = "Unassigned"; }
  { id = "focusUpOrRight"; binding = "Unassigned"; }
  { id = "focusWindowTop"; binding = "Unassigned"; }
  { id = "focusWindowBottom"; binding = "Unassigned"; }
  { id = "focusWindowDownOrTop"; binding = "Unassigned"; }
  { id = "focusWindowUpOrBottom"; binding = "Unassigned"; }
  { id = "focusWindowOrWorkspaceDown"; binding = "Unassigned"; }
  { id = "focusWindowOrWorkspaceUp"; binding = "Unassigned"; }
  { id = "centerColumn"; binding = "Unassigned"; }
  { id = "centerVisibleColumns"; binding = "Unassigned"; }
  { id = "moveWindowToWorkspaceUp"; binding = "Unassigned"; }
  { id = "moveWindowToWorkspaceDown"; binding = "Unassigned"; }
  { id = "moveColumnToWorkspaceUp"; binding = "Unassigned"; }
  { id = "moveColumnToWorkspaceDown"; binding = "Unassigned"; }
  { id = "moveColumnToWorkspace.0"; binding = "Unassigned"; }
  { id = "moveColumnToWorkspace.1"; binding = "Unassigned"; }
  { id = "moveColumnToWorkspace.2"; binding = "Unassigned"; }
  { id = "moveColumnToWorkspace.3"; binding = "Unassigned"; }
  { id = "moveColumnToWorkspace.4"; binding = "Unassigned"; }
  { id = "moveColumnToWorkspace.5"; binding = "Unassigned"; }
  { id = "moveColumnToWorkspace.6"; binding = "Unassigned"; }
  { id = "moveColumnToWorkspace.7"; binding = "Unassigned"; }
  { id = "moveColumnToWorkspace.8"; binding = "Unassigned"; }
  { id = "move.left"; binding = "Unassigned"; }
  { id = "move.down"; binding = "Unassigned"; }
  { id = "move.up"; binding = "Unassigned"; }
  { id = "move.right"; binding = "Unassigned"; }
  { id = "moveWindowDown"; binding = "Unassigned"; }
  { id = "moveWindowUp"; binding = "Unassigned"; }
  { id = "moveWindowDownOrToWorkspaceDown"; binding = "Unassigned"; }
  { id = "moveWindowUpOrToWorkspaceUp"; binding = "Unassigned"; }
  { id = "consumeOrExpelWindowLeft"; binding = "Unassigned"; }
  { id = "consumeOrExpelWindowRight"; binding = "Unassigned"; }
  { id = "consumeWindowIntoColumn"; binding = "Unassigned"; }
  { id = "expelWindowFromColumn"; binding = "Unassigned"; }
  { id = "focusMonitorNext"; binding = "Unassigned"; }
  { id = "focusMonitorPrevious"; binding = "Unassigned"; }
  { id = "focusMonitorLast"; binding = "Unassigned"; }
  # 緊急脱出用に native 併記（space 側にも割り当て済み）
  { id = "toggleFullscreen"; binding = "Option+Return"; }
  { id = "toggleNativeFullscreen"; binding = "Unassigned"; }
  { id = "moveColumn.left"; binding = "Unassigned"; }
  { id = "moveColumn.right"; binding = "Unassigned"; }
  { id = "moveColumn.up"; binding = "Unassigned"; }
  { id = "moveColumn.down"; binding = "Unassigned"; }
  { id = "moveColumnToFirst"; binding = "Unassigned"; }
  { id = "moveColumnToLast"; binding = "Unassigned"; }
  { id = "toggleColumnTabbed"; binding = "Unassigned"; }
  { id = "focusColumnFirst"; binding = "Unassigned"; }
  { id = "focusColumnLast"; binding = "Unassigned"; }
  { id = "focusColumn.0"; binding = "Unassigned"; }
  { id = "focusColumn.1"; binding = "Unassigned"; }
  { id = "focusColumn.2"; binding = "Unassigned"; }
  { id = "focusColumn.3"; binding = "Unassigned"; }
  { id = "focusColumn.4"; binding = "Unassigned"; }
  { id = "focusColumn.5"; binding = "Unassigned"; }
  { id = "focusColumn.6"; binding = "Unassigned"; }
  { id = "focusColumn.7"; binding = "Unassigned"; }
  { id = "focusColumn.8"; binding = "Unassigned"; }
  { id = "focusWindowInColumn.1"; binding = "Unassigned"; }
  { id = "focusWindowInColumn.2"; binding = "Unassigned"; }
  { id = "focusWindowInColumn.3"; binding = "Unassigned"; }
  { id = "focusWindowInColumn.4"; binding = "Unassigned"; }
  { id = "focusWindowInColumn.5"; binding = "Unassigned"; }
  { id = "focusWindowInColumn.6"; binding = "Unassigned"; }
  { id = "focusWindowInColumn.7"; binding = "Unassigned"; }
  { id = "focusWindowInColumn.8"; binding = "Unassigned"; }
  { id = "focusWindowInColumn.9"; binding = "Unassigned"; }
  { id = "moveColumnToIndex.1"; binding = "Unassigned"; }
  { id = "moveColumnToIndex.2"; binding = "Unassigned"; }
  { id = "moveColumnToIndex.3"; binding = "Unassigned"; }
  { id = "moveColumnToIndex.4"; binding = "Unassigned"; }
  { id = "moveColumnToIndex.5"; binding = "Unassigned"; }
  { id = "moveColumnToIndex.6"; binding = "Unassigned"; }
  { id = "moveColumnToIndex.7"; binding = "Unassigned"; }
  { id = "moveColumnToIndex.8"; binding = "Unassigned"; }
  { id = "moveColumnToIndex.9"; binding = "Unassigned"; }
  { id = "cycleSizeForward"; binding = "Unassigned"; }
  { id = "cycleSizeBackward"; binding = "Unassigned"; }
  { id = "cycleWindowPrimarySpanForward"; binding = "Unassigned"; }
  { id = "cycleWindowPrimarySpanBackward"; binding = "Unassigned"; }
  { id = "cycleWindowSecondarySpanForward"; binding = "Unassigned"; }
  { id = "cycleWindowSecondarySpanBackward"; binding = "Unassigned"; }
  { id = "toggleContainerFullPrimarySpan"; binding = "Unassigned"; }
  { id = "expandContainerToAvailablePrimarySpan"; binding = "Unassigned"; }
  { id = "resetWindowSecondarySpan"; binding = "Unassigned"; }
  { id = "setContainerPrimarySpan.decrease10Percent"; binding = "Unassigned"; }
  { id = "setContainerPrimarySpan.increase10Percent"; binding = "Unassigned"; }
  { id = "setWindowPrimarySpan.decrease10Percent"; binding = "Unassigned"; }
  { id = "setWindowPrimarySpan.increase10Percent"; binding = "Unassigned"; }
  { id = "setWindowSecondarySpan.decrease10Percent"; binding = "Unassigned"; }
  { id = "setWindowSecondarySpan.increase10Percent"; binding = "Unassigned"; }
  { id = "balanceSizes"; binding = "Unassigned"; }
  { id = "moveToRoot"; binding = "Unassigned"; }
  { id = "toggleSplit"; binding = "Unassigned"; }
  { id = "swapSplit"; binding = "Unassigned"; }
  { id = "resizeGrow.left"; binding = "Unassigned"; }
  { id = "resizeGrow.right"; binding = "Unassigned"; }
  { id = "resizeGrow.up"; binding = "Unassigned"; }
  { id = "resizeGrow.down"; binding = "Unassigned"; }
  { id = "resizeShrink.left"; binding = "Unassigned"; }
  { id = "resizeShrink.right"; binding = "Unassigned"; }
  { id = "resizeShrink.up"; binding = "Unassigned"; }
  { id = "resizeShrink.down"; binding = "Unassigned"; }
  { id = "resizeFocusedWindow.grow"; binding = "Unassigned"; }
  { id = "resizeFocusedWindow.shrink"; binding = "Unassigned"; }
  { id = "preselect.left"; binding = "Unassigned"; }
  { id = "preselect.right"; binding = "Unassigned"; }
  { id = "preselect.up"; binding = "Unassigned"; }
  { id = "preselect.down"; binding = "Unassigned"; }
  { id = "preselectClear"; binding = "Unassigned"; }
  { id = "openCommandPalette"; binding = "Unassigned"; }
  { id = "raiseAllFloatingWindows"; binding = "Unassigned"; }
  # 緊急脱出用に native 併記（space 側にも割り当て済み）
  { id = "rescueOffscreenWindows"; binding = "Control+Option+Shift+R"; }
  { id = "toggleFocusedWindowFloating"; binding = "Unassigned"; }
  { id = "assignFocusedWindowToScratchpad"; binding = "Unassigned"; }
  { id = "toggleScratchpadWindow"; binding = "Unassigned"; }
  { id = "openMenuAnywhere"; binding = "Unassigned"; }
  { id = "toggleWorkspaceBarVisibility"; binding = "Unassigned"; }
  { id = "toggleHiddenBarPanel"; binding = "Unassigned"; }
  { id = "toggleQuakeTerminal"; binding = "Unassigned"; }
  { id = "toggleWorkspaceLayout"; binding = "Unassigned"; }
  { id = "toggleOverview"; binding = "Unassigned"; }
  { id = "toggleSystemStats"; binding = "Unassigned"; }
]
