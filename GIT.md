# Git 設定ガイド

Nix（Home Manager）で管理するグローバル設定と、マシンごとにローカルで管理する機密設定を分離するアーキテクチャです。

---

## アーキテクチャ概要

```
Nix 管理（dotfiles に含まれる）
  └── modules/common/git.nix
        ├── グローバル user.name / user.email（個人アカウント）
        ├── credential helper（gh auth git-credential）
        └── include.path = "~/.config/git/local"  ← ローカルファイルを読み込む

ローカル管理（dotfiles に含まれない・新規 Mac で手動作成）
  └── ~/.config/git/local          ← includeIf でワーク設定を条件適用
        └── ~/.config/git/work     ← 業務アカウントの user.name / user.email
```

**分離の理由:**

| 情報 | 管理場所 | 理由 |
|------|----------|------|
| 個人アカウントの name / email | Nix (dotfiles) | 公開リポジトリに書いて問題ない情報 |
| credential helper の宣言 | Nix (dotfiles) | 実トークンは Keychain にあり、宣言だけは安全 |
| 業務アカウントの name / email | ローカル (`~/.config/git/work`) | 会社名・パスが推測されるため非公開 |
| 実トークン | macOS Keychain | 絶対に dotfiles に含めない |

---

## ファイルテンプレート

### `~/.config/git/local`

業務ディレクトリ配下でのみ `work` 設定を適用する。  
**パスは各自の環境に合わせる（dotfiles には書かない）。**

```ini
[includeIf "gitdir:~/path/to/work-directory/"]
    path = ~/.config/git/work
```

### `~/.config/git/work`

業務リポジトリ内で上書きされる user 情報。  
**会社アカウント名・メールを記載（dotfiles には書かない）。**

```ini
[user]
    name  = your-work-github-username
    email = GITHUB_USER_ID+your-work-github-username@users.noreply.github.com
```

> **GitHub no-reply メールの確認方法:** GitHub Settings → Emails → "Keep my email addresses private" を有効にすると表示されるアドレス（形式: `{ID}+{username}@users.noreply.github.com`）。

---

## `programs.git.includes` の仕組みと順序

### なぜ `includes` を使うか

Home Manager の `programs.git.settings` はキーをアルファベット順に出力します。  
`include` は `i`、`user` は `u` なので、`[include]` が先、`[user]` が後になります。

```ini
# settings で書いた場合（アルファベット順 → 順序が逆転）
[credential "https://github.com"]
    helper = ...
[include]               ← i: ここで ~/local を読む
    path = ~/.config/git/local
[user]                  ← u: グローバル user がここで上書きしてしまう ❌
    name = personal
    email = personal@...
```

`programs.git.includes` を使うと HM がファイル末尾に `[include]` を追記するため、  
includeIf の業務設定がグローバル user 設定を正しく上書きできます。

```ini
# includes を使った場合（末尾保証 → 正しい順序）
[user]
    name = personal
    email = personal@...
[credential "https://github.com"]
    helper = ...
[include]               ← 末尾: ~/local を読み、その中の includeIf が適用される ✅
    path = ~/.config/git/local
```

---

## credential helper の仕組み

```
git push
  ↓
[credential] helper が呼ばれる
  ↓
gh auth git-credential（gh CLI が macOS Keychain からトークンを取得）
  ↓
GitHub API 認証
```

**`git.nix` の credential 設定は「どのプログラムを呼ぶか」の宣言のみ。**  
実トークンは `gh auth login` 時に macOS Keychain に保存される。dotfiles に秘密情報は含まれない。

複数アカウントがある場合は `gh auth switch` でアクティブアカウントを切り替える。  
`gh auth status` で現在のアクティブアカウントを確認できる。

---

## 新規 Mac セットアップ手順

1. **Nix / darwin-rebuild でシステムを構成**  
   `git.nix` の内容が `~/.config/git/config` に反映される。

2. **`gh auth login` を実行**  
   ```bash
   gh auth login
   # GitHub.com → HTTPS → Browser でログイン
   ```
   トークンが Keychain に保存され、credential helper が機能するようになる。

3. **ローカルファイルを作成**  
   テンプレートを参考に `~/.config/git/local` と `~/.config/git/work` を作成する。  
   `~/.config/git/local` が存在しない場合、git はサイレントに無視するためエラーにはならない。

4. **動作確認**  
   ```bash
   # 個人リポジトリで確認
   cd ~/path/to/personal-repo
   git config user.email   # 個人 no-reply アドレスが表示されること

   # 業務リポジトリで確認
   cd ~/path/to/work-repo
   git config user.email   # 業務 no-reply アドレスが表示されること
   ```
