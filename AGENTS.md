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
    home-manager.users.${config.system.primaryUser} = {
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
- `home-manager.users.${config.system.primaryUser}` を介して HM 設定を書く
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
  home-manager.users.${config.system.primaryUser} = {
    home.username      = config.system.primaryUser;
    home.homeDirectory = lib.mkForce "/Users/${config.system.primaryUser}";
    home.stateVersion  = "24.11";
  };
}
```

---

## 新しいモジュールを追加する手順

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
home.homeDirectory = "/Users/${config.system.primaryUser}";

# OK
home.homeDirectory = lib.mkForce "/Users/${config.system.primaryUser}";
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
