-- Model layer for thread state.
-- Maintains a repo-level thread cache and per-buffer extmark tracking.

local client = require('nota.client')
local repo_utils = require('nota.repo')

local M = {}

local repos = {}
local buffers = {}
local subscribers = {}
local ns = vim.api.nvim_create_namespace('nota')

function M._reset()
  for bufnr, _ in pairs(buffers) do
    if vim.api.nvim_buf_is_valid(bufnr) then
      vim.api.nvim_buf_clear_namespace(bufnr, ns, 0, -1)
    end
  end
  repos = {}
  buffers = {}
  subscribers = {}
end

function M._get_namespace()
  return ns
end

local function get_or_create_repo_state(repo)
  if not repos[repo] then
    repos[repo] = {
      threads = {},
      threads_by_id = {},
      threads_by_file = {},
      generation = 0,
      loaded = false,
    }
  end
  return repos[repo]
end

local function index_threads(repo_state)
  repo_state.threads_by_id = {}
  repo_state.threads_by_file = {}
  for _, thread in ipairs(repo_state.threads) do
    repo_state.threads_by_id[thread.id] = thread
    if thread.resolvedAnchor and thread.resolvedAnchor.file then
      local file = thread.resolvedAnchor.file
      if not repo_state.threads_by_file[file] then
        repo_state.threads_by_file[file] = {}
      end
      table.insert(repo_state.threads_by_file[file], thread)
    end
  end
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

local function place_marks(bufnr, buf_state, threads)
  if not vim.api.nvim_buf_is_valid(bufnr) then
    return
  end
  local line_count = vim.api.nvim_buf_line_count(bufnr)
  for _, thread in ipairs(threads) do
    if thread.resolvedAnchor and thread.resolvedAnchor.line and thread.resolvedAnchor.line > 0 then
      local line = math.max(1, math.min(thread.resolvedAnchor.line, math.max(1, line_count)))
      local mark_id = vim.api.nvim_buf_set_extmark(bufnr, ns, line - 1, 0, {})
      buf_state.marks[thread.id] = mark_id
    end
  end
end

local function refresh_repo(repo, callback)
  local repo_state = get_or_create_repo_state(repo)
  repo_state.generation = repo_state.generation + 1
  local gen = repo_state.generation

  vim.schedule(function()
    coroutine.wrap(function()
      local response = client.list(repo)
      if response.err then
        vim.notify('nota: failed to list threads: ' .. (response.err.message or 'unknown error'), vim.log.levels.WARN)
        if callback then
          callback(false)
        end
        return
      end
      if repos[repo] == repo_state and repo_state.generation == gen then
        repo_state.threads = response.result or {}
        repo_state.loaded = true
        index_threads(repo_state)
        if callback then
          callback(true)
        end
      end
    end)()
  end)
end

local function update_buffer_marks(bufnr, buf_state)
  if not vim.api.nvim_buf_is_valid(bufnr) then
    return
  end
  local repo_state = repos[buf_state.repo]
  if not repo_state then
    return
  end
  local threads = repo_state.threads_by_file[buf_state.file] or {}
  vim.api.nvim_buf_clear_namespace(bufnr, ns, 0, -1)
  buf_state.marks = {}
  place_marks(bufnr, buf_state, threads)
  emit_update(bufnr, M.get_threads(bufnr))
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

  local buf_state = {
    repo = repo,
    file = rel_path,
    marks = {},
  }
  buffers[bufnr] = buf_state

  local repo_state = get_or_create_repo_state(repo)
  if repo_state.loaded then
    update_buffer_marks(bufnr, buf_state)
  else
    refresh_repo(repo, function(ok)
      if ok and buffers[bufnr] == buf_state then
        update_buffer_marks(bufnr, buf_state)
      end
    end)
  end

  return true
end

function M.detach(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()

  local buf_state = buffers[bufnr]
  if not buf_state then
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
  local buf_state = buffers[bufnr]
  if not buf_state then
    return nil
  end
  local repo_state = repos[buf_state.repo]
  local threads = repo_state and repo_state.threads_by_file[buf_state.file] or {}
  return {
    repo = buf_state.repo,
    file = buf_state.file,
    threads = threads,
    marks = buf_state.marks,
  }
end

function M._get_state_internal(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  return buffers[bufnr]
end

function M._get_repo_state(repo)
  return repos[repo]
end

function M.refresh(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  local buf_state = buffers[bufnr]
  if not buf_state then
    return false
  end
  if not vim.api.nvim_buf_is_valid(bufnr) then
    return false
  end

  refresh_repo(buf_state.repo, function(ok)
    if ok and buffers[bufnr] == buf_state then
      update_buffer_marks(bufnr, buf_state)
    end
  end)

  return true
end

local function apply_extmark_positions(bufnr, buf_state, threads)
  if not vim.api.nvim_buf_is_valid(bufnr) then
    return {}
  end
  local all_marks = vim.api.nvim_buf_get_extmarks(bufnr, ns, 0, -1, {})
  local mark_rows = {}
  for _, mark in ipairs(all_marks) do
    mark_rows[mark[1]] = mark[2]
  end
  local result = {}
  for i, thread in ipairs(threads) do
    local copy = vim.tbl_extend('force', {}, thread)
    local mark_id = buf_state.marks[thread.id]
    if mark_id and mark_rows[mark_id] then
      local outdated = thread.resolvedAnchor and thread.resolvedAnchor.outdated or false
      copy.anchor = { line = mark_rows[mark_id] + 1, outdated = outdated }
    end
    result[i] = copy
  end
  return result
end

function M.get_threads(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  local buf_state = buffers[bufnr]
  if not buf_state then
    return {}
  end

  if not vim.api.nvim_buf_is_valid(bufnr) then
    return {}
  end

  local repo_state = repos[buf_state.repo]
  if not repo_state then
    return {}
  end

  local threads = repo_state.threads_by_file[buf_state.file] or {}
  return apply_extmark_positions(bufnr, buf_state, threads)
end

function M.get_all_threads(repo)
  if not repo then
    local bufnr = vim.api.nvim_get_current_buf()
    local buf_state = buffers[bufnr]
    if buf_state then
      repo = buf_state.repo
    else
      repo = repo_utils.get_root(vim.fn.getcwd())
    end
  end

  if not repo then
    return {}
  end

  local repo_state = repos[repo]
  if not repo_state or not repo_state.loaded then
    return {}
  end

  local file_threads = {}
  for bufnr, buf_state in pairs(buffers) do
    if buf_state.repo == repo then
      local threads = repo_state.threads_by_file[buf_state.file] or {}
      local with_anchors = apply_extmark_positions(bufnr, buf_state, threads)
      for _, t in ipairs(with_anchors) do
        file_threads[t.id] = t
      end
    end
  end

  local result = {}
  for i, thread in ipairs(repo_state.threads) do
    if file_threads[thread.id] then
      result[i] = file_threads[thread.id]
    else
      local copy = vim.tbl_extend('force', {}, thread)
      if thread.resolvedAnchor then
        local outdated = thread.resolvedAnchor.outdated or false
        local line = thread.resolvedAnchor.line
        if line and line > 0 then
          copy.anchor = { line = line, outdated = outdated }
        end
      end
      result[i] = copy
    end
  end

  return result
end

function M.ensure_loaded(repo, callback)
  if not repo then
    local bufnr = vim.api.nvim_get_current_buf()
    local buf_state = buffers[bufnr]
    if buf_state then
      repo = buf_state.repo
    else
      repo = repo_utils.get_root(vim.fn.getcwd())
    end
  end

  if not repo then
    if callback then
      callback(false)
    end
    return false
  end

  local repo_state = repos[repo]
  if repo_state and repo_state.loaded then
    if callback then
      callback(true)
    end
    return true
  end

  refresh_repo(repo, callback)
  return false
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

local change_handler_registered = false

local function on_change_handler(params)
  local changed_repo = params.repo
  if not changed_repo then
    return
  end

  refresh_repo(changed_repo, function(ok)
    if not ok then
      return
    end
    local changed_files = params.files or {}
    local changed_set = {}
    for _, file in ipairs(changed_files) do
      changed_set[file] = true
    end
    local bufs_to_update = {}
    for bufnr, buf_state in pairs(buffers) do
      if buf_state.repo == changed_repo and changed_set[buf_state.file] then
        table.insert(bufs_to_update, { bufnr, buf_state })
      end
    end
    for _, entry in ipairs(bufs_to_update) do
      if buffers[entry[1]] == entry[2] then
        update_buffer_marks(entry[1], entry[2])
      end
    end
  end)
end

function M._init()
  if change_handler_registered then
    return
  end
  change_handler_registered = true
  client.on_change(on_change_handler)
end

M._init()

return M
