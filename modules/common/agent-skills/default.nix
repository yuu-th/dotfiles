# modules/common/agent-skills/default.nix
#
# Nix-managed agent skills: external skills via agent-skills-nix,
# personal skills via direct symlinks for immediate editing.
# gitwatch: auto-commit & push ~/dev/my-skills/ on file changes.
{ config, lib, inputs, ... }:
let
  cfg = config.myConfig.agentSkills;
  user = config.myConfig.primaryUser;
  homeDir = "/Users/${user}";
  mySkillsDir = "${homeDir}/dev/my-skills";

  # All target directories where skills are deployed
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

  # Personal skills: symlinked directly for immediate editing
  personalSkills = [
    "browser-use"
    "my_gemini_cli"
  ];
in
{
  options.myConfig.agentSkills.enable = lib.mkEnableOption "Agent Skills (Nix-managed)";

  config = lib.mkIf cfg.enable {
    # gitwatch: Homebrew でインストール（fswatch, coreutils も自動で入る）
    homebrew.brews = [ "gitwatch" ];

    # gitwatch launchd user agent: ~/dev/my-skills/ の変更を自動 commit & push
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
        # credential helper (gh auth git-credential) がキーチェーンにアクセスできるよう
        # Homebrew と git の両方にパスを通す
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

    # HM module context: config.lib.file.mkOutOfStoreSymlink is available here
    home-manager.users.${user} = { config, ... }:
    let
      personalSymlinks = builtins.listToAttrs (lib.concatMap (target:
        map (skill: {
          name  = "${target}/${skill}";
          value = {
            source = config.lib.file.mkOutOfStoreSymlink "${mySkillsDir}/${skill}";
          };
        }) personalSkills
      ) skillsTargets);
    in {
      imports = [ inputs.skills-catalog.homeManagerModules.default ];

      # Personal skills: direct symlinks for immediate reflection
      home.file = personalSymlinks;
    };
  };
}
