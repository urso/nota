-- Model layer for per-buffer thread state.
-- Holds threads anchored in attached buffers, manages extmarks, emits change events.

local client = require('nota.client')
local repo_utils = require('nota.repo')

local M = {}

local buffers = {}

function M.attach(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()

  if buffers[bufnr] then
    return true
  end

  local abs_path = vim.api.nvim_buf_get_name(bufnr)
  if abs_path == '' then
    return false
  end

  local repo = repo_utils.get_root(abs_path)
  if not repo then
    return false
  end

  if not repo_utils.has_nota_dir(repo) then
    return false
  end

  local rel_path = repo_utils.get_relative_path(abs_path, repo)

  buffers[bufnr] = {
    repo = repo,
    file = rel_path,
    threads = {},
    marks = {},
  }

  local co = coroutine.create(function()
    local response = client.list(repo, { file = rel_path })
    if response.err then
      vim.notify('nota: failed to list threads: ' .. response.err.message, vim.log.levels.WARN)
      return
    end
    if buffers[bufnr] then
      buffers[bufnr].threads = response.result or {}
    end
  end)

  vim.schedule(function()
    coroutine.resume(co)
  end)

  return true
end

function M.detach(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()

  local state = buffers[bufnr]
  if not state then
    return false
  end

  buffers[bufnr] = nil
  return true
end

function M.is_attached(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  return buffers[bufnr] ~= nil
end

function M.get_state(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  return buffers[bufnr]
end

return M
