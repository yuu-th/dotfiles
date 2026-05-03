# modules/darwin/projwm/sudoers.nix
#
# projwm 開発・運用用に最小限の管理コマンドを NOPASSWD で許可。
#
# 目的: ユーザ不在時の自走作業中、claude が macOS の TCC (Transparency, Consent,
# Control) DB の検査・調整、launchd 操作、xattr quarantine 削除等を行える
# ようにする（OmniWM-Ghostty visibility bug など、対象不明の調査に必須）。
#
# セキュリティ: 影響範囲は yuta ユーザに限定。Phase 7 撤去完了後にこの module
# 全体を削除する想定（一時的設定）。
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.darwin.projwm;
in {
  config = lib.mkIf cfg.enable {
    # nix-darwin の security.sudo.extraConfig は /etc/sudoers.d/10-nix-darwin-extra-config
    # に書き込まれる（root:wheel 0440、visudo 検証済）。
    security.sudo.extraConfig = ''

      # ── projwm 開発・運用用 nopass 設定 (modules/darwin/projwm/sudoers.nix) ──
      yuta ALL=(ALL) NOPASSWD: /usr/bin/tccutil
      yuta ALL=(ALL) NOPASSWD: /opt/homebrew/bin/tccutil
      yuta ALL=(ALL) NOPASSWD: /usr/bin/sqlite3
      yuta ALL=(ALL) NOPASSWD: /bin/launchctl
      yuta ALL=(ALL) NOPASSWD: /usr/bin/killall
      yuta ALL=(ALL) NOPASSWD: /usr/sbin/spctl
      yuta ALL=(ALL) NOPASSWD: /usr/bin/xattr
      yuta ALL=(ALL) NOPASSWD: /usr/sbin/lsof
      yuta ALL=(ALL) NOPASSWD: /usr/bin/codesign
      yuta ALL=(ALL) NOPASSWD: /bin/cat
      yuta ALL=(ALL) NOPASSWD: /bin/cp
      yuta ALL=(ALL) NOPASSWD: /bin/rm
      yuta ALL=(ALL) NOPASSWD: /bin/mv
      yuta ALL=(ALL) NOPASSWD: /usr/bin/defaults
    '';
  };
}
