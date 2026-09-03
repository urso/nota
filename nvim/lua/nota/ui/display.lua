local model = require('nota.model')
local config = require('nota.config')

local M = {}

local ns = model._get_namespace()
local buffer_state = {}
local subscribed = false

local BADGE_OPEN = '●'
local BADGE_RESOLVED = '○'
local BADGE_OUTDATED = '◌'

local SIGN_OPEN = 'NotaSignOpen'
local SIGN_RESOLVED = 'NotaSignResolved'
local SIGN_OUTDATED = 'NotaSignOutdated'

local signs_defined = false

local function define_signs()
  if signs_defined then
    return
  end
  signs_defined = true
  vim.fn.sign_define(SIGN_OPEN, { text = '●', texthl = 'NotaBadge' })
  vim.fn.sign_define(SIGN_RESOLVED, { text = '○', texthl = 'NotaBadgeResolved' })
  vim.fn.sign_define(SIGN_OUTDATED, { text = '◌', texthl = 'NotaBadgeOutdated' })
end

local function define_highlights()
  local existing = vim.api.nvim_get_hl(0, { name = 'NotaBadge' })
  if vim.tbl_isempty(existing) then
    vim.api.nvim_set_hl(0, 'NotaBadge', { fg = '#61afef', bold = true })
  end
  existing = vim.api.nvim_get_hl(0, { name = 'NotaBadgeResolved' })
  if vim.tbl_isempty(existing) then
    vim.api.nvim_set_hl(0, 'NotaBadgeResolved', { fg = '#98c379' })
  end
  existing = vim.api.nvim_get_hl(0, { name = 'NotaBadgeOutdated' })
  if vim.tbl_isempty(existing) then
    vim.api.nvim_set_hl(0, 'NotaBadgeOutdated', { fg = '#5c6370', italic = true })
  end
  existing = vim.api.nvim_get_hl(0, { name = 'NotaInlineHeader' })
  if vim.tbl_isempty(existing) then
    vim.api.nvim_set_hl(0, 'NotaInlineHeader', { fg = '#61afef', bold = true })
  end
  existing = vim.api.nvim_get_hl(0, { name = 'NotaInlineSeparator' })
  if vim.tbl_isempty(existing) then
    vim.api.nvim_set_hl(0, 'NotaInlineSeparator', { fg = '#5c6370' })
  end
  existing = vim.api.nvim_get_hl(0, { name = 'NotaInlineAuthor' })
  if vim.tbl_isempty(existing) then
    vim.api.nvim_set_hl(0, 'NotaInlineAuthor', { fg = '#c678dd' })
  end
  existing = vim.api.nvim_get_hl(0, { name = 'NotaInlineBody' })
  if vim.tbl_isempty(existing) then
    vim.api.nvim_set_hl(0, 'NotaInlineBody', { fg = '#abb2bf' })
  end
end

local function get_buffer_state(bufnr)
  if not buffer_state[bufnr] then
    local cfg = config.get()
    buffer_state[bufnr] = {
      show_statuses = vim.deepcopy(cfg.show_statuses),
      inline_scope = cfg.inline_scope,
      last_cursor_line = nil,
    }
  end
  return buffer_state[bufnr]
end

local function should_show_thread(thread, show_statuses)
  if not thread.status then
    return true
  end
  for _, status in ipairs(show_statuses) do
    if thread.status == status then
      return true
    end
  end
  return false
end

local function group_threads_by_line(threads)
  local by_line = {}
  for _, thread in ipairs(threads) do
    if thread.anchor and thread.anchor.line then
      local line = thread.anchor.line
      if not by_line[line] then
        by_line[line] = {}
      end
      table.insert(by_line[line], thread)
    end
  end
  return by_line
end

local function format_badge(threads)
  local open_count = 0
  local resolved_count = 0
  local has_outdated = false

  for _, thread in ipairs(threads) do
    if thread.anchor and thread.anchor.outdated then
      has_outdated = true
    end
    if thread.status == 'resolved' then
      resolved_count = resolved_count + 1
    else
      open_count = open_count + 1
    end
  end

  local parts = {}

  if open_count > 0 then
    local badge = has_outdated and BADGE_OUTDATED or BADGE_OPEN
    local hl = has_outdated and 'NotaBadgeOutdated' or 'NotaBadge'
    if open_count > 1 then
      table.insert(parts, { badge .. open_count, hl })
    else
      table.insert(parts, { badge, hl })
    end
  end

  if resolved_count > 0 then
    if resolved_count > 1 then
      table.insert(parts, { BADGE_RESOLVED .. resolved_count, 'NotaBadgeResolved' })
    else
      table.insert(parts, { BADGE_RESOLVED, 'NotaBadgeResolved' })
    end
  end

  if #parts == 0 then
    return nil
  end

  local result = { { ' ', 'Normal' } }
  for i, part in ipairs(parts) do
    if i > 1 then
      table.insert(result, { ' ', 'Normal' })
    end
    table.insert(result, part)
  end
  return result
end

local function relative_time(timestamp)
  if not timestamp then
    return ''
  end
  local now = os.time()
  local ts = timestamp
  if type(timestamp) == 'string' then
    local y, m, d, h, min, s = timestamp:match('(%d+)-(%d+)-(%d+)T(%d+):(%d+):(%d+)')
    if y then
      ts = os.time({ year = y, month = m, day = d, hour = h, min = min, sec = s })
    else
      return ''
    end
  end
  local diff = now - ts
  if diff < 60 then
    return 'now'
  elseif diff < 3600 then
    return math.floor(diff / 60) .. 'm ago'
  elseif diff < 86400 then
    return math.floor(diff / 3600) .. 'h ago'
  else
    return math.floor(diff / 86400) .. 'd ago'
  end
end

local function format_inline_conversation(thread)
  local lines = {}
  local prefix = '│ '

  local header = '[' .. (thread.status or 'open') .. '] ' .. (thread.goal or thread.title or 'thread')
  table.insert(lines, { { prefix, 'NotaInlineSeparator' }, { header, 'NotaInlineHeader' } })

  local sep = string.rep('─', 24)
  table.insert(lines, { { prefix, 'NotaInlineSeparator' }, { sep, 'NotaInlineSeparator' } })

  local comments = thread.comments or {}
  for i, comment in ipairs(comments) do
    if i > 1 then
      table.insert(lines, { { prefix, 'NotaInlineSeparator' } })
    end
    local author = comment.author or 'unknown'
    local bodies = comment.bodies or {}
    local first_body = bodies[1]
    local time = first_body and relative_time(first_body.time) or ''
    local meta = author .. (time ~= '' and (' · ' .. time) or '')
    table.insert(lines, { { prefix, 'NotaInlineSeparator' }, { meta, 'NotaInlineAuthor' } })

    local body = first_body and first_body.content or ''
    for line in body:gmatch('[^\n]+') do
      table.insert(lines, { { prefix .. '  ', 'NotaInlineSeparator' }, { line, 'NotaInlineBody' } })
    end
  end

  return lines
end

local function get_mark_for_line(bufnr, line)
  local marks = vim.api.nvim_buf_get_extmarks(bufnr, ns, { line - 1, 0 }, { line - 1, -1 }, {})
  if #marks > 0 then
    return marks[1][1]
  end
  return nil
end

local function get_sign_for_threads(line_threads)
  local has_outdated = false
  local has_resolved = false
  for _, thread in ipairs(line_threads) do
    if thread.anchor and thread.anchor.outdated then
      has_outdated = true
    end
    if thread.status == 'resolved' then
      has_resolved = true
    end
  end
  if has_outdated then
    return SIGN_OUTDATED
  end
  if has_resolved then
    return SIGN_RESOLVED
  end
  return SIGN_OPEN
end

local function render_buffer(bufnr, threads)
  if not vim.api.nvim_buf_is_valid(bufnr) then
    return
  end

  local cfg = config.get()
  local use_eol = cfg.eol_badge ~= false
  local use_sign = cfg.sign_column == true

  if use_sign then
    define_signs()
    vim.fn.sign_unplace('nota', { buffer = bufnr })
  end

  local state = get_buffer_state(bufnr)
  local filtered = {}
  for _, thread in ipairs(threads) do
    if should_show_thread(thread, state.show_statuses) then
      table.insert(filtered, thread)
    end
  end

  local by_line = group_threads_by_line(filtered)
  local cursor_line = nil
  if state.inline_scope == 'cursor' then
    cursor_line = vim.api.nvim_win_get_cursor(0)[1]
  end

  for line, line_threads in pairs(by_line) do
    local mark_id = get_mark_for_line(bufnr, line)
    if mark_id then
      local badge = use_eol and format_badge(line_threads) or nil
      local virt_lines = nil

      if state.inline_scope == 'buffer' or (state.inline_scope == 'cursor' and line == cursor_line) then
        virt_lines = {}
        for i, thread in ipairs(line_threads) do
          if i > 1 then
            local thick_sep = string.rep('═', 24)
            table.insert(virt_lines, { { '│ ', 'NotaInlineSeparator' }, { thick_sep, 'NotaInlineSeparator' } })
          end
          local conv_lines = format_inline_conversation(thread)
          for _, conv_line in ipairs(conv_lines) do
            table.insert(virt_lines, conv_line)
          end
        end
      end

      vim.api.nvim_buf_set_extmark(bufnr, ns, line - 1, 0, {
        id = mark_id,
        virt_text = badge,
        virt_text_pos = 'eol',
        virt_lines = virt_lines,
      })

      if use_sign then
        local sign_name = get_sign_for_threads(line_threads)
        vim.fn.sign_place(0, 'nota', sign_name, bufnr, { lnum = line, priority = 10 })
      end
    end
  end
end

local function on_model_update(bufnr, threads)
  render_buffer(bufnr, threads)
end

function M.set_inline_scope(scope, bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  if scope ~= 'off' and scope ~= 'cursor' and scope ~= 'buffer' then
    vim.notify('nota: invalid inline scope: ' .. tostring(scope), vim.log.levels.WARN)
    return
  end
  local state = get_buffer_state(bufnr)
  state.inline_scope = scope
  local threads = model.get_threads(bufnr)
  render_buffer(bufnr, threads)
end

function M.get_inline_scope(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  local state = get_buffer_state(bufnr)
  return state.inline_scope
end

function M.toggle_resolved(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  local state = get_buffer_state(bufnr)
  local has_resolved = false
  for i, status in ipairs(state.show_statuses) do
    if status == 'resolved' then
      has_resolved = true
      table.remove(state.show_statuses, i)
      break
    end
  end
  if not has_resolved then
    table.insert(state.show_statuses, 'resolved')
  end
  local threads = model.get_threads(bufnr)
  render_buffer(bufnr, threads)
end

function M.set_show_statuses(statuses, bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  local state = get_buffer_state(bufnr)
  state.show_statuses = vim.deepcopy(statuses)
  local threads = model.get_threads(bufnr)
  render_buffer(bufnr, threads)
end

function M.get_show_statuses(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  local state = get_buffer_state(bufnr)
  return state.show_statuses
end

function M.on_cursor_moved(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  local state = buffer_state[bufnr]
  if not state or state.inline_scope ~= 'cursor' then
    return
  end
  local cursor_line = vim.api.nvim_win_get_cursor(0)[1]
  if state.last_cursor_line == cursor_line then
    return
  end
  state.last_cursor_line = cursor_line
  local threads = model.get_threads(bufnr)
  render_buffer(bufnr, threads)
end

function M.cleanup(bufnr)
  buffer_state[bufnr] = nil
  vim.fn.sign_unplace('nota', { buffer = bufnr })
end

function M._reset()
  buffer_state = {}
end

function M._init()
  if subscribed then
    return
  end
  subscribed = true
  define_highlights()
  model.on_update(on_model_update)
end

M._init()

function M.relative_time(timestamp)
  return relative_time(timestamp)
end

function M.format_thread_comments(thread)
  local lines = {}
  local comments = thread.comments or {}
  for i, comment in ipairs(comments) do
    if i > 1 then
      table.insert(lines, '')
    end
    local author = comment.author or 'unknown'
    local bodies = comment.bodies or {}
    local first_body = bodies[1]
    local time = first_body and relative_time(first_body.time) or ''
    local meta = '@' .. author .. (time ~= '' and (' (' .. time .. ')') or '') .. ':'
    table.insert(lines, meta)
    local body = first_body and first_body.content or ''
    for line in body:gmatch('[^\n]+') do
      table.insert(lines, '  ' .. line)
    end
  end
  return lines
end

return M
