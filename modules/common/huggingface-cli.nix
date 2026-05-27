# modules/common/huggingface-cli.nix
{ config, lib, pkgs, ... }:
let
  cfg = config.myConfig.huggingfaceCli;
  # upstream は v1.0 で `huggingface-cli` を `hf` にリネーム済み。
  # 旧名で叩く習慣 / ドキュメントが多いので互換 symlink を追加する。
  huggingface-cli = pkgs.symlinkJoin {
    name = "huggingface-cli";
    paths = [ pkgs.python312Packages.huggingface-hub ];
    postBuild = ''
      ln -s $out/bin/hf $out/bin/huggingface-cli
    '';
  };
in {
  options.myConfig.huggingfaceCli.enable = lib.mkEnableOption "Hugging Face CLI (hf / tiny-agents)";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ huggingface-cli ];
    };
  };
}
