# ── ワークスペース構築ヘルパ（v4: OmniWM 0.5.9 / displayUUID ベース）──────────
#
# OmniWM 0.5.9 は workspace のモニタ固定を **displayUUID** でしか解決しない。
# `OutputId.resolveMonitor` は
#   - displayUUID があれば UUID で一意マッチ
#   - なければ「候補モニタの displayUUID が nil」かつ displayId 一致かつ名前一致
# を要求する。実機の全モニタは UUID を持つため、後者は**絶対に成立しない**。
# さらに `Monitor.namesMatch` は両方が非空文字を要求するので、名前なしモニタを
# 名前で指定することも原理的に不可能。
#
# よって nix 側は「どのモニタか」を名前で表明するだけにして、deploy.sh が runtime に
# ColorSync (CGDisplayCreateUUIDFromDisplayID) で UUID を解決し TOML を書き換える。
#
# 出力される placeholder の形:
#   [workspaces.monitorAssignment.output]
#   displayId = 0          ← deploy.sh が displayUUID = "<解決値>" に置換する印
#   name = "HP V27ie G5"   ← 解決キー（"" は「名前を持たないモニタ」を意味する）
#
# 解決失敗（モニタ未接続等）時は deploy.sh が monitorAssignment を secondary に
# 書き換えるので、プロファイルが現状と合わなくても crash しない。
{ ... }:
{
  # ── ワークスペース定義 ──────────────────────────────────────────────────
  #
  # ルール: キーの上下段 = ディスプレイの上下層
  #   上段 w e r + 数字 3-9 → メイン作業ディスプレイ
  #   下段 s d f + 数字 1,2 → ブラウザディスプレイ
  #   最下段 x c v          → 常駐（Media / Chat / 予定・ノート）
  #
  # rawName（数値）の連番が `switch-workspace next/prev` の巡回順になるので
  # 「数字 → 作業(W,E,R) → ブラウザ(S,D,F) → 常駐(X,C,V)」の順に振る。
  # こうするとモニタごとにまとまって巡回する。
  #
  # OmniWM の workspace `name` は数値のみ受理される（WorkspaceIDPolicy）。
  # 人間可読ラベルは displayName で持つ。`workspace focus-name` は両方を受理する。
  mkWorkspaces = { monitorMap }:
    let
      rawNames = {
        "1" = { rawName = "1";  displayName = null; };
        "2" = { rawName = "2";  displayName = null; };
        "3" = { rawName = "3";  displayName = null; };
        "4" = { rawName = "4";  displayName = null; };
        "5" = { rawName = "5";  displayName = null; };
        "6" = { rawName = "6";  displayName = null; };
        "7" = { rawName = "7";  displayName = null; };
        "8" = { rawName = "8";  displayName = null; };
        "9" = { rawName = "9";  displayName = null; };
        # ── 上段: メイン作業（プロジェクト 1/2/3）─────────────────────────
        "W" = { rawName = "10"; displayName = "W"; };
        "E" = { rawName = "11"; displayName = "E"; };
        "R" = { rawName = "12"; displayName = "R"; };
        # ── 下段: そのプロジェクトのブラウザ ──────────────────────────────
        "S" = { rawName = "13"; displayName = "S"; };
        "D" = { rawName = "14"; displayName = "D"; };
        "F" = { rawName = "15"; displayName = "F"; };
        # ── 最下段: 常駐（暗記系の役割 WS）────────────────────────────────
        "X" = { rawName = "16"; displayName = "X"; };   # Media
        "C" = { rawName = "17"; displayName = "C"; };   # Chat
        "V" = { rawName = "18"; displayName = "V"; };   # 予定 / ノート
      };

      order = [
        "1" "2" "3" "4" "5" "6" "7" "8" "9"
        "W" "E" "R"
        "S" "D" "F"
        "X" "C" "V"
      ];

      # 固定 UUID。rawName と一致させて追跡しやすくする。
      uuids = {
        "1" = "a0000001-0000-4000-8000-000000000001";
        "2" = "a0000002-0000-4000-8000-000000000002";
        "3" = "a0000003-0000-4000-8000-000000000003";
        "4" = "a0000004-0000-4000-8000-000000000004";
        "5" = "a0000005-0000-4000-8000-000000000005";
        "6" = "a0000006-0000-4000-8000-000000000006";
        "7" = "a0000007-0000-4000-8000-000000000007";
        "8" = "a0000008-0000-4000-8000-000000000008";
        "9" = "a0000009-0000-4000-8000-000000000009";
        "W" = "a0000010-0000-4000-8000-000000000010";
        "E" = "a0000011-0000-4000-8000-000000000011";
        "R" = "a0000012-0000-4000-8000-000000000012";
        "S" = "a0000013-0000-4000-8000-000000000013";
        "D" = "a0000014-0000-4000-8000-000000000014";
        "F" = "a0000015-0000-4000-8000-000000000015";
        "X" = "a0000016-0000-4000-8000-000000000016";
        "C" = "a0000017-0000-4000-8000-000000000017";
        "V" = "a0000018-0000-4000-8000-000000000018";
      };

      mk = key:
        let r = rawNames.${key};
        in
        {
          id = uuids.${key};
          name = r.rawName;
          # Dwindle は廃止したので全 WS niri 固定。
          layoutType = "niri";
          monitorAssignment = monitorMap.${key};
        }
        // (if r.displayName != null then { inherit (r) displayName; } else { });
    in
      map mk order;

  # ── monitorAssignment ヘルパ ────────────────────────────────────────────
  # main / secondary は OmniWM ネイティブ解決なので常に堅牢。
  main      = { type = "main"; };
  secondary = { type = "secondary"; };

  # 名前付きモニタへピン留め。deploy.sh が name → displayUUID を解決する。
  #
  # `@@OMNIWM_UUID:<selector>@@` は deploy.sh が置換するトークン。TOML パースを
  # 伴わない純粋な文字列置換で済むようにこの形にしている。
  # selector の意味は deploy.sh 側に定義がある:
  #   "<名前>"                  … その名前のモニタ
  #   ""                        … EDID name を持たないモニタ
  #   "Built-in Retina Display" … CGDisplayIsBuiltin で判定される内蔵ディスプレイ
  display = displayName: {
    type = "specificDisplay";
    output = {
      name = displayName;
      displayUUID = "@@OMNIWM_UUID:${displayName}@@";
    };
  };

  # 名前なしモニタ（macOS が EDID name を返さないモニタ）。
  unnamedDisplay = {
    type = "specificDisplay";
    output = {
      name = "";
      displayUUID = "@@OMNIWM_UUID:@@";
    };
  };

  # ── routing grid ヘルパ ─────────────────────────────────────────────────
  # OmniWM Routing map（Settings → Monitors の「実際の机の配置」）を宣言する。
  # `MonitorRouting.gridAdjacent` がこの grid で方向隣接を解決するので、macOS の
  # Arrange が実配置と食い違っていても方向操作と mouse warp が正しく動く。
  #
  # `MonitorSettingsStore.get` も workspace と同じ UUID 優先ロジックなので、
  # ここも monitorDisplayUUID が必須。deploy.sh が monitorName から解決する。
  #
  # 注意: 接続中モニタのうち 1 枚でも grid に無いと `completeLayout` が nil を返し
  # routing 全体が macOS 配置へフォールバックする（安全側の劣化）。
  routeAt = { name, row, column ? 0 }: {
    monitorName = name;
    monitorDisplayUUID = "@@OMNIWM_UUID:${name}@@";
    gridColumn = column;
    gridRow = row;
  };

  # 内蔵ディスプレイの論理名。deploy.sh は CGDisplayIsBuiltin で解決するので
  # この文字列は表示用でしかないが、プロファイル間で表記を揃えるために定数化する。
  builtinName = "Built-in Retina Display";
}
