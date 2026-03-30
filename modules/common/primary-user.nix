{ lib, ... }: {
  options.myConfig.primaryUser = lib.mkOption {
    type        = lib.types.str;
    description = "Primary user of this system";
  };
}
