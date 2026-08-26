-- Model layer for per-buffer thread state.
-- Holds threads anchored in attached buffers, manages extmarks, emits change events.

local client = require('nota.client')
local repo_utils = require('nota.repo')

local M = {}

local buffers = {}
local subscribers = {}
local ns = vim.api.nvim_create_namespace('nota')

function M._reset()
  for bufnr, _ in pairs(buffers) do
    if vim.api.nvim_buf_is_valid(bufnr) then
      vim.api.nvim_buf_clear_namespace(bufnr, ns, 0, -1)
    end
  end
  buffers = {}
  subscribers = {}
end

function M._get_namespace()
  return ns
end

local function emit_update(bufnr, threads)
  local snapshot = {}
  for i, cb in ipairs(subscribers) do
    snapshot[i] = cb
  end
  for _, cb in ipairs(snapshot) do
    pcall(cb, bufnr, threads)
  end
end

local function place_marks(bufnr, state)
  if not vim.api.nvim_buf_is_valid(bufnr) then
    return
  end
  local line_count = vim.api.nvim_buf_line_count(bufnr)
  for _, thread in ipairs(state.threads) do
    if thread.resolvedAnchor and thread.resolvedAnchor.line and thread.resolvedAnchor.line > 0 then
      local line = math.max(1, math.min(thread.resolvedAnchor.line, math.max(1, line_count)))
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

  local state = {
    repo = repo,
    file = rel_path,
    threads = {},
    marks = {},
  }
  buffers[bufnr] = state

  local co = coroutine.create(function()
    local response = client.list(repo, { file = rel_path })
    if response.err then
      vim.notify('nota: failed to list threads: ' .. (response.err.message or 'unknown error'), vim.log.levels.WARN)
      return
    end
    if buffers[bufnr] == state then
      state.threads = response.result or {}
      place_marks(bufnr, state)
      emit_update(bufnr, M.get_threads(bufnr))
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

function M.refresh(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  local state = buffers[bufnr]
  if not state then
    return false
  end
  if not vim.api.nvim_buf_is_valid(bufnr) then
    return false
  end

  vim.api.nvim_buf_clear_namespace(bufnr, ns, 0, -1)
  state.marks = {}
  state.threads = {}

  local co = coroutine.create(function()
    local response = client.list(state.repo, { file = state.file })
    if response.err then
      vim.notify('nota: refresh failed: ' .. (response.err.message or 'unknown error'), vim.log.levels.WARN)
      return
    end
    if buffers[bufnr] == state then
      state.threads = response.result or {}
      place_marks(bufnr, state)
      emit_update(bufnr, M.get_threads(bufnr))
    end
  end)

  vim.schedule(function()
    coroutine.resume(co)
  end)

  return true
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
  local mark_rows = {}
  for _, mark in ipairs(all_marks) do
    mark_rows[mark[1]] = mark[2]
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
    if mark_id and mark_rows[mark_id] then
      entry.line = mark_rows[mark_id] + 1
    end
    table.insert(result, entry)
  end
  return result
end

function M.on_update(callback)
  if type(callback) ~= 'function' then
    return false
  end
  for _, cb in ipairs(subscribers) do
    if cb == callback then
      return false
    end
  end
  table.insert(subscribers, callback)
  return true
end

function M.off_update(callback)
  for i, cb in ipairs(subscribers) do
    if cb == callback then
      table.remove(subscribers, i)
      return true
    end
  end
  return false
end

client.on_change(function(params)
  local changed_repo = params.repo
  local changed_files = params.files or {}
  local changed_set = {}
  for _, file in ipairs(changed_files) do
    changed_set[file] = true
  end
  for bufnr, state in pairs(buffers) do
    if state.repo == changed_repo and changed_set[state.file] then
      M.refresh(bufnr)
    end
  end
end)

return M
