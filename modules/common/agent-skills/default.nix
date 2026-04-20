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
#   [個人スキル] ~/dev/my-skills/<name>/ → mkOutOfStoreSymlink → 全ターゲット
#   更新: ファイルを保存するだけ（シンボリックリンク先が即時変わる）
#   自動 push: gitwatch が 10 秒以内に検知して origin/main へ commit & push
#
# ──────────────────────────────────────────────────────────────────
# 個人スキルを追加する手順:
#   1. ~/dev/my-skills/<skill-name>/SKILL.md を作成
#      → gitwatch が自動で commit & push（手動 git 操作不要）
#   2. 下の personalSkills リストに "<skill-name>" を追加
#   3. git add → darwin-rebuild switch
#      → 全ターゲット（.claude/skills/ 等）にシンボリックリンクが張られる
# ──────────────────────────────────────────────────────────────────
{ config, lib, inputs, ... }:
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

  # 個人スキル: ~/dev/my-skills/ 以下のディレクトリ名を列挙する。
  # ここに追加すると全ターゲットに mkOutOfStoreSymlink が張られ、即時編集が反映される。
  # 追加手順は このファイル冒頭のコメントを参照。
  personalSkills = [
    "browser-use"
    "my_gemini_cli"
  ];
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
          "-s" "10"        # 変更検知後10秒待機してからコミット（/opt/homebrew は Apple Silicon 固定パス）
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

    # ログディレクトリを事前作成（launchd 起動前に存在しないと失敗する）
    system.activationScripts.gitwatchLogDir.text = ''
      mkdir -p "${homeDir}/.local/log"
    '';

    # config.lib.file.mkOutOfStoreSymlink は HM モジュールコンテキスト内でのみ使える。
    # { config, ... }: の関数形式にすることで HM の config を受け取る。
    home-manager.users.${user} = { config, ... }:
    let
      # personalSkills × skillsTargets の直積でシンボリックリンクの attrset を生成する。
      # mkOutOfStoreSymlink: Nix store 外のパスへのシンボリックリンクを作成する HM 関数。
      # 通常の home.file.source は Nix store 内のパスのみ受け付けるが、これで mutable パスを指定できる。
      personalSymlinks = builtins.listToAttrs (lib.concatMap (target:
        map (skill: {
          name  = "${target}/${skill}";
          value = {
            source = config.lib.file.mkOutOfStoreSymlink "${mySkillsDir}/${skill}";
          };
        }) personalSkills
      ) skillsTargets);
    in {
      # flake.nix が提供する HM モジュール（外部スキルを全ターゲットに配置）
      imports = [ inputs.skills-catalog.homeManagerModules.default ];

      # 個人スキル: ~/dev/my-skills/<skill>/ → 全ターゲットへの直接シンボリックリンク
      # ファイル保存が即座に反映される（Nix rebuild 不要）
      home.file = personalSymlinks;
    };
  };
}
