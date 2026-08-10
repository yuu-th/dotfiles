# ── omniwm Karabiner ルール（OmniWM 0.5.9 / space レイヤに全面集約）────────
#
# "Hyper modifier — space を karabiner variable で普通の modifier 化"
#
# 仕組み (v2.5、time-limit 無し):
#   1. spaceLeader: bare spacebar 押下で variable `space_held=1` を立てる。
#      OS には何も流さない。release 時に `to_if_alone` が発火して通常 space
#      を OS に届ける。途中で他キーが押されると `to_if_alone` はキャンセル、
#      `to_after_key_up` で `space_held=0` に戻る。
#      → 「普通の modifier (shift/ctrl と同じ) のように扱える」: 長押し中
#        ずっと variable=1、letter を押すまでの時間制限なし。
#      → modifier 押下中の space (ctrl+space / shift+space) は spaceLeader
#        の optional=[] により **マッチしない** → karabiner は無干渉で
#        ctrl+space などをそのまま OS へ流す。race ゼロ。
#   2. spaceBind: 各キーを `space_held=1` の variable_if 条件付きで定義。
#      space hold 中の押下が即 shell_command 発火。time threshold 無し。
#
# ── OmniWM native hotkey を使わない理由 ──
# Option ベースの打鍵がしづらいという判断で、ウィンドウ操作を全部この層に集約した。
# hotkeys.nix 側は全 149 ID を "Unassigned" にしてある（緊急脱出の 2 つを除く）。
#
# ── 使ってはいけないキー ──
#   space+space        … spaceLeader が自分自身に再入する
#   space+return       … 文章入力で「行末に space → Enter」を打つと改行が消える
#   space+0            … 「space の後に数字」は入力中に起きる。1-9 は WS で
#                        既にこのリスクを負っているが、これ以上増やさない
#
# ── OmniWM 0.5.9 で確認済みの前提 ──
#   * `command focus-column <n>` と `command move-column-to-workspace <n>` の
#     数値引数は **1-based**（capabilities の arg kind が "One-based column index"
#     / "Positive numeric workspace ID"）。0 を渡すと exit 3 になる。
#   * `set-container-primary-span` / `set-window-secondary-span` の size-change は
#     `+10%` / `-10%` をそのまま渡せる（先頭ハイフンでもフラグ扱いされない・実機確認済み）。
#   * Overview は modal で、開いている間は IPC コマンドが全部 `ignored_overview` に
#     なる。よって space+z は「開く」専用で、閉じるのは Escape / Enter / 背景クリック。
{ moveWindowToNamedWS
, omniwmctl
}:
let
  ctl = args: "${omniwmctl} ${args}";
  cmd = args: ctl "command ${args}";
  ws  = name: ctl "workspace focus-name ${name}";
  moveTo = name: "${moveWindowToNamedWS}/bin/omniwm-move-window-to-named-ws ${name}";

  # spaceLeader: bare spacebar (no modifier) → 変数 space_held=1。
  # `optional = []` で modifier 押下中の space は match せず素通し。
  # `to_if_alone_timeout_milliseconds = 5000` で 5 秒以内のタップなら
  # release 時に space emit (長時間 hold は modifier 専用扱い)。
  spaceLeader = {
    description = "Hyper modifier: space → space_held variable";
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

  # variable_if space_held=1 のもとで key (+ mandatory modifier) を
  # shell_command にすり替える。
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

  # ── WS 定義（キーの上下段 = ディスプレイの上下層）──────────────────────
  # 上段 w/e/r = メイン作業、下段 s/d/f = そのプロジェクトのブラウザ、
  # 最下段 x/c/v = 常駐（Media / Chat / 予定・ノート）。
  wsLetters = [ "W" "E" "R" "S" "D" "F" "X" "C" "V" ];
  wsDigits  = [ "1" "2" "3" "4" "5" "6" "7" "8" "9" ];

  lowerOf = {
    "W" = "w"; "E" = "e"; "R" = "r";
    "S" = "s"; "D" = "d"; "F" = "f";
    "X" = "x"; "C" = "c"; "V" = "v";
  };
  digitKeyOf = {
    "1" = "1"; "2" = "2"; "3" = "3"; "4" = "4"; "5" = "5";
    "6" = "6"; "7" = "7"; "8" = "8"; "9" = "9";
  };

  # WS 切替 / WS へ送る を letters と digits について生成する
  wsSwitchLetter = n: spaceBind "space+${lowerOf.${n}} → workspace ${n}"
    lowerOf.${n} [ ] (ws n);
  wsMoveLetter = n: spaceBind "space+shift+${lowerOf.${n}} → move to ${n}"
    lowerOf.${n} [ "shift" ] (moveTo n);
  wsSwitchDigit = n: spaceBind "space+${n} → workspace ${n}"
    digitKeyOf.${n} [ ] (ws n);
  wsMoveDigit = n: spaceBind "space+shift+${n} → move to ${n}"
    digitKeyOf.${n} [ "shift" ] (moveTo n);

  # focus-column / move-column-to-workspace は 1-based。
  # space+ctrl+N → N 列目、space+ctrl+shift+N → WS N へ column を送る。
  focusColumn = n: spaceBind "space+ctrl+${n} → focus-column ${n}"
    digitKeyOf.${n} [ "control" ] (cmd "focus-column ${n}");
  moveColumnToWs = n: spaceBind "space+ctrl+shift+${n} → move-column-to-workspace ${n}"
    digitKeyOf.${n} [ "control" "shift" ] (cmd "move-column-to-workspace ${n}");
in
[
  # ── space leader root manipulator ────────────────────────────────────
  spaceLeader
]
# ── WS 切替 & WS へ送る ─────────────────────────────────────────────────
++ (map wsSwitchLetter wsLetters)
++ (map wsMoveLetter   wsLetters)
++ (map wsSwitchDigit  wsDigits)
++ (map wsMoveDigit    wsDigits)
# ── Niri column 操作（1-based）──────────────────────────────────────────
++ (map focusColumn    wsDigits)
++ (map moveColumnToWs wsDigits)
++ [
  # ── scratchpad（WS 並みに押しやすい位置に置く）──────────────────────
  # WS と違って「今いる場所に呼び出せる」のが本質的な差。
  (spaceBind "space+a → scratchpad toggle" "a" [ ] (cmd "scratchpad toggle"))
  (spaceBind "space+shift+a → scratchpad assign" "a" [ "shift" ] (cmd "scratchpad assign"))

  # ── OmniWM 固有 UI ─────────────────────────────────────────────────
  (spaceBind "space+g → Quake Terminal" "g" [ ] (cmd "toggle-quake-terminal"))
  # Command Palette は窓検索 / クリップボード履歴 / メニュー検索の共通入口。
  # クリップボード履歴を有効にしているのでここが高頻度キーになる。
  (spaceBind "space+t → Command Palette" "t" [ ] (cmd "open-command-palette"))
  # Overview は modal。開くだけで、閉じるのは Escape / Enter / 背景クリック。
  (spaceBind "space+z → Overview を開く" "z" [ ] (cmd "toggle-overview"))
  (spaceBind "space+ctrl+g → system stats" "g" [ "control" ] (cmd "toggle-system-stats"))

  # ── 行方不明の窓の救済 ─────────────────────────────────────────────
  (spaceBind "space+n → raise-all-floating-windows" "n" [ ] (cmd "raise-all-floating-windows"))
  (spaceBind "space+shift+n → rescue-offscreen-windows" "n" [ "shift" ] (cmd "rescue-offscreen-windows"))

  # ── focus 方向（WS 内で完結。crossesMonitorAtEdge = false）──────────
  (spaceBind "space+h → focus left"  "h" [ ] (cmd "focus left"))
  (spaceBind "space+j → focus down"  "j" [ ] (cmd "focus down"))
  (spaceBind "space+k → focus up"    "k" [ ] (cmd "focus up"))
  (spaceBind "space+l → focus right" "l" [ ] (cmd "focus right"))
  (spaceBind "space+; → focus previous" "semicolon" [ ] (cmd "focus previous"))

  # ── 窓の移動（moveCrossesMonitorAtEdge = true なので端で隣モニタへ抜ける）──
  (spaceBind "space+shift+h → move left"  "h" [ "shift" ] (cmd "move left"))
  (spaceBind "space+shift+j → move down"  "j" [ "shift" ] (cmd "move down"))
  (spaceBind "space+shift+k → move up"    "k" [ "shift" ] (cmd "move up"))
  (spaceBind "space+shift+l → move right" "l" [ "shift" ] (cmd "move right"))

  # ── WS ナビゲーション ───────────────────────────────────────────────
  (spaceBind "space+] → switch-workspace next" "close_bracket" [ ] (cmd "switch-workspace next"))
  (spaceBind "space+[ → switch-workspace prev" "open_bracket"  [ ] (cmd "switch-workspace prev"))
  (spaceBind "space+tab → switch-workspace back-and-forth" "tab" [ ] (cmd "switch-workspace back-and-forth"))
  (spaceBind "space+shift+] → move-to-workspace down" "close_bracket" [ "shift" ] (cmd "move-to-workspace down"))
  (spaceBind "space+shift+[ → move-to-workspace up"   "open_bracket"  [ "shift" ] (cmd "move-to-workspace up"))

  # ── モニタ間の focus（方向指定は無い。循環のみ）──────────────────────
  (spaceBind "space+ctrl+tab → focus-monitor next" "tab" [ "control" ] (cmd "focus-monitor next"))
  (spaceBind "space+ctrl+shift+tab → focus-monitor prev" "tab" [ "control" "shift" ] (cmd "focus-monitor prev"))

  # ── column 操作 ─────────────────────────────────────────────────────
  (spaceBind "space+ctrl+[ → focus-column first" "open_bracket"  [ "control" ] (cmd "focus-column first"))
  (spaceBind "space+ctrl+] → focus-column last"  "close_bracket" [ "control" ] (cmd "focus-column last"))
  (spaceBind "space+ctrl+shift+h → move-column left"  "h" [ "control" "shift" ] (cmd "move-column left"))
  (spaceBind "space+ctrl+shift+l → move-column right" "l" [ "control" "shift" ] (cmd "move-column right"))
  (spaceBind "space+ctrl+shift+] → move-column-to-workspace down" "close_bracket" [ "control" "shift" ] (cmd "move-column-to-workspace down"))
  (spaceBind "space+ctrl+shift+[ → move-column-to-workspace up"   "open_bracket"  [ "control" "shift" ] (cmd "move-column-to-workspace up"))

  # ── 複数窓を 1 column に畳む ────────────────────────────────────────
  (spaceBind "space+shift+t → column タブ化トグル" "t" [ "shift" ] (cmd "toggle-column-tabbed"))
  (spaceBind "space+ctrl+, → 左の窓を consume/expel" "comma"  [ "control" ] (cmd "consume-or-expel-window-left"))
  (spaceBind "space+ctrl+. → 右の窓を consume/expel" "period" [ "control" ] (cmd "consume-or-expel-window-right"))

  # ── size / 構造（Option ベースから全面移管）─────────────────────────
  (spaceBind "space+m → fullscreen" "m" [ ] (cmd "toggle-fullscreen"))
  (spaceBind "space+shift+m → floating トグル" "m" [ "shift" ] (cmd "toggle-focused-window-floating"))
  (spaceBind "space+b → 全幅トグル" "b" [ ] (cmd "toggle-container-full-primary-span"))
  (spaceBind "space+, → 幅プリセット cycle backward" "comma"  [ ] (cmd "cycle-size backward"))
  (spaceBind "space+. → 幅プリセット cycle forward"  "period" [ ] (cmd "cycle-size forward"))
  (spaceBind "space+- → 幅 -10%" "hyphen"    [ ] (cmd "set-container-primary-span -10%"))
  (spaceBind "space+= → 幅 +10%" "equal_sign" [ ] (cmd "set-container-primary-span +10%"))
  (spaceBind "space+shift+- → 高さ -10%" "hyphen"    [ "shift" ] (cmd "set-window-secondary-span -10%"))
  (spaceBind "space+shift+= → 高さ +10%" "equal_sign" [ "shift" ] (cmd "set-window-secondary-span +10%"))
  (spaceBind "space+/ → balance-sizes" "slash" [ ] (cmd "balance-sizes"))

  # ── macOS Hide ブロック（space と無関係、保持）───────────────────────
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
