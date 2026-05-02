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
  # フォーカス column を常に画面中央、column 幅は広めをデフォルトに。
  #
  # 各設定の意味:
  #   defaultColumnWidth       : 新規ウィンドウが開く時の column 幅（画面幅に対する比）
  #   columnWidthPresets       : Option+, / Option+. で巡回する preset リスト
  #   singleWindowAspectRatio  : 1 column = 1 window の時の最大アスペクト比
  #     "16:10" にして横長を許容（旧 "4:3" だとワイドモニタで狭く見える）
  #   alwaysCenterSingleColumn : 1 column 時は中央に配置
  #   centerFocusedColumn      : 常にフォーカス column を画面中央
  #   maxVisibleColumns        : 同時表示する column 数（広いモニタで増やすと見渡しやすい）
  niri = {
    alwaysCenterSingleColumn = true;
    centerFocusedColumn = "on-overflow";  # 通常は左端 packing、画面に収まらない時のみ中央寄せ
                                          # ("always" だとフォーカス左の column が画面外に押し出される)
    columnWidthPresets = [ 0.4 0.5 0.66 0.8 0.95 ];
    defaultColumnWidth = 0.66;
    infiniteLoop = false;
    maxVisibleColumns = 3;
    maxWindowsPerColumn = 4;
    singleWindowAspectRatio = "16:10";
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
  # OmniWM の mouseWarp は「画面端でカーソルを別モニタに飛ばす」拡張機能。
  # axis = "horizontal" / "vertical" の単一軸でしか動かないため、L 字型配置などの
  # 複雑なモニタ配置には対応できない（横方向 or 縦方向のどちらかしか面倒見ない）。
  #
  # 対処: margin = 0 で warp トリガを無効化 → macOS のネイティブ挙動が前面に出て、
  # System Settings の配置通りにカーソルが移動するようになる。
  # 要 warp が必要になったら margin を 1+ に戻し axis を変える。
  mouseWarp = {
    axis = "horizontal";   # OmniWM が String? を要求するため値は維持（margin = 0 で無効化されるので影響なし）
    margin = 0;            # 0 で OmniWM 拡張 warp を無効化、macOS native に任せる
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
