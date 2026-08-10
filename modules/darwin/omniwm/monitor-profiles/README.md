# Monitor Profiles

各 `.nix` ファイルは「あるモニタ構成 1 つ」に対する **ワークスペース配置 + OmniWM Routing map** を定義する。

## 切替方法

`profiles/darwin.nix`:

```nix
myConfig.darwin.omniwm.monitorProfile = "auto";        # 既定: 接続中モニタから自動選択
myConfig.darwin.omniwm.monitorProfile = "office-3mon"; # 強制指定
```

切替後 `sudo darwin-rebuild switch --flake .#yuta`。
`"auto"` ならモニタを物理的に抜き差ししても `deploy.sh` が再評価するので、
プロファイル指定を日常的に触る必要はない。

## 既存プロファイル

| ファイル | 想定構成 | match | routing |
|---|---|---|---|
| `default.nix` | 任意の 1〜N モニタ | なし（catch-all） | `macOS`（実配置を推測できないため） |
| `office-3mon.nix` | HP V27ie G5 + 名前なしモニタ + 内蔵 | `requiredDisplays=["HP V27ie G5"]`, `requireUnnamed=true` | `custom`（上→中→下） |
| `hp-2mon.nix` | HP V27ie G5 + 内蔵 | `requiredDisplays=["HP V27ie G5"]`, `monitorCount=2` | `custom`（上→下） |

match の評価は specificity（`requiredDisplays` の数 + `requireUnnamed` + `monitorCount`）降順 →
同点は名前順 → catch-all が最後。

## プロファイルの形

```nix
{ helpers }:
let
  inherit (helpers) mkWorkspaces main secondary display unnamedDisplay routeAt builtinName;
  hp = "HP V27ie G5";
in {
  match = {
    requiredDisplays = [ hp ];   # 全部接続されていること
    requireUnnamed   = true;     # EDID name を持たないモニタが必要
    monitorCount     = 3;        # （任意）モニタ枚数一致
  };

  # 実際の机の配置を宣言する（macOS の Arrange とは別物）
  routing = { mode = "custom"; };          # または "macOS"
  monitorRoutingOverrides = [
    (routeAt { name = hp;          row = 0; })
    (routeAt { name = "";          row = 1; })
    (routeAt { name = builtinName; row = 2; })
  ];

  workspaces = mkWorkspaces {
    monitorMap = {
      "W" = display hp;      # 名前指定でピン留め
      "S" = unnamedDisplay;  # EDID name を持たないモニタ
      "X" = main;            # macOS の主ディスプレイ
      # … 18 個すべてに指定が必要（W E R S D F X C V 1〜9）
    };
  };
}
```

## monitorMap のヘルパ

| ヘルパ | 意味 | 堅牢性 |
|---|---|---|
| `main` | macOS の主ディスプレイ | ◎ OmniWM がネイティブ解決 |
| `secondary` | 主ディスプレイ以外 | ◎ 同上 |
| `display "<名前>"` | その名前のモニタ | ○ deploy.sh が displayUUID を解決 |
| `unnamedDisplay` | EDID name を持たないモニタ | ○ 同上 |

`display` / `unnamedDisplay` は `@@OMNIWM_UUID:<selector>@@` トークンを出力し、
`deploy.sh` が ColorSync で実 UUID に置換する。**0.5.9 は displayUUID でしか
モニタを解決しない**ため（`displayId` + 名前の経路は成立しない）、この置換が必須。

## 解決失敗時の挙動（堅牢性）

| 事象 | 挙動 |
|---|---|
| `display "X"` の X が未接続 | その WS は **main の UUID** で代替され、`[routing] mode` は `macOS` に降格 |
| grid に無いモニタが繋がっている | OmniWM 側で `completeLayout` が nil を返し routing が macOS 配置にフォールバック |
| ディスプレイ列挙が空 / UUID を持つモニタがゼロ | **settings.toml を書かずに終了**（前回の良い設定を保持） |
| トークンが未置換で残った | **書かずに終了**（`displayUUID` の形式違反は `.corrupt` に直行するため） |

これにより「プロファイルが現状のモニタと完全一致しない」状態でも crash せず、
出張先で別の外部モニタを繋いでも `default` プロファイルで最低限動く。

## 新規プロファイルの作り方

1. `monitor-profiles/<名前>.nix` を作成（既存を参考に）
2. **`git add` する** — flake は git 管理下のファイルしかストアに取り込まないので、
   未追跡だと `builtins.readDir` から見えず**黙って無視される**
3. `sudo darwin-rebuild switch --flake .#yuta`
4. 選択結果をログで確認: `tail ~/.local/share/omniwm/deploy.log`

## モニタ名と UUID を調べる

```bash
# OmniWM が認識している名前（"" は EDID name なし）
omniwmctl query displays --format json | jq -r '.result.payload.displays[] | "\(.id) \(.name)"'

# deploy.sh が使う名前（system_profiler 側。内蔵は "Color LCD"、無名は "spdisplays_display"）
system_profiler SPDisplaysDataType -json | jq -r '.SPDisplaysDataType[].spdisplays_ndrvs[]?._name'

# displayUUID（ColorSync）
python3 - <<'EOF'
import ctypes
cg=ctypes.CDLL('/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics')
cs=ctypes.CDLL('/System/Library/Frameworks/ColorSync.framework/ColorSync')
cf=ctypes.CDLL('/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation')
n=ctypes.c_uint32(); ids=(ctypes.c_uint32*16)()
cg.CGGetActiveDisplayList(16, ids, ctypes.byref(n))
cs.CGDisplayCreateUUIDFromDisplayID.restype=ctypes.c_void_p
cs.CGDisplayCreateUUIDFromDisplayID.argtypes=[ctypes.c_uint32]
cf.CFUUIDCreateString.restype=ctypes.c_void_p
cf.CFUUIDCreateString.argtypes=[ctypes.c_void_p,ctypes.c_void_p]
cf.CFStringGetCString.argtypes=[ctypes.c_void_p,ctypes.c_char_p,ctypes.c_long,ctypes.c_uint32]
buf=ctypes.create_string_buffer(256)
for i in range(n.value):
    d=ids[i]; u=cs.CGDisplayCreateUUIDFromDisplayID(d); s=None
    if u:
        c=cf.CFUUIDCreateString(None,u)
        if c and cf.CFStringGetCString(c,buf,256,0x08000100): s=buf.value.decode()
    print(f"displayId={d} builtin={bool(cg.CGDisplayIsBuiltin(d))} uuid={s}")
EOF
```
