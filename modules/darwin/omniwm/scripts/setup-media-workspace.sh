# メディアワークスペース自動構築（niri 流の簡略版）
# WS M に行って必要なアプリを開くだけ。あとは appRules が振り分け、
# niri が column としてスクロール可能に並べる。
#
# 元の AeroSpace 版は「join-with でアコーディオン化 + Calendar 縮小」までを
# スクリプトで強制していたが、niri は column 単位のスクロールが標準なので
# その後の整形は手動 (Option+, / Option+. / Option+T) で十分。
#
# 環境変数 (default.nix から注入):
#   OMNIWMCTL : omniwmctl のフルパス
set -euo pipefail

"$OMNIWMCTL" workspace focus-name M > /dev/null
/usr/bin/open -a Calendar
/usr/bin/open -a Spotify
/usr/bin/open -a Discord
