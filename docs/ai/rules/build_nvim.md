---
name: Neovim Plugin Build
applies-to:
  - lua
  - neovim
tags:
  - build
paths:
  - nvim/**
description: Build and lint commands for the Neovim plugin
---

# Neovim Plugin Build

- Run `make nvim-lint` to lint Lua code with selene
- Run `make nvim-check` to type-check with lua-language-server
- Run both from repo root; they cd into `nvim/` internally
- Config files live in `nvim/`: `selene.toml`, `vim.yml`, `.luarc.json`
