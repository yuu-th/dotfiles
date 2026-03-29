# Pake Webapps

Pake CLI を使用して、Web サイトを macOS ネイティブアプリ（.app）にビルドし、宣言的に管理します。

## 仕組み

1. `darwin-rebuild switch` 実行時に、Homebrew で `pake` がインストールされます。
2. `activationScripts` が走り、`hosts/darwin/default.nix` で定義した webapps をチェックします。
3. `~/.local/state/pake-webapps/` 内のバージョン情報と比較し、差分があれば `pake build` を実行して `/Applications/` に配置します。
4. 宣言から削除されたアプリは、自動的に `/Applications/` から削除（GC）されます。

## 使い方

`webapps` アトリビュートにアプリを追加します。

```nix
pake.webapps.my-app = {
  url     = "https://example.com";
  name    = "MyApp";
  version = "1.0.0"; # 設定を変更した場合は、ここを上げるとリビルドされます
};
```

## 注意点

- **ビルド時間**: 初回は Rust のコンパイルが走るため、数分かかります。
- **Spotlight 連携**: ビルドされたアプリは `/Applications/` に直接置かれるため、標準の Spotlight ですぐに検索可能です。
- **GUI の制限**: アプリ内の設定は保持されますが、ウィンドウサイズやタイトルバーの有無などの「ビルド時設定」を変更するには、`version` を更新して再ビルドが必要です。
