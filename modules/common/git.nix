# modules/common/git.nix
{ config, lib, ... }:
let
  cfg  = config.myConfig.git;
  user = config.myConfig.primaryUser;
in {
  options.myConfig.git.enable = lib.mkEnableOption "git with user config";

  config = lib.mkIf cfg.enable {
    home-manager.users.${user} = {
      programs.git = {
        enable = true;
        settings = {
          user = {
            name  = "yuu-th";
            email = "88813495+yuu-th@users.noreply.github.com";
          };
          # ローカルオーバーライド: ~/.config/git/local が存在すれば読む（非公開設定用）。
          # 例: ~/dev/work/ 以下で別アカウントを使う場合はここに includeIf を書く。
          # ファイルが存在しない場合は git がサイレントに無視する。
          include.path = "~/.config/git/local";
          # gh auth git-credential を credential helper として宣言することで
          # 新規 Mac でのセットアップ後も git push が動く（gh auth login は依然必要）。
          # 先頭の空文字列でデフォルト helper をリセットしてから gh を指定する（gh の慣例）。
          "credential \"https://github.com\"".helper = [
            ""
            "!/etc/profiles/per-user/${user}/bin/gh auth git-credential"
          ];
          "credential \"https://gist.github.com\"".helper = [
            ""
            "!/etc/profiles/per-user/${user}/bin/gh auth git-credential"
          ];
        };
      };
    };
  };
}
