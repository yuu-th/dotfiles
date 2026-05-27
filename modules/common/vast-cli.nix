# modules/common/vast-cli.nix
{ config, lib, inputs, pkgs, ... }:
let
  cfg = config.myConfig.vast-cli;

  inherit (inputs) vast-cli-src uv2nix pyproject-nix pyproject-build-systems;

  # 上流 vast-cli の source に、自前で生成した uv.lock を合流させた
  # 仮の workspace を構築する。uv.lock は darwin-up が `nix flake update
  # vast-cli-src` の後に `uv lock` で再生成して commit する。
  workspaceRoot = pkgs.runCommandLocal "vast-cli-workspace" { } ''
    cp -r ${vast-cli-src} $out
    chmod -R u+w $out
    cp ${./vast-cli/uv.lock} $out/uv.lock
  '';

  workspace = uv2nix.lib.workspace.loadWorkspace { inherit workspaceRoot; };
  overlay = workspace.mkPyprojectOverlay { sourcePreference = "wheel"; };

  python = pkgs.python312;
  pythonSet =
    (pkgs.callPackage pyproject-nix.build.packages { inherit python; }).overrideScope
      (lib.composeManyExtensions [
        pyproject-build-systems.overlays.wheel
        overlay
      ]);

  vastai-env = pythonSet.mkVirtualEnv "vastai-env" workspace.deps.default;
  # venv 全体を home.packages に入れると bin/python3 が既存の python と衝突するので、
  # vastai / serve-vast-deployment の実行可能ファイルだけを露出させた wrapper を返す。
  vast-cli = pkgs.runCommandLocal "vast-cli" { } ''
    mkdir -p $out/bin
    ln -s ${vastai-env}/bin/vastai $out/bin/vastai
    ln -s ${vastai-env}/bin/serve-vast-deployment $out/bin/serve-vast-deployment
  '';
in {
  options.myConfig.vast-cli.enable = lib.mkEnableOption "vast.ai CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser}.home.packages = [ vast-cli ];
  };
}
