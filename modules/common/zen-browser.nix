# modules/common/zen-browser.nix
{ config, lib, inputs, ... }:
let cfg = config.myConfig.zenBrowser; in {
  options.myConfig.zenBrowser.enable = lib.mkEnableOption "Zen Browser";

  config = lib.mkIf cfg.enable {
    home-manager.sharedModules = [ inputs.zen-browser.homeModules.default ];

    home-manager.users.${config.myConfig.primaryUser} = {
      programs.zen-browser = {
        enable = true;
        profiles.default = {
          id        = 0;
          isDefault = true;
          search = {
            force = true;
            engines = {
              "Perplexity" = {
                urls = [{ template = "https://www.perplexity.ai/search?q={searchTerms}"; }];
                keyword = "p";
              };
            };
          };
        };
      };
    };
  };
}
