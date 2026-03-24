{ ... }: {
  # ── macOS システムデフォルト (AeroSpace 最適化) ──────────────────────────────
  # これらの設定は darwin-rebuild switch で冪等に適用される

  system.defaults = {
    # ── Dock 設定 ──────────────────────────────────────────────────────────────
    # AeroSpace がウィンドウを右下隅に隠すため、Dock は bottom + autohide が推奨
    dock = {
      autohide = true;
      autohide-delay = 0.0;          # Dock 表示ディレイをゼロに
      autohide-time-modifier = 0.15; # アニメーション時間を最小に
      mru-spaces = false;            # Spaces の「最近の使用順」自動並べ替えを無効
      show-recents = false;          # 最近使ったアプリを Dock に表示しない
      orientation = "bottom";
      tilesize = 48;
    };

    # ── グローバル設定 ────────────────────────────────────────────────────────
    NSGlobalDomain = {
      # 解決策1: 別モニタの同一アプリへの余計なフォーカス移動を無効化
      AppleSpacesSwitchOnActivate = false;
    };
  };

  # ── nix-darwin のオプションに存在しない設定は activationScripts で適用 ────────
  # defaults write で直接 macOS の隠し設定を変更する
  system.activationScripts.aerospaceSystemTweaks = {
    text = ''
      echo "AeroSpace: applying system tweaks..."

      # ── Goodies #1: どこでもウィンドウをドラッグ (ctrl+cmd+drag) ──────────
      # タイトルバー以外の部分でもドラッグしてウィンドウ移動できるようになる
      /usr/bin/defaults write -g NSWindowShouldDragOnGesture -bool true

      # ── Goodies #5: ウィンドウアニメーションを無効化 ──────────────────────
      # ウィンドウの開閉・タイリング操作がきびきびになる
      /usr/bin/defaults write -g NSAutomaticWindowAnimationsEnabled -bool false

      # ── Displays have separate Spaces を OFF ──────────────────────────────
      # AeroSpace 公式ガイドの推奨設定
      # - マルチモニタ時のフォーカス暴走やパフォーマンス問題が軽減
      # - ウィンドウが Space 間を移動する非公開 API の問題を回避
      # ⚠️ 初回適用後はログアウト → ログインが必要
      # ⚠️ macOS ネイティブフルスクリーンが他モニタを黒画面にする
      #    → AeroSpace の fullscreen コマンド (alt-enter) で代替
      /usr/bin/defaults write com.apple.spaces spans-displays -bool true

      # ── Mission Control: Group windows by application ────────────────────
      # AeroSpace がウィンドウを右下に隠す影響で
      # Mission Control のプレビューが見づらくなるのを軽減する
      /usr/bin/defaults write com.apple.dock expose-group-apps -bool true

      # ── ウィンドウリサイズ速度を最大に ────────────────────────────────────
      /usr/bin/defaults write -g NSWindowResizeTime -float 0.001

      /usr/bin/killall Dock 2>/dev/null || true
    '';
  };
}
