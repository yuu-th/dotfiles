# ── ワークスペース構築ヘルパ（v3: プロファイル + runtime 解決対応）──────────
#
# OmniWM v0.4.8 の monitorAssignment は decoder 不安定性のため、堅牢に動かすには
# specificDisplay に **実 displayId** を必ず含める必要がある。displayId は
# ハードウェア依存で nix ビルド時には不明なので、deploy.sh が runtime に
# system_profiler から解決して TOML を書き換える設計。
#
# プロファイルファイル（monitor-profiles/*.nix）が `display "X"` や
# `unnamedDisplay` を使うと、TOML に placeholder が出力される：
#   displayId = 0 + name = "X"          ← deploy.sh が name で resolve
#   displayId = 0 + name = ""           ← deploy.sh が unnamed として resolve
#
# 解決失敗（モニタ未接続等）時は deploy.sh が monitorAssignment を main/secondary
# にフォールバック書き換えするため、プロファイルが現状と合わなくても crash しない。
{ ... }:
{
  mkWorkspaces = { monitorMap, layoutMap ? { } }:
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
        "M" = { rawName = "10"; displayName = "M"; };
        "B" = { rawName = "11"; displayName = "B"; };
        "E" = { rawName = "12"; displayName = "E"; };
      };

      order = [ "1" "2" "3" "4" "5" "6" "7" "8" "9" "M" "B" "E" ];

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
        "M" = "a000000a-0000-4000-8000-00000000000a";
        "B" = "a000000b-0000-4000-8000-00000000000b";
        "E" = "a000000c-0000-4000-8000-00000000000c";
      };

      mk = key:
        let
          r = rawNames.${key};
          layout = layoutMap.${key} or "niri";
        in
        {
          id = uuids.${key};
          name = r.rawName;
          layoutType = layout;
          monitorAssignment = monitorMap.${key};
        }
        // (if r.displayName != null then { inherit (r) displayName; } else { });
    in
      map mk order;

  # ── monitorAssignment ヘルパ ────────────────────────────────────────────
  # main / secondary は OmniWM ネイティブ、常に堅牢。
  main      = { type = "main"; };
  secondary = { type = "secondary"; };

  # 名前付きモニタへ厳密ピン留め。deploy.sh が runtime に displayId を解決。
  # 解決失敗時は secondary にフォールバック。
  display = displayName: {
    type = "specificDisplay";
    output = {
      name = displayName;
      displayId = 0;             # placeholder, deploy.sh が解決
    };
  };

  # 名前なしモニタ（macOS が EDID name を持たないモニタ）。deploy.sh が解決。
  unnamedDisplay = {
    type = "specificDisplay";
    output = {
      name = "";
      displayId = 0;             # placeholder
    };
  };
}
