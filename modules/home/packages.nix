{ pkgs, ... }:

{
  home.packages = with pkgs; [
    # 言語ツールチェーン
    rustc
    cargo
    go
    python3
    nodejs_22

    # CLI 系
    git
    gh
    jq
    google-cloud-sdk
    ripgrep
    fd
    fzf
    htop

    # インフラ系
    terraform
    tflint

    # エディタ / IDE
    antigravity
  ];

  programs.zsh.shellAliases = {
    tf      = "terraform";
    tfi     = "terraform init";
    tfa     = "terraform apply";
    tfplan  = "terraform plan";
  };
}
