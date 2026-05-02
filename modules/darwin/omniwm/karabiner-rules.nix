# ── OmniWM 用 Karabiner ルール ───────────────────────────────────────────────
# OmniWM ネイティブで実装できないキー操作をここで補う。
# - シェル実行マクロ (alt-s/c/a/ctrl-m)
# - 名前指定 WS 切替/送り (alt-m/b/e, alt-shift-m/b/e)
# - 方向ベース focus-monitor (alt-ctrl-h/j/k/l)
# - resize モード (alt-r → set_variable → h/j/k/l 再バインド)
# - macOS Hide ブロック (cmd-h, cmd-alt-h)
{ wsLaunch
, moveWindowToNamedWS
, setupMedia
, focusMonitorDir
, omniwmctl
}:
let
  # ── 1 manipulator を作るユーティリティ ───────────────────────────────────
  basicShell = key: mods: shellCmd: {
    type = "basic";
    from = {
      key_code = key;
      modifiers = { mandatory = mods; optional = [ "any" ]; };
    };
    to = [ { shell_command = shellCmd; } ];
  };

  basicVar = key: mods: var: value: {
    type = "basic";
    from = {
      key_code = key;
      modifiers = { mandatory = mods; optional = [ "any" ]; };
    };
    to = [ { set_variable = { name = var; inherit value; }; } ];
  };

  # 変数条件付きの shell ルール
  shellWhenVar = key: var: value: shellCmd: {
    type = "basic";
    from = {
      key_code = key;
      modifiers = { optional = [ "any" ]; };
    };
    to = [ { shell_command = shellCmd; } ];
    conditions = [ { type = "variable_if"; name = var; inherit value; } ];
  };

  # ルールラッパ
  rule = description: manipulators: { inherit description manipulators; };
  rules1 = description: manipulator: rule description [ manipulator ];

  # OmniWM ctl 引数を埋め込んだ shell コマンド
  ctl  = args: "${omniwmctl} ${args}";
in
[
  # ── alt-s/c/a: WS M に行ってアプリ起動 ───────────────────────────────────
  (rules1 "OmniWM: alt-s = WS M + Spotify"
    (basicShell "s" [ "left_option" ] "${wsLaunch}/bin/omniwm-ws-launch M Spotify"))
  (rules1 "OmniWM: alt-c = WS M + Discord"
    (basicShell "c" [ "left_option" ] "${wsLaunch}/bin/omniwm-ws-launch M Discord"))
  (rules1 "OmniWM: alt-a = WS M + Calendar"
    (basicShell "a" [ "left_option" ] "${wsLaunch}/bin/omniwm-ws-launch M Calendar"))

  # ── alt-ctrl-m: メディアレイアウト自動構築 ───────────────────────────────
  (rules1 "OmniWM: alt-ctrl-m = setup media workspace"
    (basicShell "m" [ "left_option" "left_control" ]
      "${setupMedia}/bin/omniwm-setup-media-workspace"))

  # ── alt-m/b/e: 名前指定 WS 切替 ──────────────────────────────────────────
  (rules1 "OmniWM: alt-m = workspace M"
    (basicShell "m" [ "left_option" ] (ctl "workspace focus-name M")))
  (rules1 "OmniWM: alt-b = workspace B"
    (basicShell "b" [ "left_option" ] (ctl "workspace focus-name B")))
  (rules1 "OmniWM: alt-e = workspace E"
    (basicShell "e" [ "left_option" ] (ctl "workspace focus-name E")))

  # ── alt-shift-m/b/e: 名前指定 WS への送り＋ジャンプ ──────────────────────
  (rules1 "OmniWM: alt-shift-m = move window to WS M"
    (basicShell "m" [ "left_option" "left_shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws M"))
  (rules1 "OmniWM: alt-shift-b = move window to WS B"
    (basicShell "b" [ "left_option" "left_shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws B"))
  (rules1 "OmniWM: alt-shift-e = move window to WS E"
    (basicShell "e" [ "left_option" "left_shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws E"))

  # ── alt-ctrl-h/j/k/l: 方向ベース focus-monitor ──────────────────────────
  (rules1 "OmniWM: alt-ctrl-h = focus-monitor left"
    (basicShell "h" [ "left_option" "left_control" ]
      "${focusMonitorDir}/bin/omniwm-focus-monitor-dir left"))
  (rules1 "OmniWM: alt-ctrl-j = focus-monitor down"
    (basicShell "j" [ "left_option" "left_control" ]
      "${focusMonitorDir}/bin/omniwm-focus-monitor-dir down"))
  (rules1 "OmniWM: alt-ctrl-k = focus-monitor up"
    (basicShell "k" [ "left_option" "left_control" ]
      "${focusMonitorDir}/bin/omniwm-focus-monitor-dir up"))
  (rules1 "OmniWM: alt-ctrl-l = focus-monitor right"
    (basicShell "l" [ "left_option" "left_control" ]
      "${focusMonitorDir}/bin/omniwm-focus-monitor-dir right"))

  # ── alt-r: resize モード入口（set_variable）+ esc/enter で抜ける ────────
  (rule "OmniWM: resize mode (alt-r → h/j/k/l)" [
    # 入口 alt-r → omniwm_resize = 1
    (basicVar "r" [ "left_option" ] "omniwm_resize" 1)
    # モード中の h/j/k/l: column width 巡回 / カラム移動
    (shellWhenVar "h" "omniwm_resize" 1
      (ctl "command cycle-column-width backward"))
    (shellWhenVar "l" "omniwm_resize" 1
      (ctl "command cycle-column-width forward"))
    (shellWhenVar "j" "omniwm_resize" 1
      (ctl "command balance-sizes"))
    (shellWhenVar "k" "omniwm_resize" 1
      (ctl "command toggle-column-full-width"))
    # 退出: enter / esc → omniwm_resize = 0
    {
      type = "basic";
      from = { key_code = "return_or_enter"; modifiers = { optional = [ "any" ]; }; };
      to = [ { set_variable = { name = "omniwm_resize"; value = 0; }; } ];
      conditions = [ { type = "variable_if"; name = "omniwm_resize"; value = 1; } ];
    }
    {
      type = "basic";
      from = { key_code = "escape"; modifiers = { optional = [ "any" ]; }; };
      to = [ { set_variable = { name = "omniwm_resize"; value = 0; }; } ];
      conditions = [ { type = "variable_if"; name = "omniwm_resize"; value = 1; } ];
    }
  ])

  # ── alt-shift-d: toggle-focused-window-floating (service モードからの救出) ─
  (rules1 "OmniWM: alt-shift-d = toggle floating"
    (basicShell "d" [ "left_option" "left_shift" ]
      (ctl "command toggle-focused-window-floating")))

  # ── cmd-h / cmd-alt-h: macOS Hide ブロック ───────────────────────────────
  # AeroSpace の `cmd-h = []` 相当。Hide してウィンドウが消えるのを防ぐ。
  (rules1 "OmniWM: block cmd-h (Hide)"
    {
      type = "basic";
      from = {
        key_code = "h";
        modifiers = { mandatory = [ "left_command" ]; };
      };
      to = [ { key_code = "vk_none"; } ];
    })
  (rules1 "OmniWM: block cmd-alt-h (Hide Others)"
    {
      type = "basic";
      from = {
        key_code = "h";
        modifiers = { mandatory = [ "left_command" "left_option" ]; };
      };
      to = [ { key_code = "vk_none"; } ];
    })
]
