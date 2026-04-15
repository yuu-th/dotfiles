# cmux ワークフロー

> cmux・zellij・git worktree・AI CLI は、それぞれが独立したツールではなく一つの開発体験として設計されています。cmux がワークスペースの器を管理し、zellij がセッションを永続化し、AI CLI が長時間タスクを担い、worktree が並走ブランチを分離します。「タブなし・ペイン分割なし」という選択も、役割ごとにレイヤーを分けてシンプルさを保つための設計判断です。

---

## 不変条件

この環境の前提として、次の条件が常に成り立ちます。

### 固定スロット

- slot `0`: `shell`
- slot `1`: `ai-viewer`
- slot `2+`: project workspace

`cmux-init` 後はこの並びが正です。

### 1 project = 1 workspace / 2 zellij sessions / 2 panes

各 project workspace は、

- 1 つの作業ディレクトリまたは git worktree に対応
- AI session と tools session の 2 つの zellij session を持つ
- 左 pane（AI）と右 pane（tools / nvim / browser）の 2 pane 構成

workspace 名は `proj [copilot]` / `proj [claude]` の形で見えます。

### 半壊は直さない・消えたものだけ復元する

半壊状態（workspace は存在するが中身が壊れている）は自動修復しません。  
壊れたら閉じて作り直します。復元対象は「消えた workspace」だけです。

---

## コマンド早見表

| コマンド | 役割 |
|---|---|
| `aidev` | 現在のディレクトリに対応する project workspace を開く |
| `aidev --ai copilot` | AI を Copilot に明示して開く |
| `aidev --ai claude` | AI を Claude に明示して開く |
| `aidev-stop` | project workspace と関連 session を止める |
| `worktree-new <branch> <base>` | sibling worktree を作成し、aidev まで進める |
| `cmux-init` | 固定スロットを整え、消えた workspace を復元する |
| `ai-viewer` | 全 AI session を一覧表示する |
| `ai-viewer-refresh` | ai-viewer を壊して正しい session で作り直し slot `1` に置く |

---

## 作業を始める

### 単一プロジェクトで始める

project ディレクトリに移動して `aidev` を実行します。

```bash
cd /path/to/project
aidev --ai copilot    # または --ai claude
```

左 pane に AI、右 pane に tools / nvim / browser が並んだ workspace が作られます。

### 並走ブランチを作る

別ブランチで並行作業したいときは `worktree-new` を使います。

```bash
worktree-new <branch> <base>
```

例：

```bash
worktree-new feature/login origin/main
```

- 親ディレクトリ直下に `dotfiles-feature-login` を sibling として作成
- branch 名（`feature/login`）はそのまま Git に渡し、directory 名だけ sanitize する
- 作成後そのまま `cd` して `aidev` まで進む

base は必ず明示してください。どの地点から切ったかを明確にし、現在 checkout 中のブランチへの暗黙依存を避けるためです。

### 既存 worktree に戻る

既に worktree directory がある場合は、手動で移動して `aidev` を実行します。

```bash
cd ../dotfiles-feature-login
aidev --ai copilot
```

---

## 複数 AI を俯瞰する

`ai-viewer` を使うと、存在中の AI session をまとめて確認できます。

- `cmux-init` 実行後は `ai-viewer` が slot `1` に配置されます
- `ai-viewer` だけを単独で作り直したい場合は `ai-viewer-refresh` を使います

---

## 止める・片付ける

### workspace を止める

作業を一時中断するときは `aidev-stop` を使います。

```bash
aidev-stop
```

workspace と関連 session を閉じますが、worktree directory は残ります。  
停止と削除を分けることで、中断再開が安全になります。

### worktree を削除する

worktree が不要になったら、次の順で行います。

```bash
# 1. 未コミット変更を確認
cd ../dotfiles-feature-login
git status --short

# 2. workspace / session を止める
aidev-stop

# 3. worktree directory を外す
cd ../dotfiles
git worktree remove ../dotfiles-feature-login

# 4. (任意) branch も不要なら削除
git branch -d feature/login
```

この順番にしておくと、まだ見たい作業を誤って消しにくく、AI workspace の取り残しも減らせます。

### `git branch -d` / `-D` / remote delete の違い

| コマンド | 役割 |
|---|---|
| `git branch -d <branch>` | local branch の安全削除（未 merge で危ない場合は Git が拒否） |
| `git branch -D <branch>` | local branch の強制削除（意図的に捨てるときだけ使う）、これは厳格に禁止である。 |
| `git push origin --delete <branch>` | remote branch の削除（local や worktree directory は消えない） |

---

## 状態を復元する

### `cmux-init` がやること

1. `shell` を slot `0` に置く
2. 既存 session から `ai-viewer` を早めに出す
3. 関係ない workspace を片付ける
4. 既知の project workspace を復元する
5. zellij session はあるが workspace がない project を復元する

復元対象は「存在しない workspace」です。半壊だが残っている workspace は自動では直しません。

---

## 壊れたときの判断

### 3 状態と対応手順

| 状態 | 見た目 | 対応 |
|---|---|---|
| **A. 正常** | 左 AI・右 tools/nvim/browser がある | そのまま使う |
| **B. 消失** | zellij session は残っているが workspace がない | `cmux-init` で復元 |
| **C. 半壊** | workspace は存在するが pane / surface が崩れている | `⌘⇧W` で閉じて `aidev` で再作成 |

半壊状態は、利用者の目には明らかでも機械的には曖昧です。自動判定を増やすより「壊れたら閉じる」の方が全体として堅くなります。

### 迷ったときのルール

1. **この workspace は正常か？** → 正常ならそのまま使う
2. **workspace が消えているだけか？** → `cmux-init` で戻せる
3. **半壊か？** → `⌘⇧W` で閉じて作り直す
