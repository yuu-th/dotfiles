# ── OmniWM 用 Karabiner ルール ───────────────────────────────────────────────
# OmniWM ネイティブで実装できないキー操作のみ補う。
# YAGNI 原則で AeroSpace 由来の擬似機能（resize モード / alt-shift-d duplicate）は削除。
#
# 残しているもの:
#   - alt-s/c/a/ctrl-m            (シェル実行マクロ — OmniWM hotkey は exec 不可)
#   - alt-m/b/e, alt-shift-m/b/e  (名前指定 WS 操作 — OmniWM の hotkey は数値のみ)
#   - alt-ctrl-h/j/k/l            (方向ベース focus-monitor — OmniWM は prev/next のみ)
#   - cmd-h / cmd-alt-h           (macOS Hide ブロック — tile WM の保護)
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
    (basicShell "s" [ "left_option" ] "${wsLaunch}/bin/omniwm-ws-launch M Spotify"))
  (rules1 "OmniWM: alt-c = WS M + Discord"
    (basicShell "c" [ "left_option" ] "${wsLaunch}/bin/omniwm-ws-launch M Discord"))
  (rules1 "OmniWM: alt-a = WS M + Calendar"
    (basicShell "a" [ "left_option" ] "${wsLaunch}/bin/omniwm-ws-launch M Calendar"))

  # ── alt-ctrl-m: メディア WS 自動構築（簡略版）────────────────────────────
  (rules1 "OmniWM: alt-ctrl-m = setup media workspace"
    (basicShell "m" [ "left_option" "left_control" ]
      "${setupMedia}/bin/omniwm-setup-media-workspace"))

  # ── 名前指定 WS 切替 (alt-m/b/e) ─────────────────────────────────────────
  (rules1 "OmniWM: alt-m = workspace M"
    (basicShell "m" [ "left_option" ] (ctl "workspace focus-name M")))
  (rules1 "OmniWM: alt-b = workspace B"
    (basicShell "b" [ "left_option" ] (ctl "workspace focus-name B")))
  (rules1 "OmniWM: alt-e = workspace E"
    (basicShell "e" [ "left_option" ] (ctl "workspace focus-name E")))

  # ── 名前指定 WS への送り＋ジャンプ (alt-shift-m/b/e) ─────────────────────
  (rules1 "OmniWM: alt-shift-m = move window to WS M"
    (basicShell "m" [ "left_option" "left_shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws M"))
  (rules1 "OmniWM: alt-shift-b = move window to WS B"
    (basicShell "b" [ "left_option" "left_shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws B"))
  (rules1 "OmniWM: alt-shift-e = move window to WS E"
    (basicShell "e" [ "left_option" "left_shift" ]
      "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws E"))

  # ── 方向ベース focus-monitor (alt-ctrl-h/j/k/l) ─────────────────────────
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

  # ── macOS Hide ブロック ────────────────────────────────────────────────
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
