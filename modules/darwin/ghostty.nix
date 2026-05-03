# modules/darwin/ghostty.nix
#
# ╔══════════════════════════════════════════════════════════════════╗
# ║  カスタマイズガイド                                              ║
# ║                                                                  ║
# ║  フォント    → profiles/fav_fonts.nix を編集                    ║
# ║  シェル      → pkgs.fish を変更（fish.nix と合わせて変更）       ║
# ║  テーマ      → theme = <name> を変更（候補は下記コメント参照）   ║
# ║  キーバインド → keybind = <combo>=<action> を追加/変更           ║
# ╚══════════════════════════════════════════════════════════════════╝
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.darwin.ghostty; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.ghostty = {
    enable = lib.mkEnableOption "Ghostty terminal emulator";
    fontFamily = lib.mkOption {
      type        = lib.types.str;
      default     = "UDEV Gothic NF";
      # フォントを変更する場合は profiles/fav_fonts.nix を編集すること
      description = "Font family used in Ghostty. Managed via profiles/fav_fonts.nix.";
    };
  };

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "ghostty" ];

    home-manager.users.${config.myConfig.primaryUser} = {
      home.file.".config/ghostty/config".text = ''
        # ── フォント ─────────────────────────────────────────────────────
        # フォントの変更は profiles/fav_fonts.nix を編集（ここは自動反映）
        font-family = ${cfg.fontFamily}
        font-size = 13
        # font-thicken = true  # Retina でフォントを少し太くしたい場合

        # ── テーマ ───────────────────────────────────────────────────────
        # 利用可能なテーマ一覧: ghostty +list-themes
        # 人気: tokyonight / catppuccin-mocha / nord / gruvbox-dark / dracula
        theme = tokyonight

        # ── 背景 ─────────────────────────────────────────────────────────
        background-opacity = 0.92
        background-blur-radius = 20
        # background-opacity = 1.0  # 完全不透明にしたい場合

        # ── カーソル ─────────────────────────────────────────────────────
        cursor-style = block
        cursor-style-blink = false
        # cursor-style = bar  # | 型カーソルの場合

        # ── ウィンドウ ───────────────────────────────────────────────────
        window-padding-x = 8
        window-padding-y = 8
        window-decoration = false
        macos-titlebar-style = hidden

        # ── シェル起動 ───────────────────────────────────────────────────
        # ログインシェルは zsh のまま。Ghostty が fish を直接起動する。
        # pkgs.fish を参照することで fish パッケージへの明示的な依存を保証。
        # nix-darwin の programs.fish.enable が /etc/fish/ に Nix PATH を生成済み。
        command = ${pkgs.fish}/bin/fish -l

        # ── シェル統合 ───────────────────────────────────────────────────
        shell-integration = detect
        copy-on-select = clipboard
        confirm-close-surface = false

        # ── キーバインド ─────────────────────────────────────────────────
        # アクション一覧: https://ghostty.org/docs/config/keybind/reference
        # Quake terminal は OmniWM 内蔵に一本化（karabiner-rules.nix 参照）

        # ── Accessibility 権限を要求するための global keybind（projwm v11.5）
        # Ghostty は global keybind が設定されると起動時に macOS Accessibility
        # 権限を要求するダイアログを出す。これにより Ghostty.app が AX 許可
        # リストに追加され、OmniWM の `AXUIElementCopyAttributeValue` 経由の
        # window enumeration から見えるようになる（ghostty-org/ghostty docs:
        # 「On macOS, launching with global keybinds requires accessibility
        # permissions to be granted to Ghostty」）。
        # 使わない組み合わせを割当：global+ctrl+opt+cmd+shift+F19=ignore は
        # 物理的に押されにくく副作用がない。
        keybind = global:ctrl+option+command+shift+f19=ignore
      '';
    };
  };
}
