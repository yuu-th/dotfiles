# modules/darwin/cmux.nix
#
# cmux AI ターミナル
# - Homebrew Cask でインストール管理
# - CLI を PATH に追加（/Applications/cmux.app/Contents/Resources/bin）
# - settings.json / cmux.json を Nix で管理
#
# ⚠️ 初回ビルド前に ~/.config/cmux/settings.json と ~/.config/cmux/cmux.json が
#    存在する場合は手動で削除すること（home-manager の force = true で上書きされる）
{ config, lib, ... }:
let cfg = config.myConfig.darwin.cmux; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.cmux.enable = lib.mkEnableOption "cmux AI terminal";

  config = lib.mkIf cfg.enable {
    homebrew = {
      taps = [ "manaflow-ai/cmux" ];
      casks = [ "cmux" ];
    };

    # cmux CLI を PATH に追加
    environment.systemPath = [ "/Applications/cmux.app/Contents/Resources/bin" ];

    home-manager.users.${config.myConfig.primaryUser} = {

      # ── settings.json ──────────────────────────────────────────────────
      home.file.".config/cmux/settings.json" = {
        force = true;
        text = ''
          {
            "$schema": "https://raw.githubusercontent.com/manaflow-ai/cmux/main/web/data/cmux-settings.schema.json",
            "schemaVersion": 1,
            "app": {
              "appearance": "dark"
            },
            "automation": {
              "claudeCodeIntegration": true
            }
          }
        '';
      };

      # ── cmux.json（グローバルコマンド定義） ──────────────────────────────
      # "Start AI Dev": 左60% = Zellij AI、右40% = shell/nvim/browser の3サーフェス
      # "Open AI Viewer": 全 AI セッション俯瞰用（レイアウトは手動で構成）
      home.file.".config/cmux/cmux.json" = {
        force = true;
        text = ''
          {
            "commands": [
              {
                "name": "Start AI Dev",
                "keywords": ["ai", "dev", "start", "setup"],
                "restart": "confirm",
                "workspace": {
                  "cwd": ".",
                  "layout": {
                    "direction": "horizontal",
                    "split": 0.6,
                    "children": [
                      {
                        "pane": {
                          "surfaces": [
                            {
                              "type": "terminal",
                              "name": "AI",
                              "command": "zellij attach \"$(basename $(pwd))-ai\" --create",
                              "focus": true
                            }
                          ]
                        }
                      },
                      {
                        "pane": {
                          "surfaces": [
                            {
                              "type": "terminal",
                              "name": "shell",
                              "command": "zellij attach \"$(basename $(pwd))-tools\" --create"
                            },
                            {
                              "type": "terminal",
                              "name": "nvim",
                              "command": "nvim ."
                            },
                            {
                              "type": "browser",
                              "name": "browser",
                              "url": "http://localhost:3000"
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              },
              {
                "name": "Open AI Viewer",
                "keywords": ["viewer", "watch", "monitor", "all"],
                "restart": "confirm",
                "workspace": {
                  "name": "viewer",
                  "cwd": "~"
                }
              }
            ]
          }
        '';
      };
    };
  };
}
