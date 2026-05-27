# ── projwm / omniwm Karabiner ルール ───────────────────────────────────────
#
# 設計準拠: queue/projwm-cockpit-requirements.md §11.1 (v2.5)
#   "Hyper modifier — space を karabiner variable で普通の modifier 化"
#
# 仕組み (v2.5、time-limit 無し):
#   1. spaceLeader: bare spacebar 押下で variable `space_held=1` を立てる。
#      OS には何も流さない。release 時に `to_if_alone` が発火して通常 space
#      を OS に届ける (押し→離しの自分の打鍵時間が遅延)。途中で他キーが
#      押されると `to_if_alone` はキャンセル、`to_after_key_up` で
#      `space_held=0` に戻る。
#      → 「普通の modifier (shift/ctrl と同じ) のように扱える」: 長押し中
#        ずっと variable=1、letter を押すまでの時間制限なし。
#      → modifier 押下中の space (ctrl+space / shift+space) は spaceLeader
#        の optional=[] により **マッチしない** → karabiner は無干渉で
#        ctrl+space などをそのまま OS へ流す。race ゼロ。
#   2. spaceBind: 各 letter を `space_held=1` の variable_if 条件付きで定義。
#      space hold 中の letter 押下が即 shell_command 発火。time threshold 無し。
#   3. spaceShiftBind / spaceCtrlBind: mandatory modifier 追加版。
#      space + shift + letter / space + ctrl + letter 等。
#
# v2.4 (simultaneous strict) からの変更理由:
#   v2.4 は letter→space ロール打鍵保護のため simultaneous を採用したが、
#   simultaneous_threshold (~80ms) がそのまま time limit となり:
#     - 単独 space tap が threshold 経過まで emit されない遅延感
#     - space → letter combo に厳格な時間制限 (押下が遅れると combo 不発)
#     - ctrl+space で spaceCtrlCombo (simultaneous) が partial match buffer →
#       letter が来ないと ctrl release race で ctrl 抜き space 発生
#   実機検証で全て体感ストレスと確認、v2.5 で variable_if + to_if_alone に
#   再回帰。letter-first ロール打鍵保護は variable_if の論理上自然に保証
#   (letter 単独押下時は variable=0 なので何もマッチしない)。
#
# cmd+space / opt+space は spaceLeader の `optional` から除外 → 他ルール
# (raycast.nix の cmd+space → opt+space 等) を妨げない。
{ wsLaunch
, moveWindowToNamedWS
, setupMedia
, focusMonitorDir
, omniwmctl
, projwmCli
}:
let
  ctl = args: "${omniwmctl} ${args}";
  moveTo = ws: "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws ${ws}";

  # spaceLeader: bare spacebar (no modifier) → 変数 space_held=1。
  # `optional = []` で modifier 押下中の space は match せず素通し。
  # `to_if_alone_timeout_milliseconds = 5000` で 5 秒以内のタップなら
  # release 時に space emit (長時間 hold は modifier 専用扱い)。
  spaceLeader = {
    description = "Hyper modifier: space → space_held variable (§11.1 v2.5)";
    manipulators = [{
      type = "basic";
      from = {
        key_code = "spacebar";
        modifiers = { optional = [ ]; };
      };
      parameters = { "basic.to_if_alone_timeout_milliseconds" = 5000; };
      to = [{ set_variable = { name = "space_held"; value = 1; }; }];
      to_after_key_up = [{ set_variable = { name = "space_held"; value = 0; }; }];
      to_if_alone = [{ key_code = "spacebar"; }];
    }];
  };

  # variable_if space_held=1 のもとで letter (+ optional modifier) を
  # shell_command にすり替える。modifier mandatory リストで shift / ctrl
  # 系を区別。
  spaceBind = description: key: mandatoryMods: shellCmd: {
    inherit description;
    manipulators = [{
      type = "basic";
      conditions = [{ name = "space_held"; type = "variable_if"; value = 1; }];
      from = {
        key_code = key;
        modifiers = { mandatory = mandatoryMods; optional = [ ]; };
      };
      to = [{ shell_command = shellCmd; }];
    }];
  };
in
[
  # ── space leader root manipulator ────────────────────────────────────
  spaceLeader

  # ── projwm slot workspaces (§11.3) Q-P + A ───────────────────────────
  (spaceBind "space+q → workspace Q" "q" [ ] (ctl "workspace focus-name Q"))
  (spaceBind "space+w → workspace W" "w" [ ] (ctl "workspace focus-name W"))
  (spaceBind "space+e → workspace E" "e" [ ] (ctl "workspace focus-name E"))
  (spaceBind "space+r → workspace R" "r" [ ] (ctl "workspace focus-name R"))
  (spaceBind "space+t → workspace T" "t" [ ] (ctl "workspace focus-name T"))
  (spaceBind "space+y → workspace Y" "y" [ ] (ctl "workspace focus-name Y"))
  (spaceBind "space+u → workspace U" "u" [ ] (ctl "workspace focus-name U"))
  (spaceBind "space+i → workspace I" "i" [ ] (ctl "workspace focus-name I"))
  (spaceBind "space+o → workspace O" "o" [ ] (ctl "workspace focus-name O"))
  (spaceBind "space+p → workspace P" "p" [ ] (ctl "workspace focus-name P"))
  (spaceBind "space+a → workspace A" "a" [ ] (ctl "workspace focus-name A"))

  # ── window move (space+shift+letter) §11.3 ───────────────────────────
  (spaceBind "space+shift+q → move to Q" "q" [ "shift" ] (moveTo "Q"))
  (spaceBind "space+shift+w → move to W" "w" [ "shift" ] (moveTo "W"))
  (spaceBind "space+shift+e → move to E" "e" [ "shift" ] (moveTo "E"))
  (spaceBind "space+shift+r → move to R" "r" [ "shift" ] (moveTo "R"))
  (spaceBind "space+shift+t → move to T" "t" [ "shift" ] (moveTo "T"))
  (spaceBind "space+shift+y → move to Y" "y" [ "shift" ] (moveTo "Y"))
  (spaceBind "space+shift+u → move to U" "u" [ "shift" ] (moveTo "U"))
  (spaceBind "space+shift+i → move to I" "i" [ "shift" ] (moveTo "I"))
  (spaceBind "space+shift+o → move to O" "o" [ "shift" ] (moveTo "O"))
  (spaceBind "space+shift+p → move to P" "p" [ "shift" ] (moveTo "P"))
  (spaceBind "space+shift+a → move to A" "a" [ "shift" ] (moveTo "A"))

  # ── 一般 workspace M/B (§11.3) ───────────────────────────────────────
  (spaceBind "space+m → workspace M" "m" [ ] (ctl "workspace focus-name M"))
  (spaceBind "space+b → workspace B" "b" [ ] (ctl "workspace focus-name B"))
  (spaceBind "space+shift+m → move to M" "m" [ "shift" ] (moveTo "M"))
  (spaceBind "space+shift+b → move to B" "b" [ "shift" ] (moveTo "B"))

  # ── 数字 workspace 1-9 (§11.3) ───────────────────────────────────────
  (spaceBind "space+1 → workspace 1" "1" [ ] (ctl "workspace focus-name 1"))
  (spaceBind "space+2 → workspace 2" "2" [ ] (ctl "workspace focus-name 2"))
  (spaceBind "space+3 → workspace 3" "3" [ ] (ctl "workspace focus-name 3"))
  (spaceBind "space+4 → workspace 4" "4" [ ] (ctl "workspace focus-name 4"))
  (spaceBind "space+5 → workspace 5" "5" [ ] (ctl "workspace focus-name 5"))
  (spaceBind "space+6 → workspace 6" "6" [ ] (ctl "workspace focus-name 6"))
  (spaceBind "space+7 → workspace 7" "7" [ ] (ctl "workspace focus-name 7"))
  (spaceBind "space+8 → workspace 8" "8" [ ] (ctl "workspace focus-name 8"))
  (spaceBind "space+9 → workspace 9" "9" [ ] (ctl "workspace focus-name 9"))
  (spaceBind "space+shift+1 → move to 1" "1" [ "shift" ] (moveTo "1"))
  (spaceBind "space+shift+2 → move to 2" "2" [ "shift" ] (moveTo "2"))
  (spaceBind "space+shift+3 → move to 3" "3" [ "shift" ] (moveTo "3"))
  (spaceBind "space+shift+4 → move to 4" "4" [ "shift" ] (moveTo "4"))
  (spaceBind "space+shift+5 → move to 5" "5" [ "shift" ] (moveTo "5"))
  (spaceBind "space+shift+6 → move to 6" "6" [ "shift" ] (moveTo "6"))
  (spaceBind "space+shift+7 → move to 7" "7" [ "shift" ] (moveTo "7"))
  (spaceBind "space+shift+8 → move to 8" "8" [ "shift" ] (moveTo "8"))
  (spaceBind "space+shift+9 → move to 9" "9" [ "shift" ] (moveTo "9"))

  # ── cockpit show/hide (§11.3) ────────────────────────────────────────
  (spaceBind "space+f → projwm cockpit toggle" "f" [ ]
    "${projwmCli}/bin/projwm cockpit toggle")

  # ── 旧 opt+ctrl 系 → space+ctrl §11.6 ────────────────────────────────
  (spaceBind "space+ctrl+m → setup media workspace" "m" [ "control" ]
    "${setupMedia}/bin/omniwm-setup-media-workspace")
  (spaceBind "space+ctrl+h → focus-monitor left" "h" [ "control" ]
    "${focusMonitorDir}/bin/omniwm-focus-monitor-dir left")
  (spaceBind "space+ctrl+j → focus-monitor down" "j" [ "control" ]
    "${focusMonitorDir}/bin/omniwm-focus-monitor-dir down")
  (spaceBind "space+ctrl+k → focus-monitor up" "k" [ "control" ]
    "${focusMonitorDir}/bin/omniwm-focus-monitor-dir up")
  (spaceBind "space+ctrl+l → focus-monitor right" "l" [ "control" ]
    "${focusMonitorDir}/bin/omniwm-focus-monitor-dir right")

  # ── space+letter → app launch ────────────────────────────────────────
  (spaceBind "space+s → ws-launch Spotify" "s" [ ]
    "${wsLaunch}/bin/omniwm-ws-launch M Spotify")
  (spaceBind "space+c → ws-launch Discord" "c" [ ]
    "${wsLaunch}/bin/omniwm-ws-launch M Discord")

  # ── Focus 方向 hjkl + ; (新規 §11.3 v2.6) ───────────────────────────
  (spaceBind "space+h → focus left"  "h" [ ] (ctl "command focus left"))
  (spaceBind "space+j → focus down"  "j" [ ] (ctl "command focus down"))
  (spaceBind "space+k → focus up"    "k" [ ] (ctl "command focus up"))
  (spaceBind "space+l → focus right" "l" [ ] (ctl "command focus right"))
  (spaceBind "space+; → focus previous" "semicolon" [ ] (ctl "command focus previous"))

  # ── Workspace ナビゲーション next / prev / back-and-forth (新規) ──────
  (spaceBind "space+] → switch-workspace next" "close_bracket" [ ] (ctl "command switch-workspace next"))
  (spaceBind "space+[ → switch-workspace prev" "open_bracket"  [ ] (ctl "command switch-workspace prev"))
  (spaceBind "space+tab → switch-workspace back-and-forth" "tab" [ ] (ctl "command switch-workspace back-and-forth"))

  # ── Window → workspace up / down (space+shift+] / [) ─────────────────
  (spaceBind "space+shift+] → move-to-workspace down" "close_bracket" [ "shift" ] (ctl "command move-to-workspace down"))
  (spaceBind "space+shift+[ → move-to-workspace up"   "open_bracket"  [ "shift" ] (ctl "command move-to-workspace up"))

  # ── Monitor focus next (space+ctrl+tab) ──────────────────────────────
  (spaceBind "space+ctrl+tab → focus-monitor next" "tab" [ "control" ] (ctl "command focus-monitor next"))

  # ── Focus column N (space+ctrl+1..9, 0-based) ─────────────────────────
  (spaceBind "space+ctrl+1 → focus-column 0" "1" [ "control" ] (ctl "command focus-column 0"))
  (spaceBind "space+ctrl+2 → focus-column 1" "2" [ "control" ] (ctl "command focus-column 1"))
  (spaceBind "space+ctrl+3 → focus-column 2" "3" [ "control" ] (ctl "command focus-column 2"))
  (spaceBind "space+ctrl+4 → focus-column 3" "4" [ "control" ] (ctl "command focus-column 3"))
  (spaceBind "space+ctrl+5 → focus-column 4" "5" [ "control" ] (ctl "command focus-column 4"))
  (spaceBind "space+ctrl+6 → focus-column 5" "6" [ "control" ] (ctl "command focus-column 5"))
  (spaceBind "space+ctrl+7 → focus-column 6" "7" [ "control" ] (ctl "command focus-column 6"))
  (spaceBind "space+ctrl+8 → focus-column 7" "8" [ "control" ] (ctl "command focus-column 7"))
  (spaceBind "space+ctrl+9 → focus-column 8" "9" [ "control" ] (ctl "command focus-column 8"))
  (spaceBind "space+ctrl+[ → focus-column first" "open_bracket"  [ "control" ] (ctl "command focus-column first"))
  (spaceBind "space+ctrl+] → focus-column last"  "close_bracket" [ "control" ] (ctl "command focus-column last"))

  # ── Window 方向 move (space+shift+hjkl) ──────────────────────────────
  (spaceBind "space+shift+h → move left"  "h" [ "shift" ] (ctl "command move left"))
  (spaceBind "space+shift+j → move down"  "j" [ "shift" ] (ctl "command move down"))
  (spaceBind "space+shift+k → move up"    "k" [ "shift" ] (ctl "command move up"))
  (spaceBind "space+shift+l → move right" "l" [ "shift" ] (ctl "command move right"))

  # ── Column 方向 move (space+ctrl+shift+h / l) ─────────────────────────
  (spaceBind "space+ctrl+shift+h → move-column left"  "h" [ "control" "shift" ] (ctl "command move-column left"))
  (spaceBind "space+ctrl+shift+l → move-column right" "l" [ "control" "shift" ] (ctl "command move-column right"))

  # ── Column → workspace N (space+ctrl+shift+1..9, 0-based) ────────────
  (spaceBind "space+ctrl+shift+1 → move-column-to-workspace 0" "1" [ "control" "shift" ] (ctl "command move-column-to-workspace 0"))
  (spaceBind "space+ctrl+shift+2 → move-column-to-workspace 1" "2" [ "control" "shift" ] (ctl "command move-column-to-workspace 1"))
  (spaceBind "space+ctrl+shift+3 → move-column-to-workspace 2" "3" [ "control" "shift" ] (ctl "command move-column-to-workspace 2"))
  (spaceBind "space+ctrl+shift+4 → move-column-to-workspace 3" "4" [ "control" "shift" ] (ctl "command move-column-to-workspace 3"))
  (spaceBind "space+ctrl+shift+5 → move-column-to-workspace 4" "5" [ "control" "shift" ] (ctl "command move-column-to-workspace 4"))
  (spaceBind "space+ctrl+shift+6 → move-column-to-workspace 5" "6" [ "control" "shift" ] (ctl "command move-column-to-workspace 5"))
  (spaceBind "space+ctrl+shift+7 → move-column-to-workspace 6" "7" [ "control" "shift" ] (ctl "command move-column-to-workspace 6"))
  (spaceBind "space+ctrl+shift+8 → move-column-to-workspace 7" "8" [ "control" "shift" ] (ctl "command move-column-to-workspace 7"))
  (spaceBind "space+ctrl+shift+9 → move-column-to-workspace 8" "9" [ "control" "shift" ] (ctl "command move-column-to-workspace 8"))
  (spaceBind "space+ctrl+shift+] → move-column-to-workspace down" "close_bracket" [ "control" "shift" ] (ctl "command move-column-to-workspace down"))
  (spaceBind "space+ctrl+shift+[ → move-column-to-workspace up"   "open_bracket"  [ "control" "shift" ] (ctl "command move-column-to-workspace up"))

  # ── macOS Hide ブロック (space と無関係、保持) ─────────────────────────
  {
    description = "block cmd-h (macOS Hide)";
    manipulators = [{
      type = "basic";
      from = { key_code = "h"; modifiers = { mandatory = [ "left_command" ]; }; };
      to = [{ key_code = "vk_none"; }];
    }];
  }
  {
    description = "block cmd-alt-h (macOS Hide Others)";
    manipulators = [{
      type = "basic";
      from = { key_code = "h"; modifiers = { mandatory = [ "left_command" "option" ]; }; };
      to = [{ key_code = "vk_none"; }];
    }];
  }
]
