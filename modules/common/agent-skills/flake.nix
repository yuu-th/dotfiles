{
  description = "Agent skills catalog — external skill sources and selection";

  inputs = {
    agent-skills.url = "github:Kyure-A/agent-skills-nix";

    # External skill sources (flake = false → pinned in flake.lock)
    anthropic-skills = {
      url = "github:anthropics/skills";
      flake = false;
    };
    vercel-skills = {
      url = "github:vercel-labs/skills";
      flake = false;
    };
    vercel-agent-browser = {
      url = "github:vercel-labs/agent-browser";
      flake = false;
    };
  };

  outputs = { agent-skills, anthropic-skills, vercel-skills, vercel-agent-browser, ... }: {
    homeManagerModules.default = {
      imports = [ agent-skills.homeManagerModules.default ];

      programs.agent-skills = {
        enable = true;

        # ── Sources ──────────────────────────────────────────────────
        sources.anthropic = {
          path   = anthropic-skills;
          subdir = "skills";
        };
        sources.vercel = {
          path   = vercel-skills;
          subdir = "skills";
        };
        sources.vercel-browser = {
          path   = vercel-agent-browser;
          subdir = "skills";
        };

        # ── Skill selection ──────────────────────────────────────────
        # External skills to enable (by discovered ID)
        skills.enable = [
          "agent-browser"   # from vercel-browser
          "find-skills"     # from vercel
        ];

        # ── Targets (structure = "link" → home.file symlinks) ────────
        # All targets use "link" so personal symlinks can coexist
        targets.claude.enable       = true;
        targets.copilot.enable      = true;
        targets.gemini.enable       = true;
        targets.codex.enable        = true;
        targets.cursor.enable       = true;
        targets.windsurf.enable     = true;
        targets.antigravity.enable  = true;
        targets.agents.enable       = true;
      };
    };
  };
}
