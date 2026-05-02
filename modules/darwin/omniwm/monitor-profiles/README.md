# Monitor Profiles

各 .nix ファイルは「あなたが使うモニタ構成 1 つ」に対するワークスペース配置を定義する。

## 切替方法

`profiles/darwin.nix` の以下を編集：

```nix
myConfig.darwin.omniwm.monitorProfile = "office-3mon";   # 既定: "default"
```

切替後 `sudo darwin-rebuild switch --flake .#yuta`。
モニタを物理的に抜き差ししても OmniWM が auto-detect するので、プロファイル自体は
日常的には切替えない（環境（家/オフィス/外出）が変わる時だけ切替）。

## 既存プロファイル

| ファイル | 想定構成 | 特徴 |
|---|---|---|
| `default.nix` | 任意の 1〜N モニタ | main/secondary だけ。完全に堅牢、どこでも動く |
| `office-3mon.nix` | Built-in + HP V27ie G5 + 名前なしモニタ | 名前指定で厳密ピン留め |

## 新規プロファイルの作り方

1. `monitor-profiles/<name>.nix` を作成（既存プロファイルを参考に）
2. helpers の関数を組み合わせて `monitorMap` を定義：
   - `main` — macOS の primary display（通常 Built-in）
   - `secondary` — primary 以外（複数あれば OmniWM が 1 つ選ぶ）
   - `display "X"` — 名前 X のモニタへ厳密ピン留め（例: `display "HP V27ie G5"`）
   - `unnamedDisplay` — EDID name を持たないモニタ
3. `git add` してから `sudo darwin-rebuild switch --flake .#yuta`

## 解決失敗時の挙動（堅牢性）

deploy.sh は OmniWM 起動前に `system_profiler` でモニタ一覧を取得し、
プロファイルの `display "X"` が存在するかチェックする：

- 存在する → 実 displayId を埋め込んで specificDisplay で deploy
- 存在しない → そのワークスペースの monitorAssignment を `secondary` に書き換え

これにより：
- プロファイルが「現状のモニタと完全には一致しない」状態でも crash しない
- 出張先で外部モニタが違っても、main + secondary で最低限動く
- 自宅に戻れば本来のモニタで厳密配置が復活

## モニタ名を調べる方法

```bash
omniwmctl query displays --format json | jq -r '.result.payload.displays[].name'
# または
system_profiler SPDisplaysDataType -json | jq -r '.SPDisplaysDataType[].spdisplays_ndrvs[]?._name'
```

`spdisplays_display` という名前は「EDID name を持たないモニタ」を意味する内部識別子。
このタイプのモニタは `unnamedDisplay` で指定する。
