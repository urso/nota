---
name: Lua Style Guide
applies-to:
  - lua
tags:
  - style
paths:
  - "nvim/**/*.lua"
description: Conventions for Neovim plugin Lua code
---

# Lua Style Guide

## API Design

- Return internal references, not copies — trust callers not to mutate (idiomatic Lua)
- Use `vim.schedule` for deferred execution, not coroutines unless actually yielding
- Guard module-level side effects (subscriptions, handlers) against hot-reload with a registered flag

## Async Patterns

- Use generation counters to guard against stale async responses overwriting fresh data
- Clear state inside async callback (after response arrives), not before dispatching request

## Resource Management

- Register `BufWipeout` autocmd to clean up buffer-keyed state tables
- Neovim extmarks persist until explicitly deleted — clean them on detach/refresh
- Extmarks survive line deletion (relocate to adjacent line) — accept drift between save cycles

## Naming

- Use snake_case for functions and variables
- Prefix internal/test-only functions with `_` (e.g., `M._reset()`, `M._get_namespace()`)
