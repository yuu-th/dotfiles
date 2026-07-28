# projwm-next 実装判断メモ

Status: 実装判断合意済み。`queue/design.md` と合わせて `projwm-next` の設計 SSOT とする。

この文書は実装コードではなく、`queue/design.md` で合意した設計骨組みを、実装可能な判断へ落とすための記録である。
実装の難所、判断、根拠、未確定リスクをここに集める。
実装・schema・validator・IPC contract・store contract・tests は、この文書と `queue/design.md` に従って整合させる。

## 1. 前提として固定した設計判断

`queue/design.md` の承認対象を実装前提にする。

| 判断 | 実装上の意味 |
|---|---|
| `projwmd` single writer | GUI/window/session/browser mutation は通常 `projwmd` だけが実行する |
| watcher は hint/evidence | watcher event は dirty scope / observe request に変換し、DesiredWorld を直接変更しない |
| lifecycle transaction | startup/wake/display/periodic は `projwmd` 内 transaction として扱う |
| Nix が environment authority | `projwmd` は環境値を永続変更せず、validate/report/suggest だけ行う |
| `state.json` は truth ではない | 永続 store は DesiredWorld/snapshot/checkpoint を保存する backend にする |
| Browser は first-class sub-world | browser window/tab/profile/snapshot は world model に出すが、raw URL/title content は private payload として分離する |
| title は identity ではない | `TitleContract` / `ObservedTitle` / `MatchHint` を分ける |
| appRules は placement authority ではない | OmniWM appRules は safety/admission に限定する |

## 2. 実装方針の大枠

### 2.1 既存実装を改造しすぎない

既存 `projwm` は adapter knowledge の参照元として使う。
ただし `cmd/reconcile.go` や `internal/reconcile` の構造を `projwm-next` の中心にはしない。

理由:

- 既存構造は command ごとに副作用を持ち、single transaction 境界を作りにくい。
- layout restore / reconcile / snapshot がそれぞれ truth を持ちやすい。
- 新設計は Controller transaction を中心にするため、既存 command 増築では責務が混ざる。

### 2.2 最初に作るべきものは「動く機能」ではなく「境界」

初期実装で優先するもの:

1. 型と authority 境界。
2. Nix 由来 environment manifest の読み込みと validation。
3. PersistentStore の schema と migration 方針。
4. Observer の最小実装。
5. Controller event loop の transaction 骨格。
6. Fake backend の story test。

実アプリ操作や完全な planner は後回しでよい。

理由:

- 最初に mutation を実装すると、また command-driven architecture に戻りやすい。
- fake/simulator で authority と transaction を固める方が安全。

## 3. Nix / environment manifest

### 判断

Nix は environment parameter の唯一の authority。
`projwmd` は Nix が生成する manifest を読む consumer/validator にする。

manifest の目的:

- Nix が所有する静的環境契約を `projwmd` に渡す。
- `projwmd` が runtime observation を解釈するための期待値を持つ。
- Nix と daemon の split-brain を検出する。
- test / trace / migration で「どの環境契約の下で動いたか」を再現可能にする。

manifest は人間が手編集する preferences ではない。
既存 `config.toml` は人間向け/運用向け config だが、managed environment manifest は Nix 生成の machine contract である。

想定:

```text
Nix modules
  -> projwm environment manifest
      - layout tuning
      - focus tuning
      - workspace topology
      - app admission policy
      - daemon topology
      - environment version
  -> projwmd reads manifest
  -> projwmd validates runtime observation
```

### 実装判断

- manifest は JSON にする。
- manifest は Nix store に生成し、`projwmd` の launchd 引数 `--managed-environment <path>` で渡す。
- `~/.config` には置かない。
- `projwmd` は manifest mismatch を `block` / `warn` / `report` に分類する。
- `projwmd` は manifest を書き戻さない。
- `projwmd` が環境変更を提案する場合は suggested Nix diff か report にする。

JSON にする理由:

- Nix から `builtins.toJSON` で自然に生成できる。
- Go 側は標準ライブラリ `encoding/json` だけで読める。
- 既存 OmniWM module も profile manifest を JSON として runtime script に渡している。
- manifest は手編集 config ではなく、Nix→daemon の schema 付き契約である。
- unknown field を forward-compatible に見逃すより、schema/version 不一致として検出する方が安全。

配置を Nix store にする理由:

- `~/.config` は人間が編集する設定の置き場に見え、Nix authority と衝突する。
- manifest は Nix build の生成物であり、ユーザーが直接編集する対象ではない。
- rebuild で store path が変わること自体が、環境契約の更新境界になる。
- `projwmd` は launchd 引数で渡された manifest path だけを読むため、どの契約で動いたかが明確になる。

想定実装:

```nix
managedEnvironment = pkgs.writeText "projwm-managed-environment.json"
  (builtins.toJSON manifest);

launchd.user.agents.projwmd.serviceConfig.ProgramArguments = [
  "${projwm}/bin/projwmd"
  "--managed-environment"
  "${managedEnvironment}"
];
```

`projwmctl` は通常 manifest を直接読まない。
必要な場合は `projwmd status environment` のような read-only API から見る。
daemon を介さない開発用実行だけ `--managed-environment <path>` を受け取る。

manifest 必須項目:

```json
{
  "schemaVersion": 1,
  "authority": "nix",
  "source": "modules/darwin/projwm",
  "minProjwmdVersion": "0.1.0",
  "windowManager": {
    "backend": "omniwm",
    "layout": {
      "defaultColumnWidth": 0.5,
      "maxVisibleColumns": 4,
      "maxWindowsPerColumn": 4,
      "centerFocusedColumn": "never"
    },
    "focus": {
      "followsMouse": false,
      "followsWindowToMonitor": true,
      "moveMouseToFocusedWindow": true
    }
  },
  "daemons": {
    "controller": "projwmd",
    "legacyAgents": "remove"
  }
}
```

schema 方針:

- `schemaVersion` は必須。
- `authority = "nix"` は必須。
- `minProjwmdVersion` より古い daemon は block。
- unknown field は原則 block。
- optional field を増やす場合も schemaVersion を上げる。
- error message は missing/unknown/unsupported を明確に出す。

validation 分類:

| 分類 | 条件 | 動作 |
|---|---|---|
| block | manifest parse/schema/version 不整合 | `projwmd` は mutation transaction を開始しない |
| block | authority が `nix` ではない | split-brain 防止のため停止 |
| block | single writer を破る legacy agent が active | mutation を止め report |
| warn | runtime 観測値と Nix manifest の drift | transaction は継続し report |
| warn | monitor profile fallback | display lifecycle transaction で再観測 |
| report | suggested Nix diff | user に提示するだけ |
| report | query で検証不能な項目 | 未検証として trace に残す |

### runtime validation の確認結果と方針

- `omniwmctl query` は windows/workspaces/displays/rules/capabilities/subscriptions を JSON で取得できる。
- `omniwmctl watch` は focus/workspace-bar/active-workspace/focused-monitor/windows-changed/display-changed/layout-changed を購読できる。
- watch payload の順序/欠落/重複保証は contract として信用しない。
- appRules は `query rules` で観測し、manifest との差分を validation report にできる。
- display/workspace/window は query snapshot で再構成する。
- launchd legacy agent は launchctl の観測で report できるが、除去は Nix rebuild 側の責務にする。
- runtime で検証不能な manifest 項目は block ではなく report にする。

### 既存根拠

- 既存 `projwm` は `~/.config/projwm/config.toml` を `home.file` で配置しているが、これは manifest ではなく人間向け運用 config である。
- OmniWM 共通設定は `version = 5` を持つ。
- OmniWM module は profile manifest を `builtins.toJSON` で runtime script に渡している。
- OmniWM deploy script は Nix 生成 JSON を runtime に渡しており、Nix→runtime contract として JSON を使う前例がある。
- 現行 `projwm` launchd は watch / periodic / startup / wake / layout の複数 agent 構成で、single writer を破る。
- 現行 appRules は float/minSize/Ghostty titleRegex 中心で、placement authority にはしない方針と相性がよい。

### Authority map の固定

この節は実装前 blocking の最初の決定である。
`projwm-next` では、設定・意図・観測・予測・永続化・runtime cache を同じ truth として扱わない。

| 領域 | authority | 書いてよいもの | 書いてはいけないもの |
|---|---|---|---|
| Nix source | Nix | installed apps, executable paths, static workspace topology, layout/focus tuning, app admission, daemon topology, managed environment schema | DesiredWorld, live windows, accepted snapshots, runtime queue |
| Managed environment manifest | Nix-generated machine contract | Nix source から導出された static environment contract, schema/version/min daemon version | project/profile assignment, browser snapshot, accepted layout, live ID, current focus |
| legacy `config.toml` | legacy/developer input only | migration hint or local development override, if explicitly enabled | normal daemon authority, Nix-owned environment value, DesiredWorld |
| OmniWM profile source | Nix | source TOML/profile definitions and deploy manifest | current live window placement, project desired state |
| `~/.config/omniwm/settings.toml` | runtime-generated cache | deploy script が現在 display に合わせて生成した applied settings cache | durable Nix truth, projwm DesiredWorld, user/project state |
| PersistentStore | durable committed record authority | committed DesiredWorld, accepted semantic layout, browser snapshots, checkpoint/journal, migration metadata | policy decision, live observation, predicted state, raw observer facts |
| `projwmd` Controller | transition authority | intent validation, transaction lifecycle, planner/replan decisions, commit decision, in-memory queue | Nix contract mutation, uncommitted state as durable truth |
| Observer | live observation authority | timestamped live display/window/workspace/process/browser facts | DesiredWorld, accepted layout, browser snapshot commit |
| PredictedWorld | in-memory prediction only | operation effect prediction within current transaction | restart recovery truth, PersistentStore commit without verification |
| `projwmctl` | intent client | user/admin intent request, read-only status query via daemon | direct GUI mutation, direct store mutation fallback |
| test/admin/migration | gated intent source | fixture/migration intent through Controller and PersistentStore API under explicit test/migration mode | raw file patch, production store mutation bypass, direct adapter mutation |

#### Manifest scope

Managed environment manifest に入れてよいもの:

- schema/version/source/min daemon version。
- WM backend/capabilities。
- layout/focus tuning。
- workspace topology と role。
- app admission/safety rule。
- daemon topology と legacy agent policy。
- validation policy。

Managed environment manifest に入れてはいけないもの:

- active profile。
- project list / profile assignment。
- DesiredWindow / DesiredBrowserSession。
- accepted semantic layout。
- browser snapshot URL/title。
- LiveWindowID / PID / frame / current focus。
- transaction state / event queue。

理由:

- manifest は Nix から daemon への environment contract であり、runtime desired state ではない。
- manifest に project/profile/browser/layout を入れると、Nix と PersistentStore の二重 truth になる。
- Nix rebuild は environment contract 更新境界であり、user intent commit 境界ではない。

#### legacy `config.toml` の扱い

既存 `config.toml` は `projwm-next` の通常 daemon authority から外す。

扱い:

1. migration では legacy input として読んでよい。
2. developer mode では明示 flag 付き override として読んでよい。
3. 通常 `projwmd` は Nix store の managed environment manifest を正本にする。
4. `safe to edit` な generated config という扱いはしない。

既存 `config.toml` の値の移管先:

| 既存 field | next の authority |
|---|---|
| `viewer_workspace` | Nix managed workspace topology |
| `slot_names` | Nix workspace topology / slot order |
| `terminal_app_path` | Nix app environment |
| `terminal_bundle_id` | Nix app environment / admission |

ユーザーが将来持つ preferences は、managed environment manifest とは別の user preference 領域に分ける。
ただし最初の実装では user preference file を作らない。

#### OmniWM `settings.toml` の扱い

現行 OmniWM deploy は display change で `~/.config/omniwm/settings.toml` を生成し、必要なら OmniWM を kickstart する。
この file は Nix source truth ではなく **runtime-generated applied settings cache** として扱う。

所有権:

- Nix owns: profile source, static TOML derivation, profile manifest, selected policy。
- deploy script owns: applied runtime cache generation。
- OmniWM owns: running WM process and loaded settings。
- `projwmd` owns: drift validation and lifecycle reaction, not settings write。

`projwmd` がしてよいこと:

- manifest と runtime observation の drift を report/warn/block する。
- display change 後に deploy/kickstart が起きた前提で full observe する。
- applied settings generation/source stamp を read-only に検査する。

`projwmd` がしてはいけないこと:

- `~/.config/omniwm/settings.toml` を直接書く。
- OmniWM profile source を書き換える。
- deploy script の代わりに display-specific settings を生成する。

#### PersistentStore の権限

PersistentStore は判断主体ではない。
PersistentStore は **committed durable record の authority** である。

書き込み rule:

1. Controller が transaction を開始する。
2. Planner/Simulator/Executor/Settler/Verifier が transaction result を作る。
3. Controller が commit 可否を決める。
4. commit 可の場合だけ PersistentStore に desired/checkpoint/snapshot/journal を書く。
5. Observer は PersistentStore へ直接書かない。
6. PredictedWorld はそのまま PersistentStore へ書かない。

PersistentStore の subdocument:

| subdocument | 性質 | commit 条件 |
|---|---|---|
| DesiredWorld | user/project desired truth | user intent または migration intent が validation を通る |
| accepted semantic layout | DesiredWorld に紐づく accepted snapshot | manual layout accept または controller-approved layout transaction |
| browser snapshot | recovery data / snapshot boundary | explicit snapshot transaction と privacy policy 通過 |
| checkpoint | recovery metadata | DesiredWorld と同じ transaction ID で commit |
| journal | crash/debug/recovery aid | transaction boundary ごと。truth ではない |

#### restart recovery order

restart 後は以下の順で復旧する。

1. Nix managed environment manifest を load/validate する。
2. PersistentStore の last committed generation を load する。
3. predicted state / in-flight queue は破棄する。
4. transaction journal があれば recovery aid として読むが、truth にはしない。
5. Observer で live world を full observe する。
6. Desired / Observed / Environment から dirty scope を計算する。
7. 必要なら `LifecycleBootstrap` transaction を開始する。

#### migration quarantine

legacy `state.json` は authority が混ざっているため、migration input は quarantine 扱いにする。

auto-promote してよいもの:

- profile 名と assignment。
- project 名と cwd。
- window kind と stable ordinal。
- browser profile 名。

quarantine または破棄するもの:

- `LiveWindowID`: 破棄。
- frame / raw layout coordinate: 破棄または manual review。
- `SavedURLs`: browser snapshot 候補として privacy policy 通過後に import。
- current focus / active live window: 破棄。
- observer 由来に見える値: DesiredWorld へ自動昇格しない。

#### Authority map の実装条件

Authority map が満たされるまで、real backend mutation には進まない。

最低条件:

1. manifest schema が DesiredWorld を含まないこと。
2. normal daemon が legacy `config.toml` を authority として読まないこと。
3. OmniWM applied settings cache を Nix truth と誤認しないこと。
4. PersistentStore write が Controller commit 経由に限定されること。
5. Observer が store write API を持たないこと。
6. restart recovery が predicted state を破棄すること。
7. migration が whitelist/quarantine 方式であること。

## 4. PersistentStore

### 判断

`state.json` は単一 truth ではない。
ただし store backend は必要。

保存するもの:

- DesiredWorld
- accepted semantic layout
- browser snapshot
- controller checkpoint
- migration metadata

保存しないもの:

- LiveWindowID
- frame 座標
- 現在 focus live window
- query で再観測できる process/window/app 状態

### 実装判断

- 最初は file-based store でよい。
- 単一巨大 `state.json` ではなく、store directory にする。
- committed generation directory と `CURRENT` pointer による atomic commit にする。
- atomic write / fsync / schema version / checksum は必須。
- legacy `state.json` は migration input として扱う。

directory layout:

```text
~/.local/state/projwm-next/
  .store_identity.json
  LOCK
  CURRENT

  generations/
    E0001-G000001/
      manifest.json
      desired_world.json
      accepted_layout.json
      browser_snapshot.json
      checkpoint.json
      journal.jsonl

  .staging/
    txn-E0001-G000002-abc123/

  quarantine/
    legacy-state-20260101T120000Z/
      state.json
      reason.json

  migrations/
    migration-log.jsonl

  repair/
    repair-log.jsonl
```

schema 判断:

| ファイル | 所有するもの | truth 扱い |
|---|---|---|
| `CURRENT` | current generation directory 名 | pointer |
| `generations/*/manifest.json` | generation metadata / checksums / schema versions | commit 判定 |
| `generations/*/desired_world.json` | DesiredWorld | yes |
| `generations/*/accepted_layout.json` | accepted semantic layout | yes, if referenced by desired/checkpoint |
| `generations/*/browser_snapshot.json` | browser profile/window/tab restore data | private snapshot boundary |
| `generations/*/checkpoint.json` | generation / transaction / digest / recovery metadata | recovery authority |
| `generations/*/journal.jsonl` | transaction audit/debug | no, replay authority ではない |
| `.staging/*` | incomplete transaction | no |
| `quarantine/*` | broken/legacy input | no |
| legacy `state.json` | migration input | no |

atomicity:

- committed generation は immutable。
- Controller commit だけが new generation を作れる。
- artifact は `.staging/txn-*` に書き、checksum と schema validation を通してから `generations/<id>` へ rename する。
- commit は `CURRENT` temp file + rename で visible になる。
- `CURRENT` は generation directory 名だけを指し、absolute path や symlink は使わない。
- 起動時は原則 `CURRENT` の指す generation だけを正本にする。

commit sequence:

```text
1. acquire exclusive store lock
2. verify store identity / kind / schema family
3. allocate next epoch/generation under lock
4. create .staging/txn-<generation>-<nonce>/
5. write artifact temp files
6. fsync each artifact file
7. rename artifact temp files to final names inside staging
8. fsync staging dir
9. write manifest temp with file checksums / parent / schema versions
10. fsync manifest temp
11. rename manifest temp to manifest.json
12. fsync staging dir
13. rename staging dir to generations/<generation>
14. fsync generations dir
15. write CURRENT.tmp with generation dir name
16. fsync CURRENT.tmp
17. rename CURRENT.tmp to CURRENT
18. fsync store root dir
19. release lock
```

manifest 必須項目:

```json
{
  "epoch": 1,
  "generation": 42,
  "transactionId": "E0001-G000042-abc123",
  "parentGeneration": "E0001-G000041",
  "committedBy": "controller",
  "commitKind": "user-intent",
  "storeSchemaVersion": 1,
  "artifactSchemaVersions": {
    "desiredWorld": 1,
    "acceptedLayout": 1,
    "browserSnapshot": 1,
    "checkpoint": 1,
    "journal": 1
  },
  "files": {
    "desired_world.json": { "sha256": "..." },
    "accepted_layout.json": { "sha256": "..." },
    "browser_snapshot.json": { "sha256": "..." },
    "checkpoint.json": { "sha256": "..." },
    "journal.jsonl": { "sha256": "..." }
  }
}
```

committed 判定:

1. generation directory が `generations/` 配下にある。
2. `manifest.json` が存在し schema/version を読める。
3. manifest checksum が全 artifact と一致する。
4. parent relation が妥当。
5. `CURRENT` がその generation を指している、または recovery scan で一意に選ばれた。

schema:

- store-wide schema と artifact schema を分ける。
- binary が対応していない newer schema は read-only または fail にする。
- down migration はしない。
- schema migration も generation commit として記録する。

journal / checkpoint:

- checkpoint/artifacts が recovery authority。
- journal は audit/debug/recovery hint であり、通常 recovery で replay しない。
- checkpoint は generation id / parent / controller commit id / artifact digest / schema versions / created_at を含む self-contained metadata にする。
- journal retention は明示し、GC は current generation と parent chain を壊さない。

production / test store separation:

- store root に `.store_identity.json` を必須にする。
- `store_kind` は `production` / `test` / `recovery` などを持つ。
- test process は production store を開けない。
- production process は test store を原則開けない。
- path は realpath 正規化、owner/permission、same filesystem、root escape、marker を検査する。
- production mutation path で任意 `--state-dir` override を許さない。

crash recovery:

```text
1. acquire store lock
2. read .store_identity.json
3. read CURRENT
4. validate pointed generation
5. valid ならそれを使う
6. invalid なら recovery mode で generations を scan
7. single unambiguous highest valid generation なら CURRENT を復旧
8. ambiguous なら automatic recovery せず offline repair を要求
9. .staging は never committed とし quarantine する
10. release lock
```

`.staging` 配下の transaction は、完全に見えても committed 扱いしない。

migration:

1. legacy flock を可能なら取得する。
2. new store lock を取得する。
3. legacy `state.json` を quarantine/migration-input へ copy する。
4. copy した file を parse する。
5. whitelist field だけ extract / normalize する。
6. semantic validation を通す。
7. migration generation を `.staging` に作る。
8. normal generation commit protocol で commit する。
9. legacy file は explicit user-approved migration なしに rename/delete しない。

legacy migration whitelist:

| legacy field | 扱い |
|---|---|
| `ActiveProfile` | DesiredWorld 候補。validation 後に import |
| `Profiles` / assignments | DesiredWorld 候補。workspace topology と照合 |
| `Projects` / cwd | DesiredWorld 候補。path validation 後に import |
| `Window.kind` / `Window.id` | stable ordinal として import |
| `BrowserProfile` | browser desired/snapshot metadata 候補 |
| `SavedURLs` | private browser snapshot 候補。ログ/report には count のみ |
| `Layout` | accepted layout 候補。semantic validation 不能なら quarantine/manual review |
| `LiveWindowID` | 破棄 |
| frame / current focus / live PID | 破棄 |
| unknown fields | quarantine metadata に記録し、意味づけしない |

`SavedURLs` は migrate してよいが private payload として扱う。
ログ、migration report、test artifact、diagnostic trace に URL 本体を出さない。
report には count と privacy marker だけを出す。

offline repair:

- daemon active なら拒否する。
- exclusive store lock を取れなければ拒否する。
- semantic mutation はしない。
- repair は plan/apply の二段階にする。
- repair result は repair log に残す。

許可する repair:

- `CURRENT` を valid generation に戻す。
- broken staging を quarantine する。
- checksum 不一致 generation を quarantine する。
- legacy state を quarantine する。
- manifest index を再生成する。ただし generation content は immutable。
- migration retry。

禁止する repair:

- DesiredWorld を編集する。
- Layout を補正する。
- SavedURLs を消す/足す。
- LiveWindowID を別 window に割り当てる。
- browser snapshot から DesiredWorld を再生成する。

retention:

- latest N generations を保持する。初期 default は 10。
- current generation は GC しない。
- active repair/migration metadata が参照する generation は GC しない。
- GC は generation parent chain を壊さない。

PersistentStore の実装条件:

1. generation directory store であること。
2. committed generation が immutable であること。
3. Controller commit だけが committed generation を作れること。
4. `.staging` から `generations/` への rename と `CURRENT` rename を commit boundary にすること。
5. manifest が schema versions / parent / checksums を持つこと。
6. journal が replay authority ではなく audit-only であること。
7. production/test store identity marker が必須であること。
8. legacy migration が whitelist/quarantine 方式であること。
9. `SavedURLs` が private data として扱われること。
10. offline repair が semantic mutation をできないこと。

### 難所

- archive/unarchive のタイミングで browser snapshot と DesiredWorld 更新を同一 transaction として扱う必要がある。
- crash 中断時に checkpoint と desired の整合性が崩れないようにする必要がある。
- layout snapshot は観測事実ではなく、明示 accept 後に accepted semantic layout として保存する必要がある。
- 既存 `cmd/state repair` 相当の手動修復が新 store schema を壊さないよう、repair は migration-aware にする必要がある。

## 5. Controller event loop

### 判断

外部 event は DesiredWorld を直接変更しない。
event は `EventReaction` に変換する。

```text
Event
  -> ReactToEvent
  -> DirtyScopes / ObserveScopes / LifecycleTransaction
  -> ObserveWorld
  -> Plan
  -> Execute
  -> Settle
  -> Verify
```

### 実装判断

- controller は single goroutine / serialized queue を基本にする。
- transaction 中に来た event は queue に積むか coalesce する。
- wake/display change は window/layout/focus event を supersede する。
- controller-origin event は settle 中の evidence として扱う。
- user-origin layout event は manual-layout candidate にするが、明示 accept なしに DesiredWorld へ保存しない。

event priority:

1. shutdown
2. startup
3. wake
4. display-changed
5. layout-changed
6. window-changed
7. periodic

coalescing:

- `wake` / `display-changed` は後続または既存の `window/layout/focus` dirty を supersede する。
- `layout-changed` は debounce して manual-layout candidate を 1 つに集約する。
- `window-changed` は workspace / app scope 単位で coalesce する。
- `periodic` は backlog がある場合 skip する。
- stale epoch の event は捨てるか evidence として trace に残す。

transaction failure:

- stale epoch なら discard。
- display change を伴うなら transaction を abort して display lifecycle transaction から restart。
- 同種 intent は latest-wins。
- verifier diff が許容不能なら bounded replan し、上限到達で Dirty/Unsupported として報告する。

`projwmctl`:

- direct mutation しない。
- `projwmd` に intent を送る client にする。
- read-only status/query は許可する。
- daemon 不在時の direct mutation fallback は持たせない。

### Single writer / IPC 境界の固定

この節は実装前 blocking の 2 つ目の決定である。
通常運用における GUI / window / session / browser / desired-state mutation は `projwmd` だけが行う。

#### Single writer rules

| rule | 内容 |
|---|---|
| WRITER-1 | `projwmd` だけが window/session/browser/store desired-state を変更できる |
| WRITER-2 | `projwmctl` は intent client であり、store や adapter を直接変更しない |
| WRITER-3 | daemon 不在時でも `projwmctl` は direct mutation fallback しない |
| WRITER-4 | sidecar / watcher / launchd agent は mutation を持たない |
| WRITER-5 | test/admin/migration も raw file patch ではなく gated controller intent または store API を通す |
| WRITER-6 | 例外は named recovery mode に限定し、GUI/window/session/browser mutation をしない |

禁止:

- CLI command が store を直接 mutate してから reconcile を呼ぶ。
- daemon 不在時に `projwmctl up/add/profile/archive/jump` が adapter を直接叩く。
- watcher が `reconcile` / `layout-snapshot` / `repair` を実行する。
- setup script が `state.json` / next store を raw patch する。
- test fixture が store file を直接書く。

#### IPC transport

初期実装の mutation IPC は Unix domain socket にする。
HTTP/gRPC のような network transport は不要であり、local user daemon の authority を曖昧にしやすい。

socket path:

- authority は managed environment manifest または launchd generated environment に固定する。
- production mutation path では任意 `--socket-path` を許さない。
- developer/test mode で socket path を変える場合は、test manifest + test store + test mode が揃う場合だけ許可する。

protocol:

- handshake で protocol version / daemon version / managed environment generation / store schema version を確認する。
- mismatch は typed error で拒否する。
- mutation intent は request ID / accepted transaction ID / committed generation / final result を返す。
- 「送信できた = 成功」扱いにしない。

error taxonomy:

| error | 意味 |
|---|---|
| `socket-absent` | expected socket が存在しない |
| `connection-refused` | socket はあるが daemon が受けない |
| `timeout` | daemon が応答しない |
| `daemon-busy` | transaction 中で intent を受理できない、または queue 制限 |
| `protocol-mismatch` | client/daemon protocol 不一致 |
| `intent-rejected` | precondition / authority / validation で拒否 |
| `transaction-failed` | intent は受理されたが transaction が失敗 |
| `unsupported` | この daemon/backend では実行不能 |

#### daemon 不在時の挙動

daemon 不在時、mutation command は失敗する。
僕の判断では、mutation command が暗黙 auto-start して続行する挙動も最初は入れない。

理由:

- daemon 起動は environment validation / store recovery / lifecycle bootstrap を伴う。
- `projwmctl up` などが暗黙に daemon 起動から mutation まで進むと、失敗点と責務が曖昧になる。
- 明示 `projwmctl daemon start` のあと、通常 IPC intent を送る方が trace と UX が明確である。

許可:

- `projwmctl daemon start`: daemon 起動を依頼する。mutation はしない。
- `projwmctl status --offline`: last committed store snapshot だけを見る。
- `projwmctl store validate --offline`: schema/invariant の read-only 検査。

禁止:

- daemon 不在を理由に store API へ direct write する。
- daemon 不在を理由に WM/browser/session adapter を直接呼ぶ。
- `status --offline` が live GUI 状態を名乗る。

daemon absence error は以下を表示する:

- expected socket path。
- socket absence/refused/timeout の区別。
- launchd label / `projwmctl daemon start` / rebuild が必要な場合の案内。
- offline read-only command が可能ならその案内。

#### sidecar limits

sidecar が送れるのは `EventHint` だけである。

sidecar がしてよいこと:

- OS event / wake / display / external hook を受ける。
- `projwmd` へ event hint を送る。
- daemon 不在時に event を drop する、または bounded retry queue に入れる。

sidecar がしてはいけないこと:

- store を読む。
- store を書く。
- adapter を呼んで window/session/browser を mutate する。
- reconcile / layout / repair / migration を実行する。
- daemon 不在時に補償 mutation する。

既存からの置換:

| 既存要素 | next |
|---|---|
| `projwm-reconcile-watch` | daemon 内 event source または EventHint sidecar |
| `projwm-reconcile-display` | daemon display lifecycle event |
| `projwm-reconcile-periodic` | daemon 内 safety timer |
| `projwm-reconcile-startup` | daemon `LifecycleBootstrap` |
| `projwm-reconcile-wake` | wake EventHint + daemon `LifecycleWakeRecovery` |
| `projwm-layout-watch` | daemon 内 layout event source。manual layout candidate 生成だけ |

#### test / admin / migration gates

test/admin/migration は single writer の例外ではない。
これらは **gated intent source** であり、production path から機械的に隔離する。

test:

- `projwmd --test-mode` でのみ test-only intent を有効化する。
- test mode は test manifest + isolated test store + test socket が揃わないと起動拒否する。
- `IntentLoadFixture` / `IntentResetWorld` は production daemon では rejected。
- fixture load は Controller transaction と PersistentStore API を通す。
- production store path では test mode 起動を拒否する。

admin:

- `projwmctl admin ...` も daemon IPC 経由。
- dangerous intent は explicit admin flag / audit log / typed result を必須にする。
- admin intent は DesiredWorld や accepted snapshot の commit boundary を迂回しない。

migration:

- migration は raw JSON patch ではなく store API を通す。
- daemon 停止中の one-shot migration、または daemon startup migration のどちらかに固定する。
- 初期実装の推奨は daemon 停止中の one-shot migration。
- migration tool は GUI/window/session/browser mutation をしない。
- migration result は quarantine report と generation commit として残す。

#### read-only command

read-only には live read と offline read の 2 種類がある。
混ぜてはいけない。

| command kind | daemon | 読んでよいもの | 注意 |
|---|---|---|---|
| live status/query | required | daemon controller status, live observation, queue, environment validation | daemon 経由のみ |
| offline status | not required | last committed store generation | stale warning 必須 |
| offline validate | not required | store schema/invariant | mutation 禁止 |

`projwmctl status` は daemon required。
daemon 不在で読む場合は `projwmctl status --offline` のように明示する。

#### emergency repair

任意の manual direct edit mode は存在させない。

禁止:

- `projwmctl state edit`。
- raw JSON edit を案内する recovery flow。
- setup script による store patch。
- daemon kill -> raw patch -> restart を正規手順にすること。

許可する emergency repair は **offline store recovery primitive** に限定する。

許可操作:

- validate。
- rollback to generation。
- quarantine corrupted generation。
- rebuild index。
- retry schema migration。
- export diagnostic bundle。

条件:

- daemon が active なら拒否する。
- GUI/window/session/browser mutation を一切しない。
- store API の recovery primitive だけを使う。
- recovery result を journal/audit log に残す。

推奨 command surface:

```text
projwmctl store validate --offline
projwmctl admin rollback-store --to-generation <id>
projwmd --recover
```

`projwmd --recover` は event loop を開始せず、adapter を呼ばず、store validate/rollback/quarantine だけを行って終了する。

#### Single writer の実装条件

Single writer 境界が満たされるまで、real backend mutation には進まない。

最低条件:

1. normal mutation command が daemon IPC intent だけを使うこと。
2. daemon absence で direct mutation fallback が存在しないこと。
3. IPC protocol/version/error taxonomy があること。
4. sidecar が EventHint しか送れないこと。
5. `state edit` / raw repair / raw setup patch が next の正規 path に存在しないこと。
6. test fixture が gated controller intent であること。
7. migration が store API を使い、daemon-exclusion rule を持つこと。
8. offline status が stale/read-only と明示されること。
9. emergency repair が constrained offline store recovery に限定されること。
10. production/test/admin gate が convention ではなく機械的に enforce されること。

### 難所

- event storm と replan loop の停止条件。
- stale epoch の扱い。
- settle 中に新しい display change が来た場合の transaction abort/restart。
- 既存 `up/add/profile/archive` は state 直接更新が多いため、intent 化の移行境界を慎重に切る必要がある。

## 6. Observer / adapter

### 判断

Observer は mutation しない。
adapter は外部 system の癖を隠すが、truth を捏造しない。

初期 adapter:

| adapter | 最初に必要な責務 |
|---|---|
| window manager | windows/workspaces/focus/layout/display observation |
| session | tmux/session existence observation |
| browser | browser window/tab/profile observation and snapshot |
| app | bundle/app/process observation |
| system | wake/display/startup event source |

### adapter 実装時の検証方針

- `omniwmctl query` は windows/workspaces/displays/rules/capabilities/subscriptions を取得できるため、Observer の主入力にする。
- `omniwmctl watch` は event source として使うが、payload/順序/欠落/重複は信用しない。
- display ID が不安定でも、display lifecycle transaction は full observe + target 再計算で吸収する。
- browser internal ID と LiveWindowID は stale 前提で rematch し、永続 identity にしない。
- Ghostty/cmux/Zed は title を identity にせず、bundle/session/project/title hint の組み合わせで照合する。

### identity / rematch 実装判断

`MatchHint` は観測証拠の優先度付き集合として扱う。
title は identity ではなく、補助証拠または contract drift の検出に使う。

rematch 優先順位:

| 種別 | 主証拠 | 補助証拠 | 失敗時 |
|---|---|---|---|
| browser | `LiveWindowID` / browser window ID | bundle ID + profile + title | orphan / snapshot restore candidate |
| Ghostty/cmux | DesiredWindowID 由来の stable ordinal | bundle ID + title prefix/regex | missing managed window |
| Zed | project root | bundle ID + basename(title) | extra/orphan editor candidate |
| external | bundle ID | none | observed-only |

危険 primitive:

- `FocusWindow`
- `move-to-workspace`
- `move-column`
- stack member move
- blind `toggle-tabbed`

これらは executor が直接ばらばらに呼ばず、operation wrapper 経由で precondition / settle / verify とセットにする。

### Real mutation safety の固定

この節は実装前 blocking の 4 つ目の決定である。
real backend mutation は、resolver / operation wrapper / verifier / app contract がすべて safety 条件を満たす場合だけ許可する。

#### Resolver result

resolver は boolean を返さない。
必ず構造化 result を返す。

```go
type ResolveStatus string

const (
    ResolveUniqueStrong ResolveStatus = "unique-strong"
    ResolveMissing      ResolveStatus = "missing"
    ResolveAmbiguous    ResolveStatus = "ambiguous"
    ResolveWeakMatch    ResolveStatus = "weak-match"
    ResolveStale        ResolveStatus = "stale"
)

type ResolveResult struct {
    Status                ResolveStatus
    Candidates            []ResolvedCandidate
    Selected              *ResolvedCandidate
    Confidence            float64
    Evidence              []MatchEvidence
    MissingEvidence       []RequiredEvidence
    ForbiddenEvidenceUsed []MatchEvidence
}
```

mutation target に使ってよい resolver result:

```text
status == unique-strong
AND confidence == 1.0
AND candidate_count == 1
AND required evidence が全部揃っている
AND forbidden evidence を使っていない
```

それ以外はすべて mutation block:

- confidence < 1.0。
- candidate_count == 0。
- candidate_count > 1。
- fallback-only match。
- stale identity。
- title fallback。
- bundle-only match。
- last focused/frontmost による推測。

曖昧性は error ではなく safety state である。
Controller は `BlockedAmbiguous` として candidates/evidence/reason を report し、best-effort selection しない。

#### Evidence policy

mutation identity に使える strong evidence:

| app kind | required strong evidence |
|---|---|
| terminal/Ghostty | bundle ID == Ghostty、exact expected title、expected workspace、必要なら session-derived marker |
| editor/Zed | bundle ID == Zed、exact project/cwd-derived title、cwd exists、expected workspace |
| browser/Vivaldi | confirmed browser LiveWindowID / chrome-cli window id、OmniWM Vivaldi window correlation、expected workspace |
| external | default では strong evidence なし |

weak/advisory evidence:

- title fallback。
- SavedURLs-only browser match。
- BundleID-only match。
- “looks like this project”。
- last focused window。
- frontmost app。
- partial title contains。

weak/advisory evidence は diagnostics と verifier 補助には使えるが、mutation target selection には使わない。

browser rule:

- `LiveWindowID` confirmed: mutation candidate にしてよい。
- SavedURLs/title/bundle only: observe/report only。
- LiveWindowID なし browser は first implementation では read-only。

#### Semantic operation wrapper

危険 primitive を個別に wrap するだけでは不十分である。
wrapper は semantic operation 単位にする。

許可する wrapper unit:

- `MoveResolvedWindowToWorkspace`
- `SpawnProjectTerminal`
- `SpawnProjectEditor`
- `SpawnProjectBrowser`
- `RestoreProjectWorkspaceLayoutDryRun`
- `RestoreProjectWorkspaceLayoutReal` は first implementation では禁止。
- `CloseResolvedExtraWindow` は first implementation では禁止。

外部から直接呼ばせない primitive:

- `FocusWindow`
- `FocusWorkspace`
- raw `MoveWindowToWorkspace`
- `MoveDirection`
- `MoveColumnDirection`
- `ToggleColumnTabbed`
- `CloseWindow`

全 mutation wrapper は同じ形にする:

```text
Precondition(read-only)
Resolve(read-only)
Execute(mutation)
Settle(read-only polling)
Verify(read-only strict)
Commit(optional state/cache update)
```

commit gate:

- Precondition fail: execute しない。
- Resolve が `unique-strong` 以外: execute しない。
- Execute fail: commit しない。
- Settle timeout: commit しない。
- Verify fail: commit しない。
- Verify fail: dependent mutation を止める。

Executor は hidden retry / replan をしない。
retry/replan は Controller が read-only 再観測・再解決した上で判断する。

#### Focus / move race prevention

focus-dependent WM mutation は global `wmMutationLock` 内で semantic operation 全体を直列化する。

lock 対象:

- `FocusWindow`
- `FocusWorkspace`
- `MoveWindowToWorkspace`
- `MoveDirection`
- `MoveColumnDirection`
- `ToggleColumnTabbed`
- layout restore
- focus を変える snapshot
- focused workspace に依存する spawn

lock scope:

```text
lock
  query current world
  resolve target again
  precondition
  if already satisfied: no-op
  if unique-strong: execute
  settle by polling
  verify
  optional commit
unlock
```

lock 取得前に作った live resolution は信用しない。
lock 待ちの間に world は変わるため、lock 取得後に必ず再観測・再解決する。

fixed sleep は safety primitive ではない。
以下のような polling wait を使う:

- `WaitForFocusedWindow(targetID)`
- `WaitForWindowInWorkspace(targetID, workspace)`
- `WaitForUniqueResolverMatch(desiredID)`
- `WaitForChromeLiveWindowID(project)`

複数 process が mutation しうる構成では process-local mutex では足りない。
`projwmd` single writer が成立するまでは real mutation を増やさない。
必要なら file lock / daemon-only runner を併用する。

#### App minimum contracts

real app control 前に、app kind ごとの minimum contract を固定する。

```text
AppContract:
  app_kind
  identity_evidence_required
  spawn_preconditions
  settle_conditions
  mutation_target_rules
  verifier_rules
  stale_identity_rules
  ambiguity_rules
  allowed_mutations
  forbidden_mutations
```

terminal / Ghostty:

- Identity: bundle ID == Ghostty AND exact expected title。
- Spawn precondition: target workspace exists、tmux session exists or can be safely created、existing exact window が 0 or unique、ambiguous candidate なし。
- Settle: exactly one Ghostty window with expected title、expected workspace。
- Allowed: spawn、exact unique resolve 後の move。
- Forbidden: partial title move、BundleID-only move、multiple candidate best-effort。

editor / Zed:

- Identity: bundle ID == Zed AND exact project/cwd title AND cwd exists。
- Spawn precondition: cwd exists and is directory、existing exact window が 0 or unique、ambiguous candidate なし。
- Settle: exactly one Zed window with expected title、expected workspace。
- Allowed: spawn、exact unique resolve 後の move。
- Forbidden: weak title mutation、multiple Zed best-effort、title changed fallback mutation。

browser / Vivaldi:

- Identity: LiveWindowID / chrome-cli window id が primary。
- Supporting evidence: marker/SavedURLs confirms project association、OmniWM Vivaldi live window correlation。
- Spawn precondition: existing LiveWindowID missing or verified stale、marker strategy available、confirmed existing browser window なし、ambiguous browser candidate なし。
- Settle: chrome-cli returns LiveWindowID、marker/SavedURLs confirms association、OmniWM sees correlated Vivaldi window、expected workspace。
- Allowed: marker spawn、LiveWindowID-confirmed move。
- Forbidden: BundleID-only move、title fallback move、SavedURLs-only move、unknown LiveWindowID browser move。

external:

- default は observe-only。
- list/report/diagnostics は許可。
- move/close/layout restore participation/spawn automation/title fallback mutation は禁止。

#### Verifier gating

Verifier は warning logger ではなく commit gate である。

post-mutation Verify failure:

- operation failure。
- state commit 禁止。
- cache update 禁止。
- layout snapshot update 禁止。
- subsequent dependent mutation block。

strict verifier と advisory verifier を分ける。
real mutation path で advisory verifier だけを通すことは禁止する。

move verifier:

- same resolved window identity still exists。
- window is now in desired workspace。
- duplicate candidate が出ていない。
- resolver still returns unique-strong。

spawn verifier:

- expected app appeared。
- expected identity evidence appeared。
- exactly one candidate。
- expected workspace。

layout restore verifier:

- first implementation では full layout restore real mutation は禁止し、dry-run にする。
- 将来 real restore する場合、aggregate verification が必須。
- missing/window mismatch/frame-group mismatch/duplicate があれば restore failed とし、state commit 禁止。

#### First implementation allow/block matrix

| operation | first implementation |
|---|---|
| Query windows/workspaces/layout | allow |
| Resolver dry-run | allow |
| URL snapshot | allow, privacy policy required |
| tmux session existence check | allow |
| tmux session create if absent | allow with exact session identity |
| Ghostty spawn | conditional allow |
| Zed spawn | conditional allow |
| Vivaldi spawn | conditional allow, strict |
| Move Ghostty/Zed to workspace | conditional allow |
| Move Vivaldi to workspace | only with LiveWindowID-confirmed identity |
| Move external app | block |
| Close window | block |
| Full layout restore | block / dry-run only |
| Frame grouping mutation | block |
| Exposed FocusWindow API | block |
| Best-effort resolver mutation | block |

Real mutation safety の実装条件:

1. confidence < 1.0 は mutation block。
2. candidate_count != 1 は mutation block。
3. browser mutation requires LiveWindowID。
4. title fallback is never mutation identity。
5. SavedURLs-only is never mutation identity。
6. external app mutation is forbidden until explicit contract exists。
7. all focus-dependent mutation uses global WM mutation lock。
8. cross-process mutation は `projwmd` single writer または file lock で防ぐ。
9. verifier failure blocks state/cache/layout commit。
10. first implementation blocks full layout restore real mutation。

browser:

- `LiveWindowID` は再起動を跨ぐ identity にしない。
- browser internal ID は stale 検出できる場合だけ強証拠にする。
- snapshot-managed な browser は、通常操作中の tab/URL drift を violation にしない。
- snapshot-managed の既定対象は browser structure であり、raw URL/title content ではない。
- raw URL/title/SavedURLs は private payload として扱い、PersistentStore / logs / reports / traces / tests へ直接出さない。

### Browser privacy / observability の固定

この節は実装前 blocking の 5 つ目の決定である。
Browser は first-class sub-world だが、browser content は private by default である。

最重要の言い換え:

```text
Browser structure is snapshot-managed by default.
Browser content may be attached as private payload only under explicit privacy policy.
```

「通常 browser content は snapshot-managed」とは言わない。
URL/title/tab content を通常 snapshot field にすると、state/log/test/diagnostics へ漏れるためである。

#### Data categories

browser observation は最低 3 種類に分ける。

| category | 例 | default persistence |
|---|---|---|
| structural browser state | window count, tab count, active tab index, pinned flag, profile ref, workspace assignment | PersistentStore に保存可 |
| sensitive browsing content | raw URL, title, favicon URL, search query, fragment, referrer, page text, file URL, local service URL | PersistentStore に直接保存不可 |
| control handles | LiveWindowID, BrowserWindowID, BrowserTabID, PID | durable identity にしない。runtime observation only |

#### Snapshot modes

```go
type BrowserSnapshotPrivacyMode string

const (
    BrowserSnapshotStructureOnly  BrowserSnapshotPrivacyMode = "structure-only"
    BrowserSnapshotRedactedContent BrowserSnapshotPrivacyMode = "redacted-content"
    BrowserSnapshotPrivateContent BrowserSnapshotPrivacyMode = "private-content"
)
```

`structure-only`:

- default。
- windows/tabs の構造だけ保存する。
- raw URL/title は保存しない。

`redacted-content`:

- scheme/category、origin HMAC、path depth、query/fragment の有無など、非可逆 descriptor だけ保存する。
- HMAC は raw SHA256 ではなく local keyed HMAC を使う。
- path/query/fragment の HMAC は opt-in。

`private-content`:

- raw URL/title を PrivatePayloadStore に保存し、PersistentStore には opaque ref だけ置く。
- explicit policy/consent が必要。
- diagnostics/export には既定で含めない。

#### Types

browser adapter は raw URL/title を plain string として汎用 logging path に渡さない。

```go
type SensitiveURL struct {
    Ref       *PrivatePayloadRef
    Safe     BrowserContentDescriptor
}

type SensitiveTitle struct {
    Ref       *PrivatePayloadRef
    Safe     BrowserContentDescriptor
}

type BrowserContentDescriptor struct {
    Class       URLClass
    OriginHMAC  *string
    PathDepth   *int
    HasQuery    bool
    HasFragment bool
    TitleBucket *string
}

type PrivatePayloadRef string
```

`SensitiveURL` / `SensitiveTitle` は default string conversion を持たない。
log/report/test へ出す場合は `Safe` descriptor または `[redacted-url]` / `[redacted-title]` だけを使う。

PersistentStore に置いてよい browser snapshot:

- browser world id。
- schema version。
- profile opaque ref。
- window count / tab count。
- active tab index。
- pinned/muted/loading flags。
- tab structural snapshot id。
- privacy mode。
- redaction policy version。
- opaque private payload refs。
- aggregate counts。

PersistentStore に置いてはいけないもの:

- raw URL。
- raw title。
- raw favicon URL。
- page content。
- query string。
- fragment。
- referrer。
- auth headers。
- cookies。
- localStorage / sessionStorage。
- screenshot / thumbnail。
- raw chrome-cli output。
- raw `SavedURLs`。
- `file://` の raw filesystem path。
- username 等を含む browser profile path。

PersistentStore は PrivatePayloadStore がなくても load できる。
private payload が欠けている場合、window/tab structure は読めるが URL reopen は行わない。

#### PrivatePayloadStore

PersistentStore と PrivatePayloadStore は別 contract にする。

PersistentStore:

- safe to inspect/share by default。
- raw browser content を含まない。
- private payload ref は opaque。

PrivatePayloadStore:

- safe to share ではない。
- raw browser content を持ちうる。
- git/diagnostics から既定で除外する。
- file permission を制限する。
- 可能なら OS keychain / local encryption key を使う。
- retention policy を持つ。

private payload metadata:

```go
type PrivatePayloadMetadata struct {
    ID               PrivatePayloadRef
    Class            PrivatePayloadClass
    CreatedBy        string
    CreatedAt        time.Time
    RetentionPolicy  PrivateRetentionPolicy
    UserConsentLevel ConsentLevel
    EncryptionStatus EncryptionStatus
}
```

retention:

- `session-only`
- `until-snapshot-deleted`
- `persistent-private`
- `migration-archive`

#### Browser privacy policy

```go
type BrowserPrivacyPolicy struct {
    Version                    int
    ObserveBrowserStructure     bool
    ObserveBrowserContent       bool
    PersistStructure            bool
    PersistPrivateURLPayload    bool
    PersistPrivateTitlePayload  bool
    LogContentDescriptors       DescriptorLevel
    DiagnosticsContent          DiagnosticsContentMode
    RestoreURLsFromPrivatePayload bool
    MutateLiveBrowserWindows    bool
    IncludeIncognito            bool
}
```

default:

```text
observeBrowserStructure = true
observeBrowserContent = false
persistStructure = true
persistPrivateURLPayload = false
persistPrivateTitlePayload = false
logContentDescriptors = scheme-only
diagnosticsContent = redacted
restoreURLsFromPrivatePayload = false
mutateLiveBrowserWindows = false
includeIncognito = false
```

consent は分ける:

- browser structure を観測する consent。
- browser content を観測する consent。
- browser structure を永続化する consent。
- raw URL/title を private payload として永続化する consent。
- private payload から URL を reopen する consent。
- live browser window/tab を mutate する consent。
- diagnostics に browser 情報を含める consent。

URL restore は layout restore より強い consent が必要。
opening URLs can reveal private context and trigger network requests.

#### Observed vs snapshot-managed

`chrome-cli` で見えた tab は自動で snapshot-managed にならない。

```go
type BrowserTabManagement string

const (
    BrowserTabSnapshotManaged BrowserTabManagement = "snapshot-managed"
    BrowserTabObservedOnly    BrowserTabManagement = "observed-only"
    BrowserTabIgnored         BrowserTabManagement = "ignored"
    BrowserTabBlockedPrivate  BrowserTabManagement = "blocked-private"
)
```

- `snapshot-managed`: structure persistence と、policy が許す private payload ref のみ。
- `observed-only`: diagnostics/control のために観測できるが content persistence はしない。
- `ignored`: 必要最小限の control boundary 以外は見ない。
- `blocked-private`: incognito/private/sensitive surface。content snapshot 禁止。

incognito/private window は default で count only。
structure/content persistence はしない。

special URL handling:

- `chrome://`
- `about:`
- `extension://`
- `file://`
- `localhost`
- `127.0.0.1`
- private network addresses

これらは safe ではない。
default では classify and redact する。

#### Redaction / diagnostics / tests

raw URL/title は default で出力禁止:

- logs。
- traces。
- debug messages。
- error messages。
- panic messages。
- test snapshots。
- golden files。
- migration reports。
- CLI output。
- diagnostics bundles。
- crash reports。
- PersistentStore diffs。
- PR/CI artifacts。

migration report は aggregate only:

```text
SavedURLs:
  discovered: 24
  migratedToPrivatePayload: 24
  skippedInvalid: 2
  committedRawUrls: 0
```

tests:

- committed fixtures は `example.test` / `invalid.test` / `browser-fixture.test` など synthetic URL だけを使う。
- golden は raw URL/title ではなく redaction を assert する。
- leak test を入れる。`SHOULD_NOT_APPEAR` を含む URL/title を流し、log/error/report/store/test artifact に出ないことを検査する。

diagnostics:

- default は counts / redacted descriptors / policy version のみ。
- raw export は explicit one-shot consent がある場合だけ。
- raw artifact は private と明示し、共有禁止・削除案内を出す。
- 可能なら scoped reveal を使い、全 URL dump を避ける。

#### Migration handling

legacy `state.json` に raw `SavedURLs` がある場合、その file は sensitive input と見なす。

rules:

- raw values を print しない。
- migration error に raw values を含めない。
- PersistentStore に raw URL/title を直接 commit しない。
- raw values は PrivatePayloadStore へ移すか、policy がなければ skip/quarantine する。
- legacy file は explicit user approval なしに raw content を rewrite/delete しない。
- migration report は count と privacy marker だけにする。

僕の判断:

- PersistentStore は inspect/share safe by default にする。
- raw browser content は PrivatePayloadStore にだけ置く。
- migrated SavedURLs は private payload に移せるが、自動 reopen しない。
- raw title は URL persistence が有効でも default persist しない。title payload は別 opt-in。
- diagnostics は default redacted。raw export は one-shot explicit consent がある場合だけ。
- incognito/private windows は default count-only。

Browser privacy の実装条件:

1. PersistentStore に raw URL/title が入らない。
2. PrivatePayloadStore が PersistentStore と分離されている。
3. browser adapter が raw URL/title plain string を汎用 logging path に渡さない。
4. migration report/log/test/trace が aggregate/redacted only である。
5. URL restore は explicit consent なしに実行しない。
6. observed tabs は自動 snapshot-managed にならない。
7. incognito/private は default count-only。
8. leak tests が canary URL/title の漏れを検出する。

Ghostty/Zed:

- 既存は title 完全一致に依存しているため、`TitleContract` は migration 上重要。
- Ghostty は titleRegex が OmniWM admission の安全条件になっている。
- Zed は `basename(title)` が補助証拠になるが、empty project / restore window は orphan として扱う。

## 7. Planner / Simulator / Executor / Settler / Verifier

### 判断

分離は維持する。
Simulator は十分に賢くしてよい。
ただし賢さの対象は semantic world / operation effect / risk estimation に限定し、pixel geometry や WM の非決定性を捏造しない。
初期実装では汎用探索 planner から始めないが、Simulator を使った plan evaluation へ発展できる形にする。

この章は以前の版では曖昧だった。
名前を並べるだけだと、既存 `reconcile` のように「差分検出・計画・実行・待機・検証」が再び混ざる。
ここでは各コンポーネントが **何をしてよくて、何をしてはいけないか** を固定する。

実装方針:

- Planner は deterministic rule-based planner から始めるが、Simulator で plan evaluation できる形にする。
- Simulator は semantic world に対して賢く予測する。
- Executor は 1 operation ずつ副作用を出す。
- Settler は operation ごとの安定条件を待つ。
- Verifier が predicted/observed/desired の差分を分類する。
- retry / replan の所有者は Controller であり、Executor や Settler は勝手に次の操作を決めない。

責務境界:

| 部品 | 入力 | 出力 | やってよいこと | 禁止 |
|---|---|---|---|---|
| Planner | WorldState, target DesiredWorld, DirtyScopes | Plan | deterministic な Operation 列を作る | observe / execute / sleep / store write |
| Simulator | PredictedWorld, Operation | PredictedWorld, uncertainty, risk | operation の宣言効果・副作用・制約違反を model に反映 | 観測していない現実に合わせて都合よく補正する |
| Executor | Operation, resolved live refs | ExecutionResult | 1 operation の副作用を実行 | retry/replan/別 operation 実行 |
| Settler | Operation, settle policy | ObservedWorld / timeout | 期待状態が観測されるまで読む | mutation |
| Verifier | desired, predicted, observed, invariants | VerificationResult | diff を分類する | mutation / replan 実行 |

Plan:

```go
type Plan struct {
    ID         PlanID
    BaseEpoch  Epoch
    Scope      []WorldScope
    Operations []Operation
    Reason     PlanReason
}

type Operation struct {
    ID              OperationID
    Kind            OperationKind
    Scope           []WorldScope
    Target          OperationTarget
    Preconditions   []Precondition
    ExpectedEffects []Effect
    Settle          SettlePolicy
    Risk            RiskClass
    IdempotencyKey  string
}
```

Operation 設計:

- Operation は「副作用 + 期待効果 + settle 条件」を必ず一緒に持つ。
- `FocusWindow` / move 系 / summon 系は単独 primitive として外へ出さない。
- live ID が必要な operation は、実行直前に resolver が Desired ID から Live ID へ解決する。
- resolver が曖昧なら executor に渡さず、Verifier/Controller が Dirty/Unsupported として扱う。
- operation はできるだけ idempotent にするが、WM 操作は完全 idempotent とは仮定しない。

Planner:

- 初期実装は search ではなく rule-based。
- dirty scope がない全世界再構築 planner を最初から作らない。
- profile switch / archive / unarchive / reconcile / lifecycle ごとに plan builder を持つ。
- 候補 plan が複数ある場合は Simulator で評価し、risk / operation count / uncertainty が低いものを選ぶ。
- 将来的に bounded search を入れる場合も、Simulator の semantic state 空間に限定する。
- column budget で詰まった場合、runtime で環境値を書き換えず、environment mismatch / suggested Nix diff にする。

Simulator:

- existence / workspace / focus / semantic layout / stack capacity / column budget / known side effect を予測する。
- operation sequence の前後関係を見て、危険な順序を検出する。
- spawn / browser / app-owned title のような非同期領域は pending/dirty として表現する。
- plan の risk score と uncertainty を返す。
- frame 座標や offscreen geometry を正確に予測しない。
- 予測不能な副作用は `Dirty` または `PredictedWithUncertainty` として明示する。
- simulator の予測は verifier で必ず観測と照合する。

Simulator fidelity:

| level | 目的 | 扱うもの | 扱わないもの |
|---|---|---|---|
| L0 effect simulator | operation の宣言効果を反映 | window existence / workspace / focus | layout search |
| L1 semantic layout simulator | plan evaluation | columns / stacks / budgets / side effects | exact frame |
| L2 recorded calibration | 実機 trace から risk を調整 | known unstable operation / timing tendency | truth の置き換え |

最初は L0 + 必要な L1 から始める。
「conservative」は賢くしない言い訳ではなく、分からないことを分からないと出す契約である。

Executor:

- adapter 呼び出しの唯一の所有者。
- precondition を実行直前にも確認する。
- 失敗を握りつぶさず typed error として返す。
- `FocusWindow + move-to-workspace` のような危険 sequence は 1 operation wrapper 内で直列化する。

Settler:

- sleep 固定ではなく、query polling + stable count を基本にする。
- timeout は operation ごとに持つ。
- settle 中に higher-priority lifecycle event が来た場合は Controller に abort/restart を要求する。

Verifier:

```go
type VerificationClass string

const (
    VerifyOK          VerificationClass = "ok"
    VerifyReplan      VerificationClass = "replan"
    VerifyDirty       VerificationClass = "dirty"
    VerifyUnsupported VerificationClass = "unsupported"
    VerifyReportOnly  VerificationClass = "report-only"
)
```

- diff を `ok/replan/dirty/unsupported/report-only` に分類する。
- replan してよいかは Controller が replan budget で決める。
- invariant violation と observation uncertainty を分ける。

Controller:

- transaction / retry / replan budget の所有者。
- bounded replan を超えたら Dirty/Unsupported として止める。
- Executor/Settler/Verifier に hidden retry を持たせない。

### 難所

- OmniWM operation の副作用が focus/layout/window movement にまたがる。
- `FocusWindow` は危険 primitive として限定利用する。
- layout operation は reversible ではない前提で扱う。
- Simulator を賢くすること自体は必要。ただし exact WM clone を目指すと第二実装が生まれる。
- Planner を最初から探索にすると、状態空間と WM 副作用に飲まれる。
- Verifier が補正まで始めると、Planner/Executor と責務が衝突する。

## 8. E2E acceptance story

### 判断

projwm-next のテスト設計は test pyramid ではない。
完成判定は **real OmniWM / sigwm を通る Human-operation E2E acceptance story** だけで行う。
unit / integration / fake / simulator / recorded は実装補助、safety preflight、fixture validation、
diagnostics、failure reproduction として使ってよいが、acceptance の代替ではない。

Go story test を実装前から設計する。
custom DSL は作らない。

### 実装判断

- 実装順序は **Human-operation E2E acceptance harness first** とする。個別 stub を先に潰すのではなく、
  §3/S1-S8 全体を cover する human-visible executable contract を先に置く。
- harness の正規経路は
  documented `projwmctl` / keyboard / window manager operation / real app operation
  -> `projwmd` -> IPC -> Controller -> real OmniWM/sigwm -> visible observation -> restart-visible persistence
  である。
- `projwmctl` が `ok request=...` を返すだけでは合格ではない。Step 後の observed WorldState、
  OmniWM 状態、visible layout/focus/CLI output/restart 後の復元が invariant を満たして初めて合格である。
  test store generation、trace / transaction log は transaction property と failure diagnosis の補助 oracle に限る。
- Controller / Adapter / Reducer / Store を test が直接呼んで mutation する経路は E2E acceptance ではない。
- direct adapter mutation / raw state patch / hidden repair / MemoryStore success / fake adapter success を正規経路にしない。
- fake / simulator / recorded は acceptance の backend matrix ではなく、real E2E を安全に実行・診断するための補助である。
- real mutation preflight が満たされない Step は skip ではなく `FAIL_UNSAFE_TO_RUN` として赤く出す。
  これは「対象外」ではなく「acceptance blocked」である。安全条件を満たさないまま実機 mutation を実行してはならない。
- recorded trace は query/watch/transaction trace の replay と regression reproduction に使う。
  trace は debug artifact であり、test definition ではない。
- reset は test harness に寄せるが、production で再現不能な世界修復は禁止する。
- `state.json` 直接編集による reset は next の正規 story test では使わない。

### Canonical E2E story

E2E は S1-S8 を独立 scenario として大量に reset しながら走らせない。
中心は **1 本の canonical story** とし、同じ daemon / socket / test store と実 workspace `A/Q/W/E` を維持したまま、
ユーザーの作業日全体に近い順序で intent・manual operation・external event を連続実行する。

初期状態:

- production daemon / production socket / production store と競合しない。
- isolated test store dir を `--store-kind=test` で作成する。
- workspace は旧 projwm integration ideal state と同じ `A/Q/W/E` を直接使う。
- fixture は旧 ideal state の visible contract を採用する:
  - `A`: `[ai-view-1:dotfiles] [ai-view-1:projwm-jtest] [ai-view-1:MyEmmoWorld]`
  - `Q`: `[dotfiles(Zed)] [ai-1:dotfiles] [shell-1:dotfiles / shell-2:dotfiles stacked] [Vivaldi]`
  - `W`: `[projwm-jtest(Zed)] [ai-1:projwm-jtest] [shell-1:projwm-jtest]`
  - `E`: `[ai-1:MyEmmoWorld]`
- `A/Q/W/E` 以外の workspace は canonical story の reset / cleanup / layout assertion 対象にしない。
  関係ない workspace を見に行って理想状態へ寄せること自体を禁止する。
- ただし `A/Q/W/E` に非 test window が紛れ込んでいる場合は、それを削除せず spill workspace（既定 `3`、環境変数で変更可）へ退避してから test baseline を取る。
- `A/Q/W/E` 以外は test 前後で snapshot を比較し、内容が変化していないことを監査する。
  これは外部 workspace を管理するためではなく、test が関係ない workspace を巻き込んでいないことを証明するためである。
- window title は `projwm-e2e-*` のような人工 namespace へ逃がさず、旧 ideal state の実 title を使う。
- preflight で `A/Q/W/E` に旧 ideal state に準ずる残り滓 window があれば test residue として cleanup 対象にする。
  ただし cleanup してよいのは title / bundle / workspace が ideal state の managed window と一致するものだけであり、
  それ以外の user window が混ざっていれば `FAIL_UNSAFE_TO_RUN` として止める。
- `projwmd --test-mode --managed-environment <fixture> --store-dir <tmp> --store-kind test --socket-path <tmp>`
  を起動し、fixture load も raw file patch ではなく daemon command と lifecycle transaction で行う。
- Vivaldi は window placement / workspace mixing / archive-unarchive 対象として扱う。
  tab URL / cookie / token / session content は canonical ideal layout の期待状態には含めず、privacy auxiliary story で扱う。

canonical story の操作順:

1. daemon 起動直後の LifecycleBootstrap / validate-environment / reconcile を Human-operation path で実行し、
   ActiveProfile、slot assignment、desired windows、viewer order、layout、focus、DirtyScope empty を検証する。
2. `switch-profile A->B`、同一 profile 再投入、`B->A`、EmptyProfile への切替を連続実行する（S1）。
3. active project を archive し、reconcile で安定性を確認し、slot へ unarchive し、同 intent 再投入の idempotency を確認する（S2/S3）。
4. slot unassign、空 slot assign、reconcile repeated N を実行し、assignment / inactive policy / zero-mutation reconcile を確認する（S4/S5）。
5. 実機 OmniWM 操作で同一 workspace 内 column reorder / stack 変更を発生させる。accept 前の reconcile では DesiredWorld が変わらないこと、
   `accept-manual-layout` 後だけ layout（column order と stack membership を含む）が保存され、
   profile round-trip 後も維持されることを確認する（S6/S8.D）。
6. test-owned window の user close / cross-workspace move / timer full reconcile を発生させ、
   external event が DesiredWorld を直接変更せず、Controller transaction で再収束することを確認する（§4/S7/S8.E）。
7. 並行 `projwmctl` Intent を投入し、trace / transaction log 上で single writer と mutation overlap 不在を確認する（S8.A）。
8. 実 `A/Q/W/E` 上に ambiguous identity / predicted-observed divergence / stale epoch race を安全に構成し、
   unique-strong precondition、bounded replan no-commit、stale discard を Human-operation path で検証する（S8.B/S8.C/S8.F）。
   安全に構成できない場合は green にせず acceptance blocked とする。
9. browser project を archive / unarchive し、URL / cookie / token / raw title が PersistentStore、log、trace、
   `projwmctl` 出力に漏れないことを確認する（§6）。
10. cleanup は test-owned resource だけを対象にし、daemon command / adapter operation / lifecycle transaction で表現できる範囲に限定する。

補助 story:

- physical lifecycle story: 実 sleep/wake、display reconfigure など、canonical story に混ぜると実環境を乱すもの。
- privacy stress story: browser session / PrivatePayloadStore / redaction を深掘りするもの。
- race story: stale epoch / delayed event / concurrent intent を canonical story で決定的に作れない場合の専用 story。

補助 story も Human-operation E2E であり、fake / simulator に置き換えて green にしてはならない。

failure classification:

| class | 意味 |
|---|---|
| `FAIL_NOT_IMPLEMENTED` | acceptance contract は存在するが実装が未完成 |
| `FAIL_INVARIANT` | Step 実行後の WorldState が §2 invariant に違反 |
| `FAIL_UNSAFE_TO_RUN` | real mutation preflight / single writer / resolver / privacy / fixture isolation が未成立 |
| `FAIL_FIXTURE_INVALID` | fixture / manifest / test environment が acceptance 条件を満たさない |
| `FAIL_OBSERVABILITY_GAP` | Human-operation real backend で必要な観測がまだ取れない |
| `FAIL_PRIVACY_LEAK` | raw URL/title/token 等が store/log/trace/test artifact に出た |

`state.json` を直接編集しないだけでは安定にならない。
安定させる条件は、reset / fixture load も Controller transaction と PersistentStore API を通ることである。

story reset 方針:

| 対象 | reset 方法 | raw state file edit |
|---|---|---|
| canonical real E2E | isolated store dir + 実 workspace `A/Q/W/E` + `projwmd --test-mode --managed-environment <fixture> --store-dir <tmp>` で起動し、admin/test fixture command と lifecycle transaction で収束 | 禁止 |
| auxiliary real E2E | canonical real E2E と同じ。ただし physical lifecycle / privacy / race の追加 preflight を満たす場合だけ実行 | 禁止 |
| fake / simulator / recorded diagnostics | real E2E の failure reproduction / preflight validation としてのみ利用 | 禁止 |

fixture load:

- fixture は Go code か JSON fixture から DesiredWorld を作る。
- fixture は raw file patch ではなく、test-mode daemon の admin/test command から `PersistentStore.SaveDesiredWorld` 相当の API を通す。
- real E2E では running controller を quiesce し、`IntentLoadFixture` のような admin/test intent で checkpoint と desired を同一 transaction ID に揃える。
- fixture load 後に `LifecycleBootstrap` 相当の transaction を走らせ、ObservedWorld と DesiredWorld を一致させる。
- production で存在しない秘密の修復操作は使わない。

なぜ直接編集を禁止するか:

- schema validation / migration / checkpoint / epoch を迂回する。
- running daemon と競合して split-brain になる。
- fixture が production invariant を満たしているか分からない。
- test harness が第二の implementation になる。

real E2E の安定条件:

- user の通常 store を使わない。
- test 専用 store directory を使う。
- workspace は実 `A/Q/W/E` を使う。window title は旧 ideal state の実 title を使う。
- real E2E は `A/Q/W/E` 以外の workspace を reset / cleanup / layout assertion / recovery の対象にしない。
  外部 workspace は before/after snapshot で「変化していないこと」だけを監査する。
  `A/Q/W/E` に紛れた非 test window は、削除せず spill workspace へ退避してから baseline を取る。
- reset は raw patch ではなく controller transaction で行う。
- GUI cleanup は adapter operation と lifecycle transaction で表現できる範囲に限定する。
- production daemon / legacy agent と同時に `A/Q/W/E` を操作しない。
- cleanup 対象は旧 ideal state に準ずる test residue window だけに限定する。
- test-owned identity が unique-strong でない対象へ destructive mutation を出さない。
- unsafe / unobservable / unconstructible な Step は skip せず acceptance blocked とする。

S8 E2E 表現:

| Step | Human-operation E2E での観測方法 | blocked 条件 |
|---|---|---|
| S8.A single writer | concurrent `projwmctl` intent、trace / transaction log / mutation span で直列性を検査 | transaction trace が取れない |
| S8.B precondition unique-strong | 実 `A/Q/W/E` 上で ambiguous real candidates を作り、daemon が mutation を拒否したことを観測 | ambiguous fixture を安全に作れない |
| S8.C verifier replan | safe divergence を作り、bounded replan と no-commit を trace / store generation で検査 | divergence を安全・決定的に作れない |
| S8.D user-origin layout no-write | 実 OmniWM layout 操作、accept 前後の DesiredWorld / ManualLayoutCandidate を検査 | user-origin event を観測できない |
| S8.E external event no DesiredWorld write | window-manager/system/timer event 後の DesiredWorld 不変と DirtyScope / lifecycle trace を検査 | physical event が危険、または event trace が取れない |
| S8.F stale epoch discard | delayed event race を作り、latest epoch の DesiredWorld / DirtyScope が変わらないことを検査 | stale event を Human-operation path で決定生成できない |

最初の invariant:

- DesiredWorld の ID 一意性。
- profile assignment の重複なし。
- project/window/session/browser reference の整合性。
- DesiredWindowID ordinal の安定性。
- ObservedWindow と DesiredWindow の rematch が一意か ambiguity を報告できること。
- canonical story の harness が `A/Q/W/E` 以外を mutate しないこと。
- external window を managed invariant に巻き込まないこと。

trace artifact:

```text
traces/<story-run-id>/
  meta.json
  events.jsonl
  snapshots/
    before.json
    after.json
  stdout.log
  stderr.log
```

trace に保存する live PID / window ID は replay/debug 用であり、永続 truth にしない。
必要なら privacy-sensitive な URL/title は redaction policy を通す。

### 難所

- recorded backend の記録粒度。
- real backend の reset 責務を production と test harness のどちらが持つか。
- sleep/wake/display change story をどう再現するか。
- 現行 integration test は focus 起点の matrix が大きすぎるため、next では real story を増やしすぎない。
- `setup_ideal_state.sh` は暫定移行用の heavy reset であり、next の正規 reset にはしない。
- reset が production controller transaction で表現できないなら、その reset は設計の外に漏れている疑いがある。

### 既存根拠

- 現行 integration test は live 実機 / sleep / focus 走査 / reset script に強く依存している。
- `setup_ideal_state.sh` は daemon kill、state patch、temp rule、app kill が混ざっており、test harness が第二の実装になっている。
- 既存 state validation test は desired/observed 整合性 invariant の出発点として使える。

## 9. 実装前に残った未解決リスク

以下は未調査のまま残すものではなく、実装判断で吸収した既知リスクである。
実装中はこの扱いから外れていないかを検証する。

| リスク | 扱い |
|---|---|
| `omniwmctl watch` の順序/欠落/重複保証 | watch は hint/evidence に限定し、query snapshot で再構成する |
| runtime で検証できない Nix manifest 項目 | `report` として未検証扱いにする |
| browser snapshot の URL/title privacy | trace/snapshot redaction policy を導入する |
| display stable ID | 不安定なら display lifecycle は full observe + target 再計算に寄せる |
| old `state.json` と new store の共存期間 | migration input としてだけ読み、二重書き込みしない |
| replan loop | bounded replan と Dirty/Unsupported 報告で止める |

## 10. 直近の実装順序

1. canonical real E2E story harness を作る。
2. documented `projwmctl` / keyboard / window-manager operation / real app operation -> `projwmd -> Controller -> real OmniWM/sigwm -> visible observation` の Human-operation real path を接続する。
3. fixture load / reset / cleanup を raw patch なしの daemon command + lifecycle transaction として実装する。
4. S1-S8 / §4 / §6 を canonical story と必要最小限の auxiliary story に割り当てる。
5. failure classification / real preflight を実装し、未実装・危険・観測不能を赤く出す。
6. Nix manifest 生成と runtime validation。
7. PersistentStore schema と legacy state migration。
8. Controller event queue / transaction trace / single writer instrumentation。
9. identity/rematch、semantic operation wrapper、verifier gating を実装する。
10. browser privacy / PrivatePayloadStore / leak test を E2E harness に接続する。
11. Human-operation real E2E acceptance を green にする。

この順で調査する理由:

- E2E acceptance contract が先にないと、個別 stub 潰しが完成定義から逸れる。
- Human-operation real path を最初から中心にしないと、実機検証が後付けの smoke になる。
- ただし authority / single writer / resolver / privacy / fixture isolation が成立しない real mutation は、
  実行せず `FAIL_UNSAFE_TO_RUN` として可視化する。
- fake / simulator は stub ではないが、完成 gate でもない。real E2E の補助である。

## 11. 実装着手条件

実装へ進む前に、この文書で固定した判断:

1. Nix manifest は JSON、Nix store 生成物、launchd 引数 `--managed-environment <path>` で `projwmd` に渡す、authority は `nix`。
2. store は directory 型、legacy `state.json` は migration input。
3. external event は DesiredWorld を直接変更しない。
4. `projwmctl` は intent client。
5. rematch は title identity ではなく MatchHint 優先度で行う。
6. dangerous WM primitive は operation wrapper 経由でのみ使う。
7. story test は canonical real Human-operation E2E を中心にし、補助 story は必要最小限に限る。
8. fake/simulator/recorded は acceptance の代替ではなく、preflight / diagnostics / failure reproduction の補助である。
9. real test は Human-operation E2E acceptance gate とし、heavy reset / hidden repair / raw state patch を正規化しない。

## 12. 客観的再点検の検証結果

ここは設計を正当化するためではなく、疑問を潰すための監査結果である。
各項目について、既存実装の根拠、放置した場合の失敗、僕の判断を明記する。

調査で確認した事実:

- 既存 `projwm` は `~/.config/projwm/config.toml` を読み、Nix でも同 path に配置している。コメント上は「safe to edit」で、loader は missing なら default、unknown field は警告扱いである。
- 既存 OmniWM は Nix 生成 JSON manifest から runtime deploy script が `~/.config/omniwm/settings.toml` を書き、display change watcher が deploy と kickstart を実行する。
- 既存 `projwm` には reconcile watch / display / periodic / startup / wake / layout-watch の複数 launchd actor がある。
- 既存 store は単一 `state.json` に対して flock + tmpfile + rename を持つが、複数ファイル transaction や directory fsync はない。
- 既存 CLI は `--state-dir` で store を差し替えられ、`state edit` / `state repair` で直接 state を触れる。
- 既存 browser は `SavedURLs` と `LiveWindowID` を `state.json` に保存し、`LiveWindowID` stale を前提に live window を再解決する。
- 既存 OmniWM adapter は `FocusWindow` / `move-to-workspace` を持ち、`MoveWindowToWorkspaceByName` は mutex と polling で race を抑えている。
- `queue/` は `.gitignore` 対象であり、この文書はそのままでは commit に乗らない。

| 領域 | 重大度 | 検証結果 | 僕の判断 |
|---|---|---|---|
| manifest scope | blocking | 現設計の manifest 例は layout/focus/workspace/app/daemon まで含む。これは environment contract としては妥当だが、DesiredWorld や runtime 操作結果を入れ始める余地がある。 | manifest は **Nix-owned static environment contract だけ**に限定する。profile assignment、project、browser snapshot、accepted layout は入れない。 |
| manifest validation | blocking | `omniwmctl query` で rules/workspaces/displays/windows は検証できるが、Nix 側の意図すべては runtime で完全検証できない。 | safety-critical かつ検証不能な項目は report-only ではなく **mutation transaction block** に寄せる。単なる観測不能 metadata だけ report。 |
| OmniWM deploy | blocking | `omniwm-display-watcher` は display change で settings deploy と OmniWM kickstart を実行する。`projwmd` も display lifecycle を持つなら二重主体になる。 | OmniWM deploy は **OmniWM settings 適用だけ**に限定し、projwm state/window/browser mutation をしない。display event は `projwmd` に集約し、deploy/kickstart 後に full observe する。 |
| existing config | blocking | 既存 `config.toml` は viewer/slot/terminal app path という next では Nix/environment contract 寄りの値を持つ。さらに「safe to edit」なので authority が曖昧。 | `projwm-next` では既存 `config.toml` を authority にしない。必要なら legacy import / developer override に降格し、通常 daemon は managed manifest だけを読む。 |
| store transaction | blocking | 既存 store の atomicity は単一 file rename まで。next の `desired.json` + `checkpoint.json` では片方だけ更新される crash がありうる。 | 複数ファイルを直接正本にしない。`generations/<txid>/...` を完全に書いて fsync し、最後に `CURRENT` pointer を atomic rename する方式を第一候補にする。 |
| store path | blocking | 既存 CLI は `--state-dir` で任意 store を使える。test が通常 store を指す事故を機械的に防げない。 | production default と test store を path/name/schema で機械的に分離する。real test daemon は通常 store path なら起動拒否する。 |
| migration | blocking | legacy state は `LiveWindowID`、`SavedURLs`、frame由来 layout、browser profile を含む。これをそのまま truth 化すると stale identity を輸入する。 | migration は whitelist 方式。Desired project/profile/window ordinal と opt-in browser snapshot だけ輸入し、LiveWindowID/frame/focus/current live state は破棄する。 |
| admin fixture | blocking | `IntentLoadFixture` は便利だが、既存 `state edit` / setup script のような裏口修復が再発する危険がある。 | fixture intent は production build/normal daemon から機械的に隔離する。test mode flag、test store、test manifest が全部揃わないと拒否する。 |
| real test isolation | important | `--state-dir` で store は分離できるが、GUI window/process/OmniWM workspace は共有される。完全隔離はできない。 | real backend は acceptance matrix に最初から含める。ただし専用 namespace/title prefix、allowlist、production daemon 停止/拒否、cleanup transaction、`FAIL_UNSAFE_TO_RUN` を必須にする。 |
| simulator scope | important | 既存 layout は frame.x/y と focus 伝播に依存し、完全再現は危険。だが column/stack budget を無視すると plan evaluation が飾りになる。 | L0 + 最小 L1 から開始する。L1 は existence/workspace/focus/column-count/stack-count/known side effect までで、pixel/offscreen exact geometry は扱わない。 |
| planner candidates | important | rule-based planner だけで常に単一 plan だと Simulator は検証器にしかならない。 | 初期は候補を増やしすぎないが、危険 operation では `conservative` / `direct` など 2-3 candidate を出せる形にする。 |
| resolver ambiguity | blocking before real mutation | 既存は title 完全一致や単一 Vivaldi 窓 fallback がある。曖昧な照合で move/close すると復旧困難。 | resolver が同点/複数候補なら mutation 禁止。Verifier/Controller は Dirty/Unsupported として人間に見せる。 |
| app contracts | blocking before real app control | Ghostty は titleRegex、Zed は basename(title)、browser は window-id/title fallback、external は bundle だけと証拠の質が違う。 | 最初の real control 対象を絞り、app ごとに TitleAuthority / TitleDriftPolicy / MatchHint 優先度を固定してから mutation する。 |
| browser snapshot | blocking before persistence | 現行 `SavedURLs` は URL をそのまま state に保存する。next では browser を first-class にするため privacy impact が増える。 | browser snapshot は opt-in privacy policy を持つ。URL/title は snapshot 用と trace 用で redaction を分け、default trace は redact 寄りにする。 |
| event queue | important | 現行は launchd + debounce + lock file で外側から抑えている。next は single daemon 内に event storm が入る。 | queue は capacity、coalesce key、latest-wins、drop/skip policy を持つ。periodic は backlog 中 skip、wake/display は lower dirty を supersede。 |
| lifecycle settle | important | 現行は 150ms/300ms/3s/4s など fixed sleep が多い。sleep 復帰直後に stale observation になる懸念と一致する。 | fixed sleep only を禁止し、query polling + stable count + lifecycle-specific warmup を使う。閾値は実機 trace で調整する。 |
| IPC | blocking | 現行 CLI は daemon ではなく直接 store/adapter を触る。next で transport 未定のままだと direct mutation fallback が残る。 | `projwmctl` は Unix domain socket などの local IPC で daemon に intent を送る。daemon 不在時の direct GUI mutation fallback は持たない。 |
| manual user ops | blocking | 既存 OmniWM appRules は手動移動を戻さない思想だが、既存 layout-watch は manual layout を即 `state.json` に保存する。 | user operation は敵ではない。ただし即 DesiredWorld 永続化しない。manual-layout candidate として提示/accept されて初めて accepted semantic layout にする。 |
| verifier classes | important | 現設計の class 名はあるが境界が粗い。境界が曖昧だと replan loop か早すぎる unsupported になる。 | 初期実装前に最低定義だけ固定する。`replan` は controller action で収束可能、`dirty` は再観測必要、`unsupported` は安全に mutation 不可、`report-only` は Desired に影響しない。 |
| operation wrappers | blocking before real backend | 既存 `MoveWindowToWorkspaceByName` は focus/move を mutex で包む。これは wrapper の必要性を示す直接根拠。 | `FocusWindow`、move、summon、stack/tab 操作は raw primitive として Planner から出さない。precondition/execute/settle/verify を持つ operation wrapper にする。 |
| package boundary | watch | 最初から package を固定すると interface と DTO が増え、責務より形が先行する。 | package は実装初期で固めすぎない。先に authority boundary と data ownership を固定し、package はそれに従って後から切る。 |
| docs persistence | process blocking | `queue/` は `.gitignore` により追跡されない。合意文書が commit/PR に残らない。 | 実装に入る前に tracked docs へ移すか、少なくとも重要決定を tracked file / issue / PR body に保存する。 |

### 12.1 僕の総合判断

この疑問リストは妥当だった。
ただし、全部を同列に扱うべきではない。

実装前に固定すべきものは、機能詳細ではなく **truth と mutation の憲法**である。

1. manifest は environment contract であり、DesiredWorld ではない。
2. DesiredWorld の永続 truth は PersistentStore だけが持つ。
3. ObservedWorld、LiveWindowID、title、frame、browser runtime ID は durable identity ではない。
4. GUI/window/session/browser mutation は通常 `projwmd` だけが行う。
5. `projwmctl`、fixture、migration、test は daemon/store authority を迂回しない。
6. ambiguous resolver match は mutation しない。
7. production store と test store は機械的に分離する。
8. browser snapshot persistence には privacy/redaction policy が必要。
9. legacy migration は quarantine であり、信頼ではない。
10. OmniWM deploy と `projwmd` lifecycle の責務を重ねない。

### 12.2 実装前 blocking と early implementation の分離

実装前 blocking:

1. manifest scope / validation / existing config の authority 整理。
2. OmniWM deploy と `projwmd` lifecycle の境界。
3. PersistentStore transaction protocol / path separation / migration whitelist。
4. IPC と single writer 境界。
5. admin fixture の production 隔離。
6. resolver ambiguity / operation wrappers / minimum app contracts。
7. browser snapshot privacy boundary。
8. docs persistence。

初期実装で refine してよいもの:

1. simulator L1 の細かい範囲。
2. planner candidate の粒度。
3. event queue の具体的 capacity / backpressure 数値。
4. lifecycle settle の具体的閾値。
5. verifier class の詳細分類。
6. package boundary。

### 12.3 優先順位

次に潰す順番はこれがよい。

1. Authority map: manifest / config / DesiredWorld / store / OmniWM deploy / `projwmd`。
2. Single writer boundary: IPC / daemon / test / migration / admin fixture。
3. PersistentStore safety: generation transaction / test path / migration whitelist。
4. Real mutation safety: resolver ambiguity / operation wrappers / app contracts。
5. Privacy and observability: browser snapshot / trace redaction / verifier/event queue。
