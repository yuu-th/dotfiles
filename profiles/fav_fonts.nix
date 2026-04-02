# profiles/fav_fonts.nix
#
# フォント管理の単一ソース。
# フォントを追加・変更する際はここだけ編集する。
# fontFamily を参照するモジュール（ghostty 等）はここで上書きする。
{ pkgs, ... }: {
  fonts.packages = with pkgs; [
    udev-gothic-nf
  ];

  # Ghostty が参照するプライマリフォント（fonts.packages と同期を保つ）
  myConfig.darwin.ghostty.fontFamily = "UDEV Gothic NF";
}
