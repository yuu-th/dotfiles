// Package terminalsetup は kitty-projwm.app をユーザ空間に user-space copy + 再署名で
// 構築する。OmniWM 0.4.8 + macOS 26.x で SwiftUI 系 / NSPrincipalClass 未宣言の
// 新規 GUI app の window を AX 列挙できないバグ (queue/projwm-design.md v11.3)
// の workaround。
//
// home.activation でやると codesign が builderEnv 制約で失敗するため、Go 側で実行する。
// 冪等: source kitty.app の inode+mtime が変化していなければ no-op。
package terminalsetup

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	SourceApp        = "/Applications/kitty.app"
	TargetAppRel     = "Applications/kitty-projwm.app"
	TargetBundleID   = "net.kovidgoyal.kitty.projwm"
	HashCacheRel     = ".cache/projwm/kitty-projwm.source-hash"
	NSPrincipalClass = "NSApplication"
)

// EnsureKittyProjwm は ~/Applications/kitty-projwm.app を user-space copy で構築する。
// 既に最新なら no-op。logger に進捗を書き込む（nil なら捨てる）。
func EnsureKittyProjwm(logger io.Writer) error {
	if logger == nil {
		logger = io.Discard
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	target := filepath.Join(home, TargetAppRel)
	hashFile := filepath.Join(home, HashCacheRel)

	if _, err := os.Stat(SourceApp); err != nil {
		return fmt.Errorf("source %s not found: %w (homebrew cask kitty 未インストール？)", SourceApp, err)
	}

	// 現在の source hash
	srcExe := filepath.Join(SourceApp, "Contents/MacOS/kitty")
	st, err := os.Stat(srcExe)
	if err != nil {
		return fmt.Errorf("stat source executable: %w", err)
	}
	newHash := fmt.Sprintf("%d %d", inodeOf(st), st.ModTime().Unix())

	// hash file 比較
	oldHash, _ := os.ReadFile(hashFile)
	if strings.TrimSpace(string(oldHash)) == newHash {
		if _, err := os.Stat(target); err == nil {
			// 既に最新 + 整合性 check
			if isValidSigning(target) {
				return nil
			}
			fmt.Fprintf(logger, "[setup-kitty] target signing invalid; rebuilding\n")
		}
	}

	fmt.Fprintf(logger, "[setup-kitty] building %s from %s\n", target, SourceApp)

	// 走っている kitty-projwm を quit
	_ = exec.Command("osascript", "-e", `tell application "kitty-projwm" to quit`).Run()
	_ = exec.Command("pkill", "-9", "-f", target).Run()

	// 既存 target 削除
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove old target: %w", err)
	}

	// targetDir 作成
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir target parent: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(hashFile), 0o755); err != nil {
		return fmt.Errorf("mkdir hash dir: %w", err)
	}

	// cp -R
	if out, err := exec.Command("cp", "-R", SourceApp, target).CombinedOutput(); err != nil {
		return fmt.Errorf("cp -R: %w (%s)", err, string(out))
	}

	// quarantine 削除
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", target).Run()

	// 既存署名削除
	if out, err := exec.Command("codesign", "--remove-signature", target).CombinedOutput(); err != nil {
		fmt.Fprintf(logger, "[setup-kitty] codesign --remove-signature warn: %v (%s)\n", err, string(out))
	}

	plist := filepath.Join(target, "Contents/Info.plist")

	// NSPrincipalClass を Add（既存があれば Set）
	addCmd := exec.Command("/usr/libexec/PlistBuddy", "-c",
		fmt.Sprintf("Add :NSPrincipalClass string %s", NSPrincipalClass), plist)
	if err := addCmd.Run(); err != nil {
		// 既存ならば Set にフォールバック
		setCmd := exec.Command("/usr/libexec/PlistBuddy", "-c",
			fmt.Sprintf("Set :NSPrincipalClass %s", NSPrincipalClass), plist)
		if out, err2 := setCmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("set NSPrincipalClass: %w (%s)", err2, string(out))
		}
	}

	// CFBundleIdentifier 変更
	if out, err := exec.Command("/usr/libexec/PlistBuddy", "-c",
		fmt.Sprintf("Set :CFBundleIdentifier %s", TargetBundleID), plist).CombinedOutput(); err != nil {
		return fmt.Errorf("set CFBundleIdentifier: %w (%s)", err, string(out))
	}

	// ad-hoc 再署名
	if out, err := exec.Command("codesign", "--force", "--deep", "--sign", "-", target).CombinedOutput(); err != nil {
		return fmt.Errorf("ad-hoc sign: %w (%s)", err, string(out))
	}

	// 整合性検証
	if !isValidSigning(target) {
		return fmt.Errorf("post-sign verification failed; codesign --verify rejected the bundle")
	}

	// hash 保存
	if err := os.WriteFile(hashFile, []byte(newHash+"\n"), 0o644); err != nil {
		return fmt.Errorf("write hash cache: %w", err)
	}

	fmt.Fprintf(logger, "[setup-kitty] OK %s (bundleId=%s)\n", target, TargetBundleID)
	return nil
}

// isValidSigning は app bundle の署名 + Identifier 整合性を確認。
func isValidSigning(app string) bool {
	// 1) verify deep strict
	if err := exec.Command("codesign", "--verify", "--deep", "--strict", app).Run(); err != nil {
		return false
	}
	// 2) Identifier が target bundle id と一致するか
	out, err := exec.Command("codesign", "-dv", app).CombinedOutput()
	if err != nil {
		return false
	}
	want := "Identifier=" + TargetBundleID
	return strings.Contains(string(out)+"\n"+string(out), want)
}
