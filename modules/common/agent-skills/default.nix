# modules/common/agent-skills/default.nix
#
# nix-darwin ラッパーモジュール。flake.nix（外部スキル）と組み合わせて
# ~/.claude/skills/ 等の全ターゲットに 2 種類のスキルを共存させる。
#
# ──────────────────────────────────────────────────────────────────
# スキルの種類と反映のメカニズム
# ──────────────────────────────────────────────────────────────────
#
#   [外部スキル] flake.nix で宣言 → Nix store（読み取り専用）→ home.file シンボリックリンク
#   更新: `nix flake update --update-input skills-catalog` → darwin-rebuild switch
#   反映: darwin-rebuild switch 後
#
#   [個人スキル] ~/dev/my-skills/<name>/ ← skills-sync デーモンが双方向同期
#   更新: ファイルを保存するだけ（シンボリックリンク先が即時変わる）
#   自動 push: gitwatch が 10 秒以内に検知して origin/main へ commit & push
#
# ──────────────────────────────────────────────────────────────────
# 個人スキルを追加する手順（rebuild 不要）:
#
#   A. ~/dev/my-skills/<skill-name>/ を作成する
#      → skills-sync が全ターゲットに即時シンボリックリンクを作成
#      → gitwatch が自動で commit & push
#
#   B. 任意のターゲット（~/.claude/skills/ 等）に実体ディレクトリを作成する
#      → skills-sync が ~/dev/my-skills/ に移動 + 全ターゲットにリンクを作成
#      → gitwatch が自動で commit & push
#
# ──────────────────────────────────────────────────────────────────
{ config, lib, pkgs, inputs, ... }:
let
  cfg = config.myConfig.agentSkills;
  user = config.myConfig.primaryUser;
  homeDir = "/Users/${user}";
  # ~/dev/ は規約として固定（README.md「原則」参照）
  mySkillsDir = "${homeDir}/dev/my-skills";

  # スキルを配置するターゲットディレクトリ一覧（flake.nix の targets と対応）
  skillsTargets = [
    ".claude/skills"
    ".copilot/skills"
    ".gemini/skills"
    ".codex/skills"
    ".cursor/skills"
    ".codeium/windsurf/skills"
    ".gemini/antigravity/skills"
    ".agents/skills"
  ];

  # skills-sync: ~/dev/my-skills/ と全ターゲットを双方向同期する fswatch デーモン。
  #
  # 識別ロジック: readlink が /nix/store/ を指す → Nix 管理（外部スキルまたは HM）→ スキップ
  # それ以外の実体ディレクトリ → 新規個人スキル → ~/dev/my-skills/ に移動して全ターゲットにリンク
  skillsSyncScript = pkgs.writeShellScript "skills-sync" ''
    PERSONAL_SKILLS_DIR="$HOME/dev/my-skills"
    SKILL_TARGETS=(
      ${lib.concatMapStringsSep "\n      " (t: "\"$HOME/${t}\"") skillsTargets}
    )

    # /nix/store/ を指すシンボリックリンクか判定（外部スキル・HM 管理スキルの識別）
    is_nix_managed() {
      local link
      link=$(readlink "$1" 2>/dev/null) || return 1
      [[ "$link" == /nix/store/* ]]
    }

    sync_skills() {
      [ -d "$PERSONAL_SKILLS_DIR" ] || return

      # ① 順方向: ~/dev/my-skills/ → 全ターゲットへシンボリックリンクを作成
      for skill_name in $(ls "$PERSONAL_SKILLS_DIR" 2>/dev/null); do
        [ -d "$PERSONAL_SKILLS_DIR/$skill_name" ] || continue
        for target in "''${SKILL_TARGETS[@]}"; do
          mkdir -p "$target"
          local link="$target/$skill_name"
          [ ! -e "$link" ] && [ ! -L "$link" ] && ln -s "$PERSONAL_SKILLS_DIR/$skill_name" "$link"
        done
      done

      # ② 逆方向: 各ターゲットの実体ディレクトリ → ~/dev/my-skills/ に移動してリンク
      for target in "''${SKILL_TARGETS[@]}"; do
        [ -d "$target" ] || continue
        for skill_name in $(ls "$target" 2>/dev/null); do
          local entry="$target/$skill_name"
          [ -e "$entry" ] || continue

          # Nix 管理（/nix/store/ を指すシンボリックリンク）はスキップ
          # 注: 末尾スラッシュなしで readlink を呼ぶこと（スラッシュ付きだと失敗する）
          is_nix_managed "$entry" && continue

          # ~/dev/my-skills/ へのシンボリックリンクはスキップ（デーモン管理済み）
          local existing_link
          existing_link=$(readlink "$entry" 2>/dev/null)
          [[ "$existing_link" == "$PERSONAL_SKILLS_DIR"* ]] && continue

          # 既に ~/dev/my-skills/ に存在する場合はスキップ
          [ -d "$PERSONAL_SKILLS_DIR/$skill_name" ] && continue

          # 実体ディレクトリを ~/dev/my-skills/ に移動して全ターゲットにリンク
          mv "$entry" "$PERSONAL_SKILLS_DIR/$skill_name"
          for t in "''${SKILL_TARGETS[@]}"; do
            mkdir -p "$t"
            local tlink="$t/$skill_name"
            [ ! -e "$tlink" ] && [ ! -L "$tlink" ] && ln -s "$PERSONAL_SKILLS_DIR/$skill_name" "$tlink"
          done
        done
      done
    }

    # 初回同期（デーモン起動時に既存スキルを全ターゲットに反映）
    sync_skills

    # 監視対象: ~/dev/my-skills/ + 全ターゲット
    watch_dirs=("$PERSONAL_SKILLS_DIR")
    for target in "''${SKILL_TARGETS[@]}"; do
      watch_dirs+=("$target")
    done

    # fswatch: -d でディレクトリイベントのみ監視（ファイル変更は gitwatch に委譲）
    # --latency 2: 2秒のレイテンシでイベントをバースト抑制
    /opt/homebrew/bin/fswatch -0 -d \
      --event Created --event Removed --event Renamed \
      --latency 2 \
      "''${watch_dirs[@]}" | while IFS= read -r -d "" _; do
      sync_skills
    done
  '';
in
{
  options.myConfig.agentSkills.enable = lib.mkEnableOption "Agent Skills (Nix-managed)";

  config = lib.mkIf cfg.enable {
    # gitwatch: Homebrew でインストール（deps の fswatch, coreutils も自動で入る）
    homebrew.brews = [ "gitwatch" ];

    # gitwatch launchd user agent:
    # ~/dev/my-skills/ 以下のファイル変更を監視し、変更後 10 秒で commit & push する。
    # 動作確認: launchctl list | grep gitwatch
    # ログ確認: tail -f ~/.local/log/gitwatch-my-skills.log
    # 前提: gh auth login 済みであること（トークンは macOS Keychain に保存される）
    launchd.user.agents.gitwatch-my-skills = {
      serviceConfig = {
        ProgramArguments = [
          "/opt/homebrew/bin/gitwatch"
          "-s" "10"        # 変更検知後10秒待機してからコミット
          "-r" "origin"    # push 先リモート
          "-b" "main"      # push するブランチ
          mySkillsDir
        ];
        RunAtLoad = true;
        KeepAlive = true;
        # credential helper (gh auth git-credential) が macOS Keychain にアクセスできるよう
        # Homebrew と Nix プロファイルの両方にパスを通す
        EnvironmentVariables = {
          PATH = "/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin:/etc/profiles/per-user/${user}/bin";
          HOME = homeDir;
        };
        StandardOutPath = "${homeDir}/.local/log/gitwatch-my-skills.log";
        StandardErrorPath = "${homeDir}/.local/log/gitwatch-my-skills.log";
      };
    };

    # skills-sync launchd user agent:
    # ~/dev/my-skills/ と全ターゲットを双方向同期する fswatch デーモン。
    # 動作確認: launchctl list | grep skills-sync
    # ログ確認: tail -f ~/.local/log/skills-sync.log
    launchd.user.agents.skills-sync = {
      serviceConfig = {
        ProgramArguments = [ "${skillsSyncScript}" ];
        RunAtLoad = true;
        KeepAlive = true;
        EnvironmentVariables = {
          PATH = "/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin";
          HOME = homeDir;
        };
        StandardOutPath = "${homeDir}/.local/log/skills-sync.log";
        StandardErrorPath = "${homeDir}/.local/log/skills-sync.log";
      };
    };

    # ログディレクトリを事前作成（launchd 起動前に存在しないと失敗する）
    system.activationScripts.gitwatchLogDir.text = ''
      mkdir -p "${homeDir}/.local/log"
    '';

    home-manager.users.${user} = { ... }: {
      # flake.nix が提供する HM モジュール（外部スキルを全ターゲットに配置）
      imports = [ inputs.skills-catalog.homeManagerModules.default ];

      # 個人スキルのシンボリックリンクは skills-sync デーモンが管理するため
      # ここでは宣言しない（rebuild なしで追加・削除できる）
    };
  };
}
