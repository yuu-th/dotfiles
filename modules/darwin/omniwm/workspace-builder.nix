# ── ワークスペース構築ヘルパ ────────────────────────────────────────────────
# 各モニタプロファイルから monitor map（"WS名" → assignment）を受け取り、
# OmniWM が要求する完全な [[workspaces]] リストを生成する。
#
# UUID は固定（Nix で再生成しても変わらない）にすることで、OmniWM 側が
# 永続化するランタイム状態（最後に開いていた WS など）と整合を保つ。
#
# NOTE: OmniWM v0.4.8 時点では `specificDisplay` 形式の monitorAssignment を
# TOML から decode するパスに不具合があり、設定全体が corrupt 扱いになる
# （roundtrip 単体テストは通るが本番ロードで失敗する）。
# ここでは `main` / `secondary` のみを使い、特定モニタへの固定割当が必要な
# 場合は OmniWM GUI（Workspaces 設定タブ）で個別に設定する運用とする。
{ ... }:
{
  # ── monitorMap のキーは外部からの「論理名」(1〜9, M, B, E) を維持。
  # 内部的には OmniWM の rawID は数値のみ受理されるため、M/B/E は数値 rawID
  # (10, 11, 12) に変換し、displayName で人間可読ラベルを保つ。
  #
  # layoutMap は省略可能（デフォルト全 niri）。WS 単位に niri / dwindle を選べる。
  mkWorkspaces = { monitorMap, layoutMap ? { } }:
    let
      # 論理名 → (rawName, displayName) マッピング
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

      # ── 永続 WS の順序（AeroSpace の persistent-workspaces と同じ） ──
      order = [ "1" "2" "3" "4" "5" "6" "7" "8" "9" "M" "B" "E" ];

      # ── 固定 UUID テーブル ────────────────────────────────────────────────
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

  # ── monitorAssignment 構築ヘルパ ─────────────────────────────────────────
  main      = { type = "main"; };
  secondary = { type = "secondary"; };

  # specificDisplay: 名前付きモニタへの厳密ピン留め。
  # displayId は hardware 依存で nix ビルド時不明 → switch-profile.sh が runtime に
  # name を `omniwmctl query displays` で解決して 0 を実値に書き換える。
  display = displayName: {
    type = "specificDisplay";
    output = {
      name = displayName;
      displayId = 0;             # placeholder (runtime に switch-profile が patch)
    };
  };

  # 名前なしモニタ（macOS が EDID name を持たないモニタ）
  # 名前一致では識別できないため displayId のみで紐づける。
  # 999000 は switch-profile.sh が runtime に実 displayId に置換するマーカー。
  # 負値は OmniWM をクラッシュさせる、0 は noRuntimeDisplayId 扱いになり
  # 解決失敗するので、安全な大きな正の整数を使う。
  unnamedDisplay = {
    type = "specificDisplay";
    output = {
      name = "";
      displayId = 999000;        # placeholder, replaced by switch-profile.sh
    };
  };
}
