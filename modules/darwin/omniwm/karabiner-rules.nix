# ── OmniWM 用 Karabiner ルール ───────────────────────────────────────────────
# OmniWM ネイティブで実装できないキー操作のみ補う。
#
# IMPORTANT: modifier は generic な `option`/`shift`/`control` を使う。
# `left_option` だと右 Option / RIGHT Shift がマッチしないので注意。
# `left_command` のみは macOS Hide ブロックに残す（右 Cmd は別キー扱いでもよいので）。
{ wsLaunch
, moveWindowToNamedWS
, setupMedia
, focusMonitorDir
, omniwmctl
}:
let
  basicShell = key: mods: shellCmd: {
    type = "basic";
    from = {
      key_code = key;
      modifiers = { mandatory = mods; optional = [ "any" ]; };
    };
    to = [ { shell_command = shellCmd; } ];
  };
  rule = description: manipulators: { inherit description manipulators; };
  rules1 = description: manipulator: rule description [ manipulator ];
  ctl = args: "${omniwmctl} ${args}";
in
[
  # ── アプリ起動マクロ (alt-s/c/a) ─────────────────────────────────────────
  (rules1 "OmniWM: alt-s = WS M + Spotify"
    (basicShell "s" [ "option" ] "${wsLaunch}/bin/omniwm-ws-launch M Spotify"))
  (rules1 "OmniWM: alt-c = WS M + Discord"
    (basicShell "c" [ "option" ] "${wsLaunch}/bin/omniwm-ws-launch M Discord"))
  (rules1 "OmniWM: alt-a = WS M + Calendar"
    (basicShell "a" [ "option" ] "${wsLaunch}/bin/omniwm-ws-launch M Calendar"))

  # ── alt-ctrl-m: メディア WS 自動構築 ──────────────────────────────────────
  (rules1 "OmniWM: alt-ctrl-m = setup media workspace"
    (basicShell "m" [ "option" "control" ]
      "${setupMedia}/bin/omniwm-setup-media-workspace"))

  # ── 名前指定 WS 切替 (alt-m/b/e) ─────────────────────────────────────────
  (rules1 "OmniWM: alt-m = workspace M"
    (basicShell "m" [ "option" ] (ctl "workspace focus-name M")))
  (rules1 "OmniWM: alt-b = workspace B"
    (basicShell "b" [ "option" ] (ctl "workspace focus-name B")))
  (rules1 "OmniWM: alt-e = workspace E"
    (basicShell "e" [ "option" ] (ctl "workspace focus-name E")))

  # ── 名前指定 WS への送り＋ジャンプ (alt-shift-m/b/e) ─────────────────────
  (rules1 "OmniWM: alt-shift-m = move window to WS M"
    (basicShell "m" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws M"))
  (rules1 "OmniWM: alt-shift-b = move window to WS B"
    (basicShell "b" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws B"))
  (rules1 "OmniWM: alt-shift-e = move window to WS E"
    (basicShell "e" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws E"))

  # ── 方向ベース focus-monitor (alt-ctrl-h/j/k/l) ─────────────────────────
  (rules1 "OmniWM: alt-ctrl-h = focus-monitor left"
    (basicShell "h" [ "option" "control" ]
      "${focusMonitorDir}/bin/omniwm-focus-monitor-dir left"))
  (rules1 "OmniWM: alt-ctrl-j = focus-monitor down"
    (basicShell "j" [ "option" "control" ]
      "${focusMonitorDir}/bin/omniwm-focus-monitor-dir down"))
  (rules1 "OmniWM: alt-ctrl-k = focus-monitor up"
    (basicShell "k" [ "option" "control" ]
      "${focusMonitorDir}/bin/omniwm-focus-monitor-dir up"))
  (rules1 "OmniWM: alt-ctrl-l = focus-monitor right"
    (basicShell "l" [ "option" "control" ]
      "${focusMonitorDir}/bin/omniwm-focus-monitor-dir right"))

  # ── Option+Space → OmniWM 内蔵 Quake terminal ───────────────────────────
  (rules1 "OmniWM: opt+space = toggle Quake terminal"
    (basicShell "spacebar" [ "option" ] (ctl "command toggle-quake-terminal")))

  # ── macOS Hide ブロック（左 Cmd のみ。右 Cmd は通す）────────────────────
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
        modifiers = { mandatory = [ "left_command" "option" ]; };
      };
      to = [ { key_code = "vk_none"; } ];
    })
]
