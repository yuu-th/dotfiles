#!/bin/sh
# scripts/setup-kitty-projwm.sh
#
# kitty.app をユーザ空間にコピー + NSPrincipalClass 注入 + ad-hoc 再署名して
# OmniWM 互換にする。homebrew cask の kitty を毎回再ビルド時に検査して
# 必要なら再構築する。
#
# 背景: macOS 26.x Tahoe で OmniWM 0.4.8 は SwiftUI / NSPrincipalClass 未宣言の
# 新規 GUI アプリの window を AX で列挙できないバグがある。kitty も該当（未宣言）。
# 回避策: ユーザ空間に複製して NSPrincipalClass=NSApplication を注入、
# CFBundleIdentifier を分離（衝突回避）、ad-hoc 再署名で起動可能に。
#
# 詳細: queue/projwm-design.md v11.3 / queue/projwm-report.md D-006
set -eu

SOURCE_APP="/Applications/kitty.app"
TARGET_APP="${HOME}/Applications/kitty-projwm.app"
TARGET_BUNDLE_ID="net.kovidgoyal.kitty.projwm"
LOCK_DIR="${HOME}/.cache/projwm"
HASH_FILE="${LOCK_DIR}/kitty-projwm.source-hash"

mkdir -p "${LOCK_DIR}"
mkdir -p "${HOME}/Applications"

# source app の存在確認
if [ ! -d "${SOURCE_APP}" ]; then
  echo "[setup-kitty-projwm] ERROR: ${SOURCE_APP} not found (homebrew cask kitty 未インストール？)" >&2
  exit 1
fi

# 再構築の必要判定: source の inode + mtime のハッシュを取って前回と比較
new_hash=$(stat -f "%i %m" "${SOURCE_APP}/Contents/MacOS/kitty" 2>/dev/null || echo "missing")
old_hash=$(cat "${HASH_FILE}" 2>/dev/null || echo "none")

if [ "${new_hash}" = "${old_hash}" ] && [ -d "${TARGET_APP}" ]; then
  # 既存ターゲットが最新の source と一致、再構築不要
  exit 0
fi

echo "[setup-kitty-projwm] Building ${TARGET_APP} from ${SOURCE_APP}..."

# 既存ターゲット撤去（kitty-projwm 起動中なら quit）
osascript -e "tell application \"${TARGET_APP##*/}\" to quit" 2>/dev/null || true
pkill -9 -f "${TARGET_APP}" 2>/dev/null || true
sleep 1
rm -rf "${TARGET_APP}"

# コピー（cp -R で symlink 構造を維持）
cp -R "${SOURCE_APP}" "${TARGET_APP}"

# quarantine 削除（user-owned なので sudo 不要）
xattr -dr com.apple.quarantine "${TARGET_APP}" 2>/dev/null || true

# 既存署名を削除（再署名のため）
codesign --remove-signature "${TARGET_APP}" 2>/dev/null || true

# Info.plist 修正:
#   1. NSPrincipalClass = NSApplication を注入（OmniWM の AX 列挙を有効化）
#   2. CFBundleIdentifier を分離（純正 kitty との衝突回避）
PLIST="${TARGET_APP}/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Add :NSPrincipalClass string NSApplication" "${PLIST}" 2>/dev/null \
  || /usr/libexec/PlistBuddy -c "Set :NSPrincipalClass NSApplication" "${PLIST}"
/usr/libexec/PlistBuddy -c "Set :CFBundleIdentifier ${TARGET_BUNDLE_ID}" "${PLIST}"

# ad-hoc 再署名（証明書なし、開発者ローカル用途）
codesign --force --deep --sign - "${TARGET_APP}" 2>/dev/null

# キャッシュ更新
echo "${new_hash}" > "${HASH_FILE}"

echo "[setup-kitty-projwm] OK: ${TARGET_APP} (bundleId=${TARGET_BUNDLE_ID})"
