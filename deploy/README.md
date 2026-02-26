# Deploying to GCP

このプロジェクトでは、Terraform を使用せず、`nixos-anywhere` を使ったシンプルなデプロイを採用しています。

## 初回デプロイ（VM作成からNixOS化まで）

1. **VM の作成**
   GCP コンソールまたは `gcloud` CLI で VM を作成します。OS は何でも構いません（Debian 12 等を推奨）。
   
1.  **VM の作成**
    GCP コンソールまたは `gcloud` CLI で VM を作成します。OS は何でも構いません（Debian 12 等を推奨）。
    
    > [!IMPORTANT]
    > **e2-micro の注意点**: 1GB RAM では `nixos-anywhere` の実行に失敗することがあります。初回デプロイ時のみ一時的に `e2-small` (2GB RAM) 以上に設定し、デプロイ完了後に `e2-micro` に戻すことを推奨します。

    ```bash
    # あなたの公開鍵（例: ~/.ssh/id_ed25519.pub）を root ユーザとして登録し、
    # かつ root ログインを許可するスタートアップスクリプトを走らせます
    gcloud compute instances create dotfiles-bot \
      --zone=us-central1-a \
      --machine-type=e2-small \
      --image-family=debian-12 \
      --image-project=debian-cloud \
      --tags=http-server,https-server \
      --scopes=cloud-platform \
      --metadata="ssh-keys=root:$(cat ~/.ssh/id_ed25519.pub),startup-script=sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin yes/g' /etc/ssh/sshd_config; systemctl reload ssh"
    ```

2.  **SSH 接続の確認**
    VM が起動し、スクリプトが完了するまで **1分ほど待ってから** 確認します。
    ```bash
    ssh root@<SERVER_IP>
    ```

3.  **NixOS のインストール**
    インストール時に、手元の SSH 鍵をサーバの `/var/lib/authorized_keys` に安全に流し込むための準備をしてから実行します。

    ```bash
    # 1. 流し込み用のディレクトリ構造を作成（一時的）
    mkdir -p deploy/temp/var/lib
    cp ~/.ssh/id_ed25519.pub deploy/temp/var/lib/authorized_keys

    # 2. --extra-files と --build-on-remote オプションを付けて実行
    # (Mac から Linux へデプロイする場合、--build-on-remote が必須です)
    nix run github:nix-community/nixos-anywhere -- \
      --flake .#server \
      --extra-files deploy/temp \
      --build-on-remote \
      root@<SERVER_IP>

    # 3. インストール完了後、一時ファイルを削除
    rm -rf deploy/temp
    ```

## トラブルシューティング

### "REMOTE HOST IDENTIFICATION HAS CHANGED!" と出る
VM を作り直すと、同じ IP アドレスでもサーバの「鍵（ホストキー）」が変わるため、SSH が警告を出します。以下のコマンドで古い鍵の記録を消去してください。

```bash
ssh-keygen -R <SERVER_IP>
```

4.  **後片付け**
    インストールが成功したら、VM のマシンタイプを `e2-micro` に戻しても構いません。

## 日常の更新

Tailscale 導入後は、IP アドレスを意識する必要はありません。MagicDNS 名前解決により、以下のコマンドでどこからでも（VPN 接続中なら）セキュアに更新できます。

```bash
nixos-rebuild switch --flake .#server --target-host root@dotfiles-bot
```
## 自動化のためのシークレット設定 (GCP Secret Manager)

サーバが GitHub に `flake.lock` の更新を push するためには、GitHub トークンを GCP に預ける必要があります。

これで、毎日 12:00 (JST) にサーバが自律的に GitHub を更新するようになります。
