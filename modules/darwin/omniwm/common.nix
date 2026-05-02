# ── OmniWM 共通設定 ────────────────────────────────────────────────────────
# モニタ構成に依存しないベース設定。プロファイルとマージされる。
#
# canonical-settings.toml のスキーマに従い、各セクションを attrset として返す。
# default.nix から import → recursiveUpdate でモニタ別プロファイルとマージ。
{ ... }:
{
  # ── スキーマバージョン ────────────────────────────────────────────────────
  version = 5;

  # ── モニタ別オーバーライド（未使用、空リストで明示） ──────────────────────
  monitorBarOverrides = [ ];
  monitorDwindleOverrides = [ ];
  monitorNiriOverrides = [ ];
  monitorOrientationOverrides = [ ];

  # ── 外観 ──────────────────────────────────────────────────────────────────
  appearance = { mode = "dark"; };

  # ── 内蔵ボーダー（JankyBorders を置換）──────────────────────────────────
  # AeroSpace 時の borders 設定: active=0xFFE8D44D (#E8D44D), inactive=0xFF3C3C3C, width=3
  borders = {
    enabled = true;
    width = 3.0;
    color = {
      red = 0.91;
      green = 0.83;
      blue = 0.30;
      alpha = 1.0;
    };
  };

  # ── Niri (scrolling columns) レイアウト既定値 ────────────────────────────
  # フォーカス column を常に画面中央、column 幅 preset を細かめに、
  # 同時可視 column を 3 つに（広めの画面で見渡しやすい）
  niri = {
    alwaysCenterSingleColumn = true;
    centerFocusedColumn = "always";
    columnWidthPresets = [ 0.25 0.3333 0.5 0.6666 0.8 ];
    infiniteLoop = false;
    maxVisibleColumns = 3;
    maxWindowsPerColumn = 4;
    singleWindowAspectRatio = "4:3";
  };

  # ── Dwindle (BSP) — Option+/ で WS 単位に切替えて使う ───────────────────
  # smartSplit ON で長辺方向に自動分割（タイル比率が崩れにくい）
  dwindle = {
    defaultSplitRatio = 1.0;
    moveToRootStable = true;
    singleWindowAspectRatio = "4:3";
    smartSplit = true;
    splitWidthMultiplier = 1.0;
    useGlobalGaps = true;
  };

  # ── フォーカス挙動 ────────────────────────────────────────────────────────
  # followsMouse = true は niri 流だがマウス操作中にチラつき/誤動作が出やすい。
  # キーボード主体の運用に戻す（フォーカス変更時にだけマウスが追ってくる）。
  focus = {
    followsMouse = false;
    followsWindowToMonitor = true;     # キーで WS を跨ぐとマウスも一緒に動く
    moveMouseToFocusedWindow = true;   # キーでフォーカス変えるとマウスが追従
  };

  # ── Gaps ──────────────────────────────────────────────────────────────────
  # AeroSpace: inner=8, outer={left=8,right=8,top=8,bottom=8}
  gaps = {
    size = 8.0;
    outer = {
      bottom = 8.0;
      left = 8.0;
      right = 8.0;
      top = 8.0;
    };
  };

  # ── 一般設定 ─────────────────────────────────────────────────────────────
  general = {
    animationsEnabled = true;
    defaultLayoutType = "niri";
    hotkeysEnabled = true;
    ipcEnabled = true;                   # omniwmctl の操作を許可
    preventSleepEnabled = false;
    updateChecksEnabled = true;
  };

  # ── トラックパッドジェスチャ ─────────────────────────────────────────────
  gestures = {
    fingerCount = 3;
    invertDirection = true;
    scrollEnabled = true;
    scrollModifierKey = "optionShift";
    scrollSensitivity = 5.0;
  };

  # ── マウス warp（モニタ間） ──────────────────────────────────────────────
  mouseWarp = {
    axis = "horizontal";
    margin = 1;
    monitorOrder = [ ];
  };

  # ── Quake terminal（OmniWM 内蔵 libghostty） ───────────────────────────
  # `Option+\`` でフォーカス中のモニタにスライド表示される。既存 Ghostty.app と併存可能。
  quakeTerminal = {
    animationDuration = 0.2;
    autoHide = false;
    enabled = true;
    heightPercent = 50.0;
    monitorMode = "focusedWindow";
    opacity = 1.0;
    position = "center";
    useCustomFrame = false;
    widthPercent = 50.0;
  };

  # ── ステータスバー ────────────────────────────────────────────────────────
  # アプリ名表示で「今何が立ってるか」を一目で把握、WS 名 (M/B/E) も見えるように
  statusBar = {
    showAppNames = true;
    showWorkspaceName = true;
    useWorkspaceId = false;
  };

  # ── ワークスペースバー ────────────────────────────────────────────────────
  # floating ウィンドウもバーに表示、空 WS は隠して整理
  workspaceBar = {
    backgroundOpacity = 0.1;
    deduplicateAppIcons = false;
    enabled = true;
    height = 24.0;
    hideEmptyWorkspaces = true;
    labelFontSize = 12.0;
    notchAware = true;
    position = "overlappingMenuBar";
    reserveLayoutSpace = false;
    showFloatingWindows = true;
    showLabels = true;
    windowLevel = "popup";
    xOffset = 0.0;
    yOffset = 0.0;
    accentColor = { red = -1.0; green = -1.0; blue = -1.0; alpha = 1.0; };
    textColor   = { red = -1.0; green = -1.0; blue = -1.0; alpha = 1.0; };
  };

  # ── state（ランタイム保持、固定値で初期化） ──────────────────────────────
  state = {
    commandPaletteLastMode = "windows";
    hiddenBarIsCollapsed = true;
  };
}
