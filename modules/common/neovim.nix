# modules/common/neovim.nix
#
# Neovim — コードビューア / エディタ
# Zellij の Editor タブで yazi と並列使用（VSCode 代替）
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.neovim; in {
  options.myConfig.neovim.enable = lib.mkEnableOption "Neovim editor";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      programs.neovim = {
        enable = true;
        defaultEditor = true;
        viAlias  = true;
        vimAlias = true;

        plugins = with pkgs.vimPlugins; [
          # テーマ
          tokyonight-nvim
          # ファイルツリー（yazi メインだが補助として）
          nvim-tree-lua
          nvim-web-devicons
          # LSP / 補完
          nvim-lspconfig
          nvim-cmp
          cmp-nvim-lsp
          # シンタックスハイライト
          (nvim-treesitter.withPlugins (p: with p; [
            nix lua python go typescript javascript json yaml toml markdown bash
          ]))
          # Git
          gitsigns-nvim
          # ステータスライン
          lualine-nvim
        ];

        initLua = ''
          -- ── 基本設定 ──────────────────────────────────────────────────────
          vim.opt.number         = true
          vim.opt.relativenumber = true
          vim.opt.expandtab      = true
          vim.opt.tabstop        = 2
          vim.opt.shiftwidth     = 2
          vim.opt.wrap           = false
          vim.opt.scrolloff      = 8
          vim.opt.signcolumn     = "yes"
          vim.opt.updatetime     = 250
          vim.opt.clipboard      = "unnamedplus"  -- システムクリップボード連携
          vim.opt.termguicolors  = true

          -- ── テーマ ────────────────────────────────────────────────────────
          require("tokyonight").setup({ style = "night", transparent = true })
          vim.cmd.colorscheme("tokyonight")

          -- ── Treesitter ────────────────────────────────────────────────────
          require("nvim-treesitter.configs").setup({
            highlight = { enable = true },
            indent    = { enable = true },
          })

          -- ── Lualine ───────────────────────────────────────────────────────
          require("lualine").setup({ options = { theme = "tokyonight" } })

          -- ── Gitsigns ──────────────────────────────────────────────────────
          require("gitsigns").setup()

          -- ── nvim-tree ─────────────────────────────────────────────────────
          require("nvim-tree").setup({
            view = { width = 30 },
            renderer = { group_empty = true },
          })

          -- ── キーマップ ────────────────────────────────────────────────────
          vim.g.mapleader = " "
          local map = vim.keymap.set

          -- ファイルツリー
          map("n", "<leader>e", "<cmd>NvimTreeToggle<cr>", { desc = "Toggle file tree" })

          -- ウィンドウ移動
          map("n", "<C-h>", "<C-w>h")
          map("n", "<C-j>", "<C-w>j")
          map("n", "<C-k>", "<C-w>k")
          map("n", "<C-l>", "<C-w>l")

          -- バッファ
          map("n", "<leader>q", "<cmd>bd<cr>", { desc = "Close buffer" })
          map("n", "<Tab>",     "<cmd>bnext<cr>")
          map("n", "<S-Tab>",   "<cmd>bprev<cr>")
        '';
      };
    };
  };
}
