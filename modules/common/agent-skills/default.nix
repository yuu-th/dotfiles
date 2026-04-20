# modules/common/agent-skills/default.nix
#
# Nix-managed agent skills: external skills via agent-skills-nix,
# personal skills via direct symlinks for immediate editing.
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
