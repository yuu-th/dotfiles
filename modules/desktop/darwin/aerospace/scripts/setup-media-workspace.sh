#!/usr/bin/env bash

# AeroSpace ユーティリティ: 右下モニタ(WS M)のレイアウト自動建築スクリプト
# 【目的】 Spotify/Discordのアコーディオン(左) ＋ カレンダー(右/幅狭) を一瞬で作成する

# 1. ワークスペースMに移動
aerospace workspace M

# 2. 必要な3つのアプリを起動
open -a Calendar
open -a Spotify
open -a Discord

# 3. ウィンドウが出現するまで少し待機 (最大10秒)
echo "Waiting for apps to appear..."
for i in {1..20}; do
  # Workspace Mに存在するアプリの bundle-id を取得
  APPS=$(aerospace list-windows --workspace M --format "%{app-bundle-id}")
  
  HAS_CAL=$(echo "$APPS" | grep -c "com.apple.iCal")
  HAS_SPO=$(echo "$APPS" | grep -c "com.spotify.client")
  HAS_DIS=$(echo "$APPS" | grep -c "com.hnc.Discord")
  
  if [ "$HAS_CAL" -gt 0 ] && [ "$HAS_SPO" -gt 0 ] && [ "$HAS_DIS" -gt 0 ]; then
    break
  fi
  sleep 0.5
done

# アプリの初期化とタイリング配置の安定を少し待つ
sleep 1

# 各アプリの window-id を取得する関数
get_window_id() {
  aerospace list-windows --workspace M --format "%{window-id} %{app-bundle-id}" | grep "$1" | awk '{print $1}' | head -n 1
}

WID_CAL=$(get_window_id "com.apple.iCal")
WID_SPO=$(get_window_id "com.spotify.client")
WID_DIS=$(get_window_id "com.hnc.Discord")

if [ -z "$WID_CAL" ] || [ -z "$WID_SPO" ] || [ -z "$WID_DIS" ]; then
  echo "Error: Not all required apps are open in Workspace M."
  exit 1
fi

# 4. カレンダーを「一番右」に寄せる
echo "Positioning Calendar..."
aerospace focus --window-id "$WID_CAL"
aerospace move right
aerospace move right

# 5. Discordを「一番左」に寄せる (一時的)
echo "Positioning Discord..."
aerospace focus --window-id "$WID_DIS"
aerospace move left
aerospace move left

# 6. Spotifyを「一番左」に寄せる (Discordが中央に押し出される)
# これで左から [Spotify] | [Discord] | [Calendar] の並びが確定する
echo "Positioning Spotify..."
aerospace focus --window-id "$WID_SPO"
aerospace move left
aerospace move left

# 7. SpotifyとDiscordを合体させる (アコーディオン化)
echo "Building Accordion..."
aerospace focus --window-id "$WID_SPO"
# 右隣(=Discord)と合体させて1つのコンテナにする
aerospace join-with right
# そのコンテナをアコーディオン・レイアウトに変更する
aerospace layout accordion

# 8. カレンダーの幅を狭くする
echo "Resizing Calendar..."
aerospace focus --window-id "$WID_CAL"
# 50pxずつ何度か縮小コマンドを送る (環境に合わせて回数を調整可能)
for i in {1..6}; do
  aerospace resize width -50
done

# 9. 最後にSpotifyを手前にして完了
aerospace focus --window-id "$WID_SPO"

echo "Layout successfully built!"
