# ── OmniWM 共通設定（OmniWM 0.5.9 / IPC protocol 8）──────────────────────────
#
# モニタ構成に依存しないベース設定。monitor-profiles とマージされる。
#
# スキーマの正典は `Sources/OmniWM/Core/Config/CanonicalTOMLConfig.swift`。
# 0.4.x 時代の `canonical-settings.toml` は upstream から消えている。
#
# ── 欠損キーについて ──
# `SettingsTOMLCodec.decode` は strict デコードを試し、`keyNotFound` を捕まえたら
# recovering モードで再デコードして欠損キーに default を入れる。よって
# 「使わないセクションは書かない」で安全。ただし **値の形式違反**（dataCorrupted /
# typeMismatch）は回復されず settings.toml.corrupt に飛ぶので、enum の文字列や
# UUID の形式は厳密に正しくないといけない。
#
# ── 書かないもの（意図的）──
#   [dwindle] / monitorDwindleOverrides … Dwindle レイアウトは廃止
#   [overview]                          … upstream default に任せる
#   version                             … スキーマに存在しない未知キー
#   [state]                             … runtime state 側へ移行済み
#   [routing] / monitorRoutingOverrides … monitor-profiles が宣言する
#   workspaceBar.notchMode 等           … この機体は notch を持たない（hasNotch=false）
{ ... }:
{
  # ── モニタ別オーバーライド（未使用、空リストで明示）──────────────────────
  # routing だけはプロファイルが宣言するのでここには書かない。
  monitorBarOverrides = [ ];
  monitorNiriOverrides = [ ];
  monitorOrientationOverrides = [ ];
  monitorGapOverrides = [ ];

  # ── 外観 ──────────────────────────────────────────────────────────────────
  appearance = { mode = "dark"; };

  # ── 内蔵ボーダー（JankyBorders を置換）──────────────────────────────────
  # 旧 AeroSpace の borders 設定: active = 0xFFE8D44D (#E8D44D), width = 3
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

  # ── クリップボード履歴 ────────────────────────────────────────────────────
  # 呼び出し口は Command Palette の 3 つ目のモード（windows / menu / clipboard）で、
  # 入口は space+t に割り当ててある。専用の hotkey action は存在しない。
  # 履歴の実体は ~/.local/state/omniwm 配下に保存される。
  # 1 項目 1MB 上限にして巨大な画像等を溜め込まないようにする。
  clipboard = {
    historyEnabled = true;
    maxItems       = 50;
    maxItemBytes   = 1048576;
    maxTotalBytes  = 67108864;
  };

  # ── フォーカス挙動 ────────────────────────────────────────────────────────
  # followsMouse は以前「マウス操作中にチラつき/誤動作が出やすい」として false に
  # していたが、0.5.2 の Focus Lock と 0.5.5 の floating focus 改善を前提に再挑戦する。
  # 合わなければ followsMouse = false に戻すだけでよい（lockModifier は無害になる）。
  #
  # crossesMonitorAtEdge を false にしている理由:
  #   niri の幅を 100% にしているので 1 画面に 1 窓しか見えず、窓が 1 つだけの WS では
  #   space+h/l が即座に「端」に達して隣モニタへ飛んでしまう。focus は WS 内で完結させ、
  #   モニタ間の移動は space+ctrl+tab / space+ctrl+shift+tab とマウスで行う。
  # moveCrossesMonitorAtEdge は意図的な操作（space+shift+h/l）でしか起きないので有効。
  focus = {
    followsMouse             = true;
    lockModifier             = "leftCommand";  # 左Cmd 保持中は follows-mouse を一時停止
    moveMouseToFocusedWindow = true;
    followsWindowToMonitor   = true;
    crossesMonitorAtEdge     = false;
    moveCrossesMonitorAtEdge = true;
  };

  # ── Gaps ──────────────────────────────────────────────────────────────────
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
    # native hotkey は全部 Unassigned にするが、機構自体は有効にしておく。
    # こうしておくと Karabiner が死んでも緊急脱出用の 2 キー（hotkeys.nix 参照）が効く。
    hotkeysEnabled    = true;
    ipcEnabled        = true;                  # omniwmctl / space-leader の生命線
    preventSleepEnabled = false;
    # brew cask で管理しているので OmniWM 自身の更新ポップアップは切る。
    updateChecksEnabled = false;
    # Karabiner の space-leader を使うので OmniWM 側の Hyper 化は使わない。
    systemHyperTrigger  = "None";
  };

  # ── トラックパッド / マウスジェスチャ ─────────────────────────────────────
  # 列スクロールは 3 本指（横）。WS スワイプは 4 本指（横）で軸ごと分離する。
  # 同じ指本数にすると upstream 仕様で軸が縦に強制されるため、あえて分ける。
  gestures = {
    scrollEnabled          = true;
    scrollSensitivity      = 5.0;
    scrollModifierKey      = "optionShift";
    mouseResizeModifierKey = "option";
    fingerCount            = 3;
    invertDirection        = true;
    trackpadScrollStyle    = "momentum";   # snap → momentum（niri 流の慣性スクロール）
    workspaceSwipeEnabled     = true;
    workspaceSwipeFingerCount = 4;
    workspaceSwipeAxis        = "horizontal";
  };

  # ── Hidden Bar ───────────────────────────────────────────────────────────
  # メニューバーアイコンの隠蔽は macOS 27 以降が必要で、この機体（26.5.2）では
  # 動作しない。OmniWM が書き戻す既定が true なので、意図を明示して false で固定する。
  hiddenBar = {
    enabled              = false;
    hiddenBundleIDs      = [ ];
    rehideIntervalSeconds = 5.0;
  };

  # ── マウス warp（モニタ間） ──────────────────────────────────────────────
  # 0.5.0 で axis / monitorOrder は削除され、routing map に従う単一トグルになった。
  # 実際の机の配置は monitor-profiles の monitorRoutingOverrides が宣言する。
  mouseWarp = {
    enabled                = true;
    margin                 = 5;
    constrainToArrangement = false;
  };

  # ── Niri (scrolling containers) ───────────────────────────────────────────
  # 「1 画面 1 窓、横スクロールで切り替える」構成。
  #   defaultContainerPrimarySpan : 新規窓の幅（画面幅に対する比）。未指定だと
  #                                 1.0 / visibleContainerCount にフォールバックする。
  #   visibleContainerCount       : 同時に見せるコンテナ数。幅 100% と整合させて 1。
  #   containerPrimarySpanPresets : space+, / space+. で巡回する幅。1.0 を含めて
  #                                 縮めたあと 100% に戻れるようにする。
  #   singleWindowFit             : "fill" | "container_primary_span" | "<W>x<H>"
  #   centerFocusedColumn         : 左寄せ packing を維持するため "never"
  niri = {
    defaultContainerPrimarySpan = 1.0;
    visibleContainerCount       = 1;
    containerPrimarySpanPresets = [ 0.3333333333333333 0.5 0.6666666666666666 1.0 ];
    singleWindowFit             = "fill";
    alwaysCenterSingleColumn    = true;
    centerFocusedColumn         = "never";
    infiniteLoop                = false;
  };

  # ── Quake terminal（OmniWM 内蔵 libghostty）──────────────────────────────
  # 以前「上端が画面外にはみ出る」問題で廃止したが、0.5.4 で配置の堅牢性
  # （モニタ変更・オフセットモニタ・保存済みカスタム位置）が修正されたので再挑戦する。
  # 呼び出しは space+g。背景効果（backgroundEffect）は 0.5.9 には未搭載。
  quakeTerminal = {
    enabled           = true;
    position          = "center";
    widthPercent      = 90.0;
    heightPercent     = 85.0;
    animationDuration = 0.2;
    autoHide          = false;
    opacity           = 1.0;
    monitorMode       = "focusedWindow";
  };

  # ── ステータスバー ────────────────────────────────────────────────────────
  statusBar = {
    showAppNames = true;
    showWorkspaceName = true;
    useWorkspaceId = false;
  };

  # ── ワークスペースバー ────────────────────────────────────────────────────
  # **常時表示**（revealModifier = "off"）。
  # 一度 Cmd 保持で出す方式（revealModifier = "command"）にしたが、常に見えている方が
  # 「今どの WS にいて何が開いているか」が分かるので戻した。
  # 常時表示なので systemStatsButton もそのままクリックできる。
  #
  # floating も表示する（見失った窓をバーからクリックして呼び戻せる救済手段になる）。
  # ただし呼んで消える系だけは除外してノイズを減らす。
  workspaceBar = {
    enabled                = true;
    showLabels             = true;
    showFloatingWindows    = true;
    windowLevel            = "popup";
    position               = "overlappingMenuBar";
    systemStatsButton      = true;
    deduplicateAppIcons    = false;
    hideEmptyWorkspaces    = true;
    excludedBundleIDs = [
      "com.raycast.macos"
      "com.1password.1password"
      "com.agilebits.onepassword7"
      "com.knollsoft.Hookshot"
      "com.apple.calculator"
      "com.apple.Dictionary"
    ];
    reserveLayoutSpace     = false;
    # "off" = 常時表示。modifier 押下中だけ出したくなったら "option" / "command" 等に。
    # （revealHoldMilliseconds は off の間は無効なので書かない）
    revealModifier         = "off";
    height                 = 24.0;
    backgroundOpacity      = 0.1;
    # バーは各モニタの中央基準で置かれ、xOffset が macOS 座標（右が正）に加算される。
    #   WorkspaceBarGeometry.frame: x = monitor.frame.midX - width/2 + xOffset
    # → **負の値で左にずれる**。メニューバーの項目との重なりを避けるため少し左へ。
    # 0.5.0 で ±500px の上限は撤廃済み。
    xOffset                = -60.0;
    yOffset                = 0.0;
    # -1 は「OmniWM の既定色を使う」を意味する番兵値。
    accentColor = { red = -1.0; green = -1.0; blue = -1.0; alpha = 1.0; };
    textColor   = { red = -1.0; green = -1.0; blue = -1.0; alpha = 1.0; };
  };
}
