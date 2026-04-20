# modules/common/agent-skills/flake.nix
#
# チャイルドフレーク: 外部スキルの宣言専用。
# メイン flake.nix から `skills-catalog.url = "path:./modules/common/agent-skills"` で参照される。
#
# このフレークの役割:
#   - 外部スキルリポジトリを inputs として取り込む（flake = false でコミットをピン留め）
#   - どのスキルを有効化するかを宣言する
#   - 配置先ターゲット（.claude/skills/, .copilot/skills/ 等）を宣言する
#
# 個人スキルの管理は default.nix 側で行う（~/dev/my-skills/ への直接シンボリックリンク）。
#
# ──────────────────────────────────────────────────────────────────
# 外部スキルを追加する手順:
#   1. inputs に新しいリポジトリを追加（flake = false）
#   2. outputs.homeManagerModules.default 内の sources に登録
#   3. skills.enable に有効化したいスキルの ID を追加
#   4. git add → nix flake update --update-input skills-catalog → darwin-rebuild switch
#
# 外部スキルを更新する手順（最新コミットに追従):
#   cd modules/common/agent-skills && nix flake update <input-name>
#   または全 input 一括: nix flake update
#   ※ 更新されるのは modules/common/agent-skills/flake.lock（メインの flake.lock ではない）
# ──────────────────────────────────────────────────────────────────
{
  description = "Agent skills catalog — external skill sources and selection";

  inputs = {
    # agent-skills-nix: Home Manager モジュールを提供するフレーム
    # https://github.com/Kyure-A/agent-skills-nix
    agent-skills.url = "github:Kyure-A/agent-skills-nix";

    # 外部スキルソース（flake = false: Nix ストアに tarball として取り込む）
    # flake.lock でコミット SHA をピン留めするため、自分のリポジトリへのコピー不要
    anthropic-skills = {
      url = "github:anthropics/skills";
      flake = false;
    };
    vercel-skills = {
      url = "github:vercel-labs/skills";
      flake = false;
    };
    # vercel-agent-browser: browser-use の公式スキル（agent-browser として使用）
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

        # ── Sources: 外部リポジトリとスキルディレクトリの対応 ────────────────
        # subdir: リポジトリ内でスキルファイルが置かれているサブディレクトリ名
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

        # ── Skill selection: 有効化する外部スキルの ID ──────────────────────
        # スキル ID は各リポジトリのスキルディレクトリ名（SKILL.md が置かれているフォルダ名）
        skills.enable = [
          "agent-browser"   # vercel-browser から: ブラウザ自動化エージェント
          "find-skills"     # vercel から: スキルを検索・発見するメタスキル
        ];

        # ── Targets: スキルを配置するディレクトリ ───────────────────────────
        # structure = "link" を使うことで home.file のシンボリックリンクとして配置される。
        # "link" 方式にすると個人スキル（~/dev/my-skills/）の直接シンボリックリンクと共存できる。
        # "copy" にすると Nix store から rsync でコピーされ、個人スキルが上書きされてしまうため使わない。
        targets.claude.enable       = true;  # ~/.claude/skills/
        targets.copilot.enable      = true;  # ~/.copilot/skills/
        targets.gemini.enable       = true;  # ~/.gemini/skills/
        targets.codex.enable        = true;  # ~/.codex/skills/
        targets.cursor.enable       = true;  # ~/.cursor/skills/
        targets.windsurf.enable     = true;  # ~/.codeium/windsurf/skills/
        targets.antigravity.enable  = true;  # ~/.gemini/antigravity/skills/
        targets.agents.enable       = true;  # ~/.agents/skills/
      };
    };
  };
}
