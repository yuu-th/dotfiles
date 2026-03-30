# modules/common/terraform.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.terraform; in {
  options.myConfig.terraform.enable = lib.mkEnableOption "Terraform infrastructure tooling";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = with pkgs; [ terraform tflint ];
      programs.zsh.shellAliases = {
        tf     = "terraform";
        tfi    = "terraform init";
        tfa    = "terraform apply";
        tfplan = "terraform plan";
      };
    };
  };
}
