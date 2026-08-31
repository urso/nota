-- Repository utilities for nota plugin.

local M = {}

function M.get_root(path)
  local cmd = 'git rev-parse --show-toplevel'
  if path then
    local dir
    if vim.fn.isdirectory(path) == 1 then
      dir = path
    else
      dir = vim.fn.fnamemodify(path, ':h')
    end
    cmd = 'git -C ' .. vim.fn.shellescape(dir) .. ' rev-parse --show-toplevel'
  end
  local result = vim.fn.systemlist(cmd)
  if vim.v.shell_error ~= 0 or #result == 0 then
    return nil
  end
  return result[1]
end

function M.has_nota_dir(repo)
  return vim.fn.isdirectory(repo .. '/.nota') == 1
end

function M.get_relative_path(abs_path, repo)
  repo = repo:gsub('/$', '')
  if abs_path:sub(1, #repo + 1) == repo .. '/' then
    return abs_path:sub(#repo + 2)
  end
  return abs_path
end

return M
