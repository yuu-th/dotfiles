# AGENTS.md — AI エージェント向けパターンガイド

このプロジェクトは厳密なパターンに従って構造化されています。
モジュールやプロファイルを追加・変更する際は、このドキュメントのパターンを必ず守ってください。

---

## アーキテクチャ: 樹状構造 (Dendritic)

```
hosts/  →  profiles/  →  modules/
```

各レイヤの責務を明確に分離します。

| レイヤ | 責務 | 書いていいもの | 書いてはいけないもの |
|---|---|---|---|
| `hosts/` | マシン固有ファクト | `hostPlatform`, `primaryUser`, `stateVersion`, `tailscale.enable` | アプリ設定、enable フラグ |
| `profiles/` | 環境の全体像 | `myConfig.*.enable` フラグ、常時ON設定（dock, touchId 等）、HM user facts | 個別アプリのロジック |
| `modules/` | 個別機能の実装 | アプリ/ツールの設定ロジック | enable フラグの決定 |

---

## modules/common/ のパターン

OS 非依存ツール（CLI ツール、開発言語、クラウド CLI 等）。

**nix-darwin システムモジュール**として実装し、内部で `home-manager.users.*` を呼び出します。
consumers は Home Manager レイヤを意識しません。

```nix
# modules/common/example.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.example; in {
  options.myConfig.example.enable = lib.mkEnableOption "Example tool";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = with pkgs; [ example-pkg ];
      programs.zsh.shellAliases = {
        ex = "example";
      };
    };
  };
}
```

**必須ルール:**
- `options.myConfig.<name>.enable` を必ず定義する
- `config = lib.mkIf cfg.enable { ... }` でガードする
- `home-manager.users.${config.myConfig.primaryUser}` を介して HM 設定を書く
- `lib.mkForce` が必要な場合（`home.homeDirectory` 等）は忘れずに付ける

---

## modules/darwin/ のパターン

macOS 専用アプリ・ツール。主に Homebrew Cask で管理するものが対象。

### シンプルな Cask アプリ

```nix
# modules/darwin/example-app.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.exampleApp; in {
  imports = [ ./homebrew.nix ];  # Homebrew 基盤を自己宣言

  options.myConfig.darwin.exampleApp.enable = lib.mkEnableOption "Example App";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "example-app" ];
  };
}
```

**必須ルール:**
- Homebrew を使うモジュールは **必ず `imports = [ ./homebrew.nix ]` を自己宣言**する
- cask 名は Homebrew の正式名称を使う（`homebrew search` で確認）
- `options.myConfig.darwin.<name>.enable` の命名規則を守る

### サブディレクトリのあるモジュール（設定ファイルを伴う場合）

```
modules/darwin/my-tool/
├── default.nix   # メインモジュール（上記パターンを適用）
└── config.json   # ツールの設定ファイル
```

---

## modules/darwin/pake-webapps/ のパターン

Web アプリを Pake でネイティブアプリ化する仕組み。

webapp を追加する場合は `google-calendar.nix` のように **専用モジュールを作成**し、
`myConfig.darwin.pake.webapps` に委譲します:

```nix
# modules/darwin/my-webapp.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.myWebapp; in {
  imports = [ ./pake-webapps ];

  options.myConfig.darwin.myWebapp = {
    enable  = lib.mkEnableOption "My Web App as native app";
    version = lib.mkOption { type = lib.types.str; default = "1.0.0"; };
  };

  config = lib.mkIf cfg.enable {
    myConfig.darwin.pake.webapps.my-webapp = {
      url     = "https://example.com";
      name    = "MyWebApp";
      width   = 1440;
      height  = 900;
      version = cfg.version;
    };
  };
}
```

**リビルドのトリガーは `version` フィールドを上げることだけ。**

---

## profiles/ のサブファイルパターン

`profiles/darwin.nix` が肥大化しないよう、**関心ごとに分割したサブファイル**を `profiles/` 以下に置き、`darwin.nix` から import できる。

### いつサブファイルにするか

| ケース | 判断 |
|---|---|
| enable フラグ不要の「常時ON設定」が1つのテーマにまとまる | サブファイルに分割 |
| 複数のモジュールに共通して値を注入する | サブファイルに分割 |
| 数行程度 | `darwin.nix` に直書き |

### フォント管理パターン（`profiles/fav_fonts.nix`）

フォントとそれを参照するモジュールオプションを**一箇所で管理**する規約。

```nix
# profiles/fav_fonts.nix
{ pkgs, ... }: {
  # フォントのインストール（nix-darwin / NixOS 共通オプション）
  fonts.packages = with pkgs; [
    udev-gothic-nf
  ];

  # フォントを参照するモジュールのオプションをここで上書き
  # → フォントを変更するときはここだけ編集すれば良い
  myConfig.darwin.ghostty.fontFamily = "UDEV Gothic NF";
}
```

`darwin.nix` への追加:

```nix
imports = [
  # ...
  ./fav_fonts.nix  # sub-profiles（常時ON設定を関心ごとに分割）
];
```

モジュール側（例: `modules/darwin/ghostty.nix`）は文字列オプションとして受け取る:

```nix
options.myConfig.darwin.ghostty = {
  enable     = lib.mkEnableOption "Ghostty";
  fontFamily = lib.mkOption {
    type    = lib.types.str;
    default = "UDEV Gothic NF";
  };
};
# 設定ファイル内で参照
# font-family = ${cfg.fontFamily}
```

**必須ルール:**
- フォントを追加・変更する際は `profiles/fav_fonts.nix` **だけ**を編集する
- モジュール側でフォント名をハードコードしない

---

## boxes/ のパターン

macOS 上の OrbStack VM として起動する、用途別の**隔離された作業環境**。
`hosts/` が物理機・クラウド VM を指すのに対し、`boxes/` はオンデマンドで起動するサンドボックス VM を指す。

### アーキテクチャ（2 層構造）

```
macOS
  └── OrbStack VM (NixOS)          ← boxes/box-NAME/default.nix
        ├── /data/boxes/NAME/      ← 永続ストレージ（VM 再起動でも消えない）
        └── nixos-container NAME   ← 隔離された作業空間（systemd-nspawn）
              └── /root/           ← /data/boxes/NAME/ をマウント
```

- **VM ホスト層**: 土台。`box-base.nix` で定型化。各 box は固有部分だけ書く
- **コンテナ層**: 実際の作業空間。同じ Linux カーネルを名前空間で隔離。`/root/` 以外は見えない
- **永続ストレージ**: `/data/boxes/NAME/` が `/root/` にマウントされ、コンテナ停止後も残る

### ファイル構造

```
boxes/
  box-NAME/
    default.nix    # 固有部分のみ（VMホスト定型は box-base.nix に委譲）
modules/nixos/
  orbstack-vm.nix  # OrbStack ハードウェア設定（bootloader なし、btrfs）
  box-base.nix     # 全 box 共通の VM ホスト定型
```

`hardware-configuration.nix` は不要。OrbStack VM のハードウェアは全インスタンス共通のため `orbstack-vm.nix` で静的に定義済み。

### box-base.nix が提供するもの

各 box が `box-base.nix` を import することで以下が自動的に適用される:

| 設定 | 内容 |
|---|---|
| `nixpkgs.hostPlatform` | `aarch64-linux` (Apple Silicon) |
| `orbstack-vm.nix` | ブートローダなし、btrfs filesystem 宣言 |
| `users.users.${myConfig.primaryUser}` | OrbStack ログイン用ホストユーザー |
| `security.sudo.wheelNeedsPassword` | `false`（開発用 VM）|
| `nix.settings` | flakes 有効、trusted-users |
| `system.stateVersion` | `"24.11"` |

### default.nix のパターン

各 box が書くのは**固有部分のみ**:

```nix
# boxes/box-NAME/default.nix
{ inputs, ... }: {
  imports = [ ../../modules/nixos/box-base.nix ];

  myConfig.primaryUser = "yuta";

  # この box が所有する永続ディレクトリ
  systemd.tmpfiles.rules = [ "d /data/boxes/NAME 0755 root root -" ];

  containers."NAME" = {
    autoStart      = true;   # VM 起動時に自動起動
    privateNetwork = false;  # ネット接続が必要な場合。隔離したい場合は true

    # /root/ にマウント → root-login で着地した瞬間に永続領域にいる
    bindMounts."/root" = {
      hostPath   = "/data/boxes/NAME";
      isReadOnly = false;
    };

    config = { pkgs, ... }: {
      system.stateVersion        = "24.11";
      nixpkgs.config.allowUnfree = true;

      # root から使えるようにシステムレベルで定義
      environment.systemPackages = with pkgs; [
        # ツールをここに列挙
      ];

      programs.zsh = {
        enable = true;
        interactiveShellInit = ''
          PROMPT='%F{COLOR}[box-NAME]%f %~ %# '
        '';
      };

      programs.git = {
        enable = true;
        config = { user.name = "NAME"; user.email = "EMAIL"; };
      };

      users.users.root.shell = pkgs.zsh;
    };
  };
}
```

**必須ルール:**
- `box-base.nix` を必ず import する
- `myConfig.primaryUser` を必ず設定する（OrbStack のホストログインユーザー）
- ツールはコンテナ内で `environment.systemPackages` に定義する（root から使えるため）
- home-manager はコンテナ内では使わない（root 運用のため不要）
- `bindMounts."/root"` に永続ストレージをマウントする（root-login で着地する場所）
- プロンプトに `[box-NAME]` と色を入れ、どの box にいるか一目でわかるようにする

### flake.nix への追加

```nix
nixosConfigurations.box-NAME = nixpkgs.lib.nixosSystem {
  specialArgs = { inherit inputs; };
  modules = [ ./boxes/box-NAME/default.nix ];
};
```

### 新しい box を追加する手順

1. `boxes/box-NAME/default.nix` を作成（上記パターン適用）
2. `flake.nix` に `nixosConfigurations.box-NAME` を追加
3. git add してから VM を作成・適用:

```bash
git add boxes/box-NAME/
# 初回 bootstrap（ssh root でも可）
orb create nixos:24.11 box-NAME
ssh root@box-NAME.orb.local "nixos-rebuild switch --flake /Users/yuta/dev/dotfiles#box-NAME"
```

4. 以降は macOS から `box` コマンドで操作:

```bash
box NAME          # コンテナのシェルに入る → [box-NAME] ~ #
box switch NAME   # 環境を再定義して適用
box stop NAME     # 停止（/data/boxes/NAME/ は保持）
```

### ファイル空間の対応

| 視点 | パス |
|---|---|
| macOS | `/Users/yuta/OrbStack/box-NAME/data/boxes/NAME/` |
| OrbStack VM | `/data/boxes/NAME/` |
| コンテナ内 | `/root/` |

コンテナ内からは macOS のファイルは見えない（隔離されている）。

### `privateNetwork` の使い分け

| 値 | 用途 |
|---|---|
| `false` | API 通信が必要な場合（AI agent CLI 等） |
| `true` + `localAddress` | 外部通信を遮断したい場合（セキュリティ重視） |

---

## modules/nixos/ のパターン

NixOS サーバ専用サービス。`myModules.nixos.*` 名前空間を使います。

```nix
# modules/nixos/example-service.nix
{ config, lib, pkgs, ... }:
let cfg = config.myModules.nixos.exampleService; in {
  options.myModules.nixos.exampleService.enable = lib.mkEnableOption "Example systemd service";

  config = lib.mkIf cfg.enable {
    systemd.services."example" = { ... };
  };
}
```

---

## profiles/darwin.nix のパターン

darwin 環境の全設定を集約する単一ファイル。ここで行うこと:

1. 全モジュールの `imports`
2. **常時ON設定の直書き**（`allowUnfree`, `touchIdAuth`, dock 設定, activation scripts 等）
3. **全 `myConfig.*.enable` フラグの設定**
4. `home-manager.users.*` の基本ファクト（username, homeDirectory, stateVersion）

```nix
# profiles/darwin.nix の構造
{ config, lib, pkgs, ... }: {
  imports = [ /* 全モジュール */ ];

  # 常時ON設定（enable フラグ不要なもの）
  nixpkgs.config.allowUnfree = true;
  security.pam.services.sudo_local.touchIdAuth = true;
  system.defaults.dock = { autohide = true; ... };

  # 全モジュールの有効化
  myConfig.darwin.homebrew.enable = true;
  myConfig.zsh.enable = true;
  # ...

  # HM user facts
  home-manager.users.${config.myConfig.primaryUser} = {
    home.username      = config.myConfig.primaryUser;
    home.homeDirectory = lib.mkForce "/Users/${config.myConfig.primaryUser}";
    home.stateVersion  = "24.11";
  };
}
```

---

## modules/common/agent-skills/ のパターン

AI エージェント向けスキルを Nix で宣言管理する仕組み。
外部スキル（公式・サードパーティ）と個人スキルを**別々のメカニズムで共存**させる点が特徴。

### 2 種類のスキルとその扱い

| 種類 | 場所 | 更新方法 | 反映タイミング |
|---|---|---|---|
| 外部スキル | Nix store（読み取り専用） | `flake.lock` 更新 → HM switch | ビルド後 |
| 個人スキル | `~/dev/my-skills/`（mutable） | ファイルを保存するだけ | 即時（シンボリックリンク） |

### ファイル構造

```
modules/common/agent-skills/
├── flake.nix      # ① 外部スキルの宣言（どのリポジトリのどのスキルを使うか）
├── flake.lock     # ② 外部スキルのコミットをピン留め（このディレクトリ専用の lock）
└── default.nix    # ③ nix-darwin ラッパー（gitwatch + 個人スキルシンボリックリンク）
```

**`flake.nix`（外部スキル管理）の責務:**
- 外部リポジトリを `inputs` として取り込む（`flake = false` でピン留め）
- `skills.enable = [ "skill-name" ]` で有効化するスキルを選択
- `targets.*.enable = true` で配置先ディレクトリを指定

**`default.nix`（nix-darwin ラッパー）の責務:**
- `myConfig.agentSkills.enable` option を提供
- `~/dev/my-skills/` の各スキルを全ターゲットに `mkOutOfStoreSymlink` で直接リンク
- `gitwatch` launchd agent を登録（`~/dev/my-skills/` の変更を自動 commit & push）

### 外部スキルを追加・変更する手順

編集対象は **`flake.nix` のみ**。

```nix
# ① 新しいスキルソースを inputs に追加
my-new-source = {
  url = "github:owner/repo";
  flake = false;
};

# ② sources に登録
sources.my-new-source = {
  path   = my-new-source;
  subdir = "skills";  # スキルファイルが置かれているサブディレクトリ
};

# ③ 有効化
skills.enable = [ "existing-skill" "new-skill-id" ];
```

```bash
git add modules/common/agent-skills/
# チャイルドフレークの lock を更新してからビルド
nix flake update --update-input skills-catalog --flake .
sudo darwin-rebuild switch --flake .#yuta
```

> **注意:** メインの `flake.lock` ではなく `modules/common/agent-skills/flake.lock` が更新される。
> `skills-catalog` input がチャイルドフレーク全体をラップしているため。

### 個人スキルを追加する手順

`~/dev/my-skills/` に新しいスキルディレクトリを作り、`default.nix` の `personalSkills` リストに追加する。

**Step 1:** `~/dev/my-skills/<skill-name>/SKILL.md` を作成（gitwatch が自動 push）

**Step 2:** `default.nix` の `personalSkills` に追加してビルド:

```nix
personalSkills = [
  "browser-use"
  "my_gemini_cli"
  "new-skill"   # ← 追加
];
```

```bash
sudo darwin-rebuild switch --flake .#yuta
# → 全ターゲット（.claude/skills/, .copilot/skills/ 等）に即時シンボリックリンクが張られる
```

### gitwatch の動作

`launchd.user.agents.gitwatch-my-skills` として常駐。

- `~/dev/my-skills/` 以下のファイル変更を検知
- 変更後 10 秒待機してから `git commit` + `git push origin main`
- ログ: `~/.local/log/gitwatch-my-skills.log`
- Keychain 経由で `gh auth git-credential` を使うため `gh auth login` が事前に必要

```bash
# gitwatch が動いているか確認
launchctl list | grep gitwatch

# ログ確認
tail -f ~/.local/log/gitwatch-my-skills.log
```

### チャイルドフレークについて

`modules/common/agent-skills/` は**独立したフレーク**。メインの `flake.nix` からは `path:` で参照される。

```nix
# メイン flake.nix
skills-catalog.url = "path:./modules/common/agent-skills";
```

外部スキルのピン留めは `modules/common/agent-skills/flake.lock` で管理される（メインの `flake.lock` とは別）。
特定のスキルソースだけ更新したい場合:

```bash
# チャイルドフレーク内の特定 input を更新
cd modules/common/agent-skills
nix flake update anthropic-skills
# → modules/common/agent-skills/flake.lock が更新される
```

---



### macOS 向け Cask アプリを追加する場合

1. `modules/darwin/<app-name>.nix` を作成（上記パターン適用）
2. `profiles/darwin.nix` の `imports` に追加
3. `profiles/darwin.nix` に `myConfig.darwin.<appName>.enable = true;` を追加
4. **`git add` してからビルド**（新規ファイルは git staging 必須）

```bash
git add modules/darwin/<app-name>.nix
sudo darwin-rebuild build --flake .#yuta  # まず build で確認
sudo darwin-rebuild switch --flake .#yuta
```

### OS 非依存ツールを追加する場合

1. `modules/common/<tool>.nix` を作成（上記パターン適用）
2. `profiles/darwin.nix` の `imports` に追加
3. `profiles/darwin.nix` に `myConfig.<tool>.enable = true;` を追加
4. `git add` してからビルド

---

## よくあるミス（やってはいけないこと）

### カテゴリ名のモジュールを作らない

```
# NG: カテゴリ名でまとめたファイル
modules/darwin/macos-tweaks.nix   # ← 削除済み。このパターンは使わない
modules/common/dev-tools.nix      # ← NG
modules/common/core.nix           # ← NG

# OK: 機能・アプリ単位
modules/darwin/borders.nix
modules/common/go.nix
```

常時ONの設定（dock, window animations 等）はモジュールにせず `profiles/darwin.nix` に直書きする。

### Homebrew 依存の宣言漏れ

```nix
# NG: homebrew.nix を import していない
{ config, lib, ... }: {
  options.myConfig.darwin.myApp.enable = lib.mkEnableOption "...";
  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "my-app" ];  # homebrew.nix を import しないと動かない
  };
}

# OK
{ config, lib, ... }: {
  imports = [ ./homebrew.nix ];  # ← 必須
  ...
}
```

### home.homeDirectory に lib.mkForce を忘れる

```nix
# NG: home-manager のデフォルト null と競合してエラーになる
home.homeDirectory = "/Users/${config.myConfig.primaryUser}";

# OK
home.homeDirectory = lib.mkForce "/Users/${config.myConfig.primaryUser}";
```

### 新規ファイルを git add せずにビルドする

Nix flake は git の追跡対象外ファイルを無視します。

```bash
# 新規ファイル作成後は必ず git add してからビルド
git add modules/darwin/new-app.nix
sudo darwin-rebuild build --flake .#yuta
```

### pake webapp を pake-webapps/default.nix に直接書く

webapp の追加は専用モジュール（`google-calendar.nix` 参照）を作り、
`myConfig.darwin.pake.webapps.*` に委譲します。
`pake-webapps/default.nix` 自体は変更しません。

---

## 名前空間の整理

| 対象 | 名前空間 |
|---|---|
| OS 非依存ツール | `myConfig.<name>.enable` |
| macOS 専用アプリ | `myConfig.darwin.<name>.enable` |
| NixOS サービス | `myModules.nixos.<name>.enable` |

---

## ビルド・確認コマンド

```bash
# ドライランで問題確認（switch より安全）
sudo darwin-rebuild build --flake .#yuta

# 適用
sudo darwin-rebuild switch --flake .#yuta

# NixOS サーバへのデプロイ
nixos-rebuild switch --flake .#server --target-host root@dotfiles-bot

# flake.lock の特定 input のみ更新
nix flake update --update-input <input-name>
```
