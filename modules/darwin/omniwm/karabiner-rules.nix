# ── OmniWM 用 Karabiner ルール ───────────────────────────────────────────────
# OmniWM ネイティブで実装できないキー操作のみ補う。
#
# IMPORTANT: ルール順序が重要。Karabiner は最初に match したルールしか発火しない。
# `optional = ["any"]` を使うと余分な modifier (shift 等) が許容されるので、
# **より specific (mandatory が多い) なルールを先に置く**。
#
# 例: Option+Shift+B (move window to B) は Option+B (focus B) より先に書く。
# でないと Option+Shift+B が押された時、optional=any のせいで Option+B が先 match
# してしまい focus 切替だけが走る（move window が発火しない）。
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
  # ──────────────────────────────────────────────────────────────────────────
  # 順序 1: 最も specific なルール（mandatory に多くの modifier を要求）
  # ──────────────────────────────────────────────────────────────────────────

  # ── alt-ctrl-m: メディア WS 自動構築 ──────────────────────────────────────
  (rules1 "OmniWM: alt-ctrl-m = setup media workspace"
    (basicShell "m" [ "option" "control" ]
      "${setupMedia}/bin/omniwm-setup-media-workspace"))

  # ── alt-ctrl-h/j/k/l: 方向ベース focus-monitor ────────────────────────
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

  # ── alt-shift-m/b/e: 名前指定 WS への送り＋ジャンプ ───────────────────
  # ★ 必ず alt-m/b/e (general) より先に配置。Karabiner の最先 match 仕様により、
  # より specific なこちらを優先発火させるため。
  (rules1 "OmniWM: alt-shift-m = move window to WS M"
    (basicShell "m" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws M"))
  (rules1 "OmniWM: alt-shift-b = move window to WS B"
    (basicShell "b" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws B"))
  (rules1 "OmniWM: alt-shift-e = move window to WS E"
    (basicShell "e" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws E"))

  # ──────────────────────────────────────────────────────────────────────────
  # 順序 2: 一般的なルール（mandatory が単独 modifier）
  # optional=any 許容なので、上で specific match されなかった時に発火する
  # ──────────────────────────────────────────────────────────────────────────

  # ── アプリ起動マクロ (alt-s/c/a) ─────────────────────────────────────────
  (rules1 "OmniWM: alt-s = WS M + Spotify"
    (basicShell "s" [ "option" ] "${wsLaunch}/bin/omniwm-ws-launch M Spotify"))
  (rules1 "OmniWM: alt-c = WS M + Discord"
    (basicShell "c" [ "option" ] "${wsLaunch}/bin/omniwm-ws-launch M Discord"))
  (rules1 "OmniWM: alt-a = WS M + Calendar"
    (basicShell "a" [ "option" ] "${wsLaunch}/bin/omniwm-ws-launch M Calendar"))

  # ── 名前指定 WS 切替 (alt-m/b/e) ─────────────────────────────────────────
  (rules1 "OmniWM: alt-m = workspace M"
    (basicShell "m" [ "option" ] (ctl "workspace focus-name M")))
  (rules1 "OmniWM: alt-b = workspace B"
    (basicShell "b" [ "option" ] (ctl "workspace focus-name B")))
  (rules1 "OmniWM: alt-e = workspace E"
    (basicShell "e" [ "option" ] (ctl "workspace focus-name E")))

  # ── Option+Space → OmniWM 内蔵 Quake terminal ───────────────────────────
  (rules1 "OmniWM: opt+space = toggle Quake terminal"
    (basicShell "spacebar" [ "option" ] (ctl "command toggle-quake-terminal")))

  # ──────────────────────────────────────────────────────────────────────────
  # 順序 3: macOS Hide ブロック（左 Cmd 限定、右 Cmd は通す）
  # ──────────────────────────────────────────────────────────────────────────
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
