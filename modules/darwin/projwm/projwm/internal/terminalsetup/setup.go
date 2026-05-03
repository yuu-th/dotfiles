// Package terminalsetup は terminal app をユーザ空間に copy + 修正 + 再署名で
// 構築する。OmniWM 0.4.8 + macOS 26.x で SwiftUI 系 / NSPrincipalClass 未宣言の
// 新規 GUI app の window を AX 列挙できないバグ (queue/projwm-design.md v11.3)
// の workaround。
//
// 対応 driver:
//   - kitty:    /Applications/kitty.app   → ~/Applications/kitty-projwm.app
//   - ghostty:  /Applications/Ghostty.app → ~/Applications/Ghostty-projwm.app
//
// home.activation でやると codesign が builderEnv 制約で失敗するため、Go 側で実行する。
// 冪等: source app の inode+mtime が変化していなければ no-op。
//
// ネスト署名: Sparkle.framework / DockTilePlugin / 内部 .app / .xpc を find -depth で
// 見つけて leaf から順に re-sign、最後に外殻。これで Ghostty 1.3.x でも動く。
package terminalsetup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Driver は対応 terminal の種別。
type Driver string

const (
	DriverKitty   Driver = "kitty"
	DriverGhostty Driver = "ghostty"
)

// Spec は driver 固有の path と bundleId。
type Spec struct {
	Driver       Driver
	SourceApp    string // /Applications/kitty.app など
	TargetName   string // kitty-projwm.app など
	TargetBundle string // net.kovidgoyal.kitty.projwm など
}

// SpecForDriver は driver から既定の Spec を返す。
func SpecForDriver(d Driver) (Spec, error) {
	switch d {
	case DriverKitty:
		return Spec{
			Driver:       d,
			SourceApp:    "/Applications/kitty.app",
			TargetName:   "kitty-projwm.app",
			TargetBundle: "net.kovidgoyal.kitty.projwm",
		}, nil
	case DriverGhostty:
		return Spec{
			Driver:       d,
			SourceApp:    "/Applications/Ghostty.app",
			TargetName:   "Ghostty-projwm.app",
			TargetBundle: "com.mitchellh.ghostty.projwm",
		}, nil
	}
	return Spec{}, fmt.Errorf("unknown driver %q", d)
}

const (
	NSPrincipalClassValue = "NSApplication"
	HashCacheRel          = ".cache/projwm"
)

// TargetAppPath は ~/Applications/<TargetName>。
func TargetAppPath(s Spec) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Applications", s.TargetName), nil
}

// EnsureUserSpaceTerminal は driver の terminal app をユーザ空間に構築する。
// 既に最新（hash 一致 + 署名整合）なら no-op。
func EnsureUserSpaceTerminal(s Spec, logger io.Writer) error {
	if logger == nil {
		logger = io.Discard
	}
	target, err := TargetAppPath(s)
	if err != nil {
		return err
	}
	hashFile, err := hashFilePath(s)
	if err != nil {
		return err
	}

	if _, err := os.Stat(s.SourceApp); err != nil {
		return fmt.Errorf("source %s not found: %w", s.SourceApp, err)
	}

	// hash 計算
	srcExe, err := findMainExecutable(s.SourceApp)
	if err != nil {
		return fmt.Errorf("find main executable: %w", err)
	}
	st, err := os.Stat(srcExe)
	if err != nil {
		return fmt.Errorf("stat exe: %w", err)
	}
	newHash := fmt.Sprintf("%d %d", inodeOf(st), st.ModTime().Unix())

	oldHash, _ := os.ReadFile(hashFile)
	if strings.TrimSpace(string(oldHash)) == newHash {
		if _, err := os.Stat(target); err == nil && isValidSigning(target, s.TargetBundle) {
			return nil
		}
		fmt.Fprintf(logger, "[setup-%s] cache hit but bundle invalid, rebuilding\n", s.Driver)
	}

	fmt.Fprintf(logger, "[setup-%s] building %s from %s\n", s.Driver, target, s.SourceApp)

	// 走っているプロセス停止
	_ = exec.Command("pkill", "-9", "-f", target).Run()

	// 既存削除
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove old: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(hashFile), 0o755); err != nil {
		return err
	}

	// cp -R
	if out, err := exec.Command("cp", "-R", s.SourceApp, target).CombinedOutput(); err != nil {
		return fmt.Errorf("cp: %w (%s)", err, string(out))
	}

	// quarantine 削除
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", target).Run()

	// 既存署名 reset (find -depth でネスト構造を leaf から順に処理)
	if err := removeAllSignatures(target, logger); err != nil {
		return fmt.Errorf("remove signatures: %w", err)
	}

	// Info.plist 修正
	plist := filepath.Join(target, "Contents/Info.plist")
	if err := plistAddOrSet(plist, "NSPrincipalClass", "string", NSPrincipalClassValue); err != nil {
		return fmt.Errorf("set NSPrincipalClass: %w", err)
	}
	if err := plistSet(plist, "CFBundleIdentifier", s.TargetBundle); err != nil {
		return fmt.Errorf("set CFBundleIdentifier: %w", err)
	}

	// ネスト署名 (leaf から順に、最後に外殻)
	if err := signNested(target, logger); err != nil {
		return fmt.Errorf("nested sign: %w", err)
	}

	// 検証
	if !isValidSigning(target, s.TargetBundle) {
		return errors.New("post-sign verification failed")
	}

	// hash 保存
	if err := os.WriteFile(hashFile, []byte(newHash+"\n"), 0o644); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}

	fmt.Fprintf(logger, "[setup-%s] OK %s (bundleId=%s)\n", s.Driver, target, s.TargetBundle)
	return nil
}

// EnsureKittyProjwm は後方互換 helper（既存呼出用）。
func EnsureKittyProjwm(logger io.Writer) error {
	s, err := SpecForDriver(DriverKitty)
	if err != nil {
		return err
	}
	return EnsureUserSpaceTerminal(s, logger)
}

// findMainExecutable は app の Contents/MacOS/<name> を返す。
func findMainExecutable(app string) (string, error) {
	macos := filepath.Join(app, "Contents/MacOS")
	entries, err := os.ReadDir(macos)
	if err != nil {
		return "", err
	}
	// 最初に見つかった通常ファイル(executable)を返す
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		return filepath.Join(macos, e.Name()), nil
	}
	return "", fmt.Errorf("no executable in %s", macos)
}

// removeAllSignatures は app 内の全ネスト bundle の署名を取る (leaf から順に)。
func removeAllSignatures(target string, logger io.Writer) error {
	bundles, err := nestedBundles(target)
	if err != nil {
		return err
	}
	// leaf から順 (深い順) に署名削除
	for _, b := range bundles {
		_ = exec.Command("codesign", "--remove-signature", b).Run()
	}
	// 最外殻
	_ = exec.Command("codesign", "--remove-signature", target).Run()
	return nil
}

// signNested は app 内の全ネスト bundle を leaf から順に ad-hoc 再署名し、最後に外殻。
func signNested(target string, logger io.Writer) error {
	bundles, err := nestedBundles(target)
	if err != nil {
		return err
	}
	for _, b := range bundles {
		if out, err := exec.Command("codesign", "--force", "--sign", "-", b).CombinedOutput(); err != nil {
			fmt.Fprintf(logger, "[setup] sign nested %s warn: %v (%s)\n", b, err, strings.TrimSpace(string(out)))
		}
	}
	if out, err := exec.Command("codesign", "--force", "--sign", "-", target).CombinedOutput(); err != nil {
		return fmt.Errorf("sign outer: %w (%s)", err, string(out))
	}
	return nil
}

// nestedBundles は app 配下の .framework / .app / .xpc / .plugin を leaf から順に列挙。
// 同じ bundle に対して find -depth と同等の効果を狙う。
func nestedBundles(root string) ([]string, error) {
	var bundles []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 無視
		}
		if !d.IsDir() {
			return nil
		}
		if path == root {
			return nil
		}
		// 各 nested bundle を集める
		base := d.Name()
		switch {
		case strings.HasSuffix(base, ".framework"),
			strings.HasSuffix(base, ".app"),
			strings.HasSuffix(base, ".xpc"),
			strings.HasSuffix(base, ".plugin"),
			strings.HasSuffix(base, ".bundle"):
			bundles = append(bundles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// leaf 優先 (深い path 先): WalkDir は親が先に来るので、深さ降順にする
	// path のスラッシュ数で簡易 sort
	for i := 0; i < len(bundles); i++ {
		for j := i + 1; j < len(bundles); j++ {
			if depth(bundles[i]) < depth(bundles[j]) {
				bundles[i], bundles[j] = bundles[j], bundles[i]
			}
		}
	}
	return bundles, nil
}

func depth(p string) int {
	return strings.Count(p, string(os.PathSeparator))
}

// plistAddOrSet は :key を type で値設定（既存なら Set）。
func plistAddOrSet(plist, key, ptype, value string) error {
	cmd := exec.Command("/usr/libexec/PlistBuddy", "-c",
		fmt.Sprintf("Add :%s %s %s", key, ptype, value), plist)
	if err := cmd.Run(); err != nil {
		// 既に存在 → Set
		return plistSet(plist, key, value)
	}
	return nil
}

func plistSet(plist, key, value string) error {
	cmd := exec.Command("/usr/libexec/PlistBuddy", "-c",
		fmt.Sprintf("Set :%s %s", key, value), plist)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PlistBuddy Set %s: %w (%s)", key, err, string(out))
	}
	return nil
}

// isValidSigning は署名整合性 + Identifier が target bundle id と一致するかを確認。
func isValidSigning(app, wantBundleID string) bool {
	if err := exec.Command("codesign", "--verify", "--deep", "--strict", app).Run(); err != nil {
		return false
	}
	out, err := exec.Command("codesign", "-dv", app).CombinedOutput()
	if err != nil {
		return false
	}
	want := "Identifier=" + wantBundleID
	return strings.Contains(string(out), want)
}

func hashFilePath(s Spec) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, HashCacheRel, fmt.Sprintf("%s.source-hash", s.TargetName)), nil
}
