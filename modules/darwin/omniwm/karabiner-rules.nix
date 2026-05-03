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

  # ── アプリ起動マクロ (alt-s/c) ───────────────────────────────────────────
  # ※ alt-a は projwm (queue/projwm-design.md §8.1) で AI Viewer (WS A) jump に
  # 振替。Calendar は手動 / Spotlight / Dock 起動を使う運用に変更。
  (rules1 "OmniWM: alt-s = WS M + Spotify"
    (basicShell "s" [ "option" ] "${wsLaunch}/bin/omniwm-ws-launch M Spotify"))
  (rules1 "OmniWM: alt-c = WS M + Discord"
    (basicShell "c" [ "option" ] "${wsLaunch}/bin/omniwm-ws-launch M Discord"))

  # ── projwm: alt-shift-letter で slot へ送る（specific 順序、alt-letter より先）─
  # AI viewer (A) と AI project slots Q/W/R/T/Y/U/I/O/P。E は既存の alt-shift-e
  # が同じ動作（move to WS E）なので兼用、新規追加は不要。
  (rules1 "projwm: alt-shift-a = move window to WS A (viewer)"
    (basicShell "a" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws A"))
  (rules1 "projwm: alt-shift-q = move window to WS Q"
    (basicShell "q" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws Q"))
  (rules1 "projwm: alt-shift-w = move window to WS W"
    (basicShell "w" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws W"))
  (rules1 "projwm: alt-shift-r = move window to WS R"
    (basicShell "r" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws R"))
  (rules1 "projwm: alt-shift-t = move window to WS T"
    (basicShell "t" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws T"))
  (rules1 "projwm: alt-shift-y = move window to WS Y"
    (basicShell "y" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws Y"))
  (rules1 "projwm: alt-shift-u = move window to WS U"
    (basicShell "u" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws U"))
  (rules1 "projwm: alt-shift-i = move window to WS I"
    (basicShell "i" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws I"))
  (rules1 "projwm: alt-shift-o = move window to WS O"
    (basicShell "o" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws O"))
  (rules1 "projwm: alt-shift-p = move window to WS P"
    (basicShell "p" [ "option" "shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws P"))

  # ── 名前指定 WS 切替 (alt-m/b/e) ─────────────────────────────────────────
  (rules1 "OmniWM: alt-m = workspace M"
    (basicShell "m" [ "option" ] (ctl "workspace focus-name M")))
  (rules1 "OmniWM: alt-b = workspace B"
    (basicShell "b" [ "option" ] (ctl "workspace focus-name B")))
  (rules1 "OmniWM: alt-e = workspace E"
    (basicShell "e" [ "option" ] (ctl "workspace focus-name E")))

  # ── projwm: alt-letter で slot に jump（A + Q/W/R/T/Y/U/I/O/P）─────────
  # E は既に alt-e で focus-name E が貼られているので兼用。
  (rules1 "projwm: alt-a = workspace A (viewer)"
    (basicShell "a" [ "option" ] (ctl "workspace focus-name A")))
  (rules1 "projwm: alt-q = workspace Q"
    (basicShell "q" [ "option" ] (ctl "workspace focus-name Q")))
  (rules1 "projwm: alt-w = workspace W"
    (basicShell "w" [ "option" ] (ctl "workspace focus-name W")))
  (rules1 "projwm: alt-r = workspace R"
    (basicShell "r" [ "option" ] (ctl "workspace focus-name R")))
  (rules1 "projwm: alt-t = workspace T"
    (basicShell "t" [ "option" ] (ctl "workspace focus-name T")))
  (rules1 "projwm: alt-y = workspace Y"
    (basicShell "y" [ "option" ] (ctl "workspace focus-name Y")))
  (rules1 "projwm: alt-u = workspace U"
    (basicShell "u" [ "option" ] (ctl "workspace focus-name U")))
  (rules1 "projwm: alt-i = workspace I"
    (basicShell "i" [ "option" ] (ctl "workspace focus-name I")))
  (rules1 "projwm: alt-o = workspace O"
    (basicShell "o" [ "option" ] (ctl "workspace focus-name O")))
  (rules1 "projwm: alt-p = workspace P"
    (basicShell "p" [ "option" ] (ctl "workspace focus-name P")))

  # ── Option+Space → OmniWM 内蔵 Quake terminal ───────────────────────────
  (rules1 "OmniWM: opt+space = toggle Quake terminal"
    (basicShell "spacebar" [ "option" ] (ctl "command toggle-quake-terminal")))

  # ── Option+` → projwm cockpit（kitty 上で projwm tui を spawn）──────────
  # 設計書 §8.3 の「cockpit を別キーで」を実現。kitty-projwm.app（OmniWM 互換の
  # user-space 派生）で projwm tui の専用 window を立てる。title="projwm-cockpit"
  # を持つので app-rules.nix の matcher で float 配置に切替えられる（拡張余地）。
  (rules1 "projwm: opt+` = open projwm cockpit (kitty + projwm tui)"
    (basicShell "grave_accent_and_tilde" [ "option" ]
      "/usr/bin/open -na ~/Applications/kitty-projwm.app --args -T projwm-cockpit -d ~ ${"\${"}HOME${"}"}/.nix-profile/bin/projwm tui"))

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
