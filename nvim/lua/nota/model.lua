-- Model layer for per-buffer thread state.
-- Holds threads anchored in attached buffers, manages extmarks, emits change events.

local client = require('nota.client')
local repo_utils = require('nota.repo')

local M = {}

local buffers = {}
local ns = vim.api.nvim_create_namespace('nota')

function M._reset()
  for bufnr, _ in pairs(buffers) do
    if vim.api.nvim_buf_is_valid(bufnr) then
      vim.api.nvim_buf_clear_namespace(bufnr, ns, 0, -1)
    end
  end
  buffers = {}
end

function M._get_namespace()
  return ns
end

local function place_marks(bufnr, state)
  if not vim.api.nvim_buf_is_valid(bufnr) then
    return
  end
  local line_count = vim.api.nvim_buf_line_count(bufnr)
  for _, thread in ipairs(state.threads) do
    if thread.resolvedAnchor and thread.resolvedAnchor.line and thread.resolvedAnchor.line > 0 then
      local line = math.min(thread.resolvedAnchor.line, line_count)
      local mark_id = vim.api.nvim_buf_set_extmark(bufnr, ns, line - 1, 0, {})
      state.marks[thread.id] = mark_id
    end
  end
end

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
      place_marks(bufnr, buffers[bufnr])
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

  if vim.api.nvim_buf_is_valid(bufnr) then
    vim.api.nvim_buf_clear_namespace(bufnr, ns, 0, -1)
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
  local state = buffers[bufnr]
  if not state then
    return nil
  end
  return {
    repo = state.repo,
    file = state.file,
    threads = state.threads,
    marks = state.marks,
  }
end

function M._get_state_internal(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  return buffers[bufnr]
end

function M.get_threads(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  local state = buffers[bufnr]
  if not state then
    return {}
  end

  if not vim.api.nvim_buf_is_valid(bufnr) then
    return {}
  end

  local all_marks = vim.api.nvim_buf_get_extmarks(bufnr, ns, 0, -1, {})
  local mark_pos = {}
  for _, m in ipairs(all_marks) do
    mark_pos[m[1]] = m[2]
  end

  local result = {}
  for _, thread in ipairs(state.threads) do
    local entry = {
      id = thread.id,
      title = thread.title,
      status = thread.status,
      resolvedAnchor = thread.resolvedAnchor,
    }
    local mark_id = state.marks[thread.id]
    if mark_id and mark_pos[mark_id] then
      entry.line = mark_pos[mark_id] + 1
    end
    table.insert(result, entry)
  end
  return result
end

return M
