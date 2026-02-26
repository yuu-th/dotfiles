{ pkgs, ... }:

{
  home.packages = with pkgs; [
    terraform
    tflint
    google-cloud-sdk
  ];

  programs.zsh.shellAliases = {
    tf      = "terraform";
    tfi     = "terraform init";
    tfa     = "terraform apply";
    tfplan  = "terraform plan";
  };
}
