local model = require('nota.model')
local client = require('nota.client')

local M = {}

local drafts = {}
local VALID_STATUSES = { 'open', 'resolved' }

local function define_highlights()
  local existing = vim.api.nvim_get_hl(0, { name = 'NotaFloatTitle' })
  if vim.tbl_isempty(existing) then
    vim.api.nvim_set_hl(0, 'NotaFloatTitle', { fg = '#61afef', bold = true })
  end
  existing = vim.api.nvim_get_hl(0, { name = 'NotaFloatBorder' })
  if vim.tbl_isempty(existing) then
    vim.api.nvim_set_hl(0, 'NotaFloatBorder', { fg = '#5c6370' })
  end
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

local function create_float_win(buf, opts)
  opts = opts or {}
  local width = opts.width or 60
  local height = opts.height or 10
  local title = opts.title or ''

  local ui = vim.api.nvim_list_uis()[1] or { width = 80, height = 24 }
  local row = math.floor((ui.height - height) / 2)
  local col = math.floor((ui.width - width) / 2)

  local win = vim.api.nvim_open_win(buf, true, {
    relative = 'editor',
    width = width,
    height = height,
    row = row,
    col = col,
    style = 'minimal',
    border = 'rounded',
    title = title ~= '' and ' ' .. title .. ' ' or nil,
    title_pos = 'center',
  })

  vim.api.nvim_set_option_value('winhl', 'FloatBorder:NotaFloatBorder,FloatTitle:NotaFloatTitle', { win = win })

  return win
end

local function close_float(win, buf)
  if win and vim.api.nvim_win_is_valid(win) then
    vim.api.nvim_win_close(win, true)
  end
  if buf and vim.api.nvim_buf_is_valid(buf) then
    vim.api.nvim_buf_delete(buf, { force = true })
  end
end

local function set_close_keymaps(buf, win, on_close)
  local close_fn = function()
    if on_close then
      on_close()
    end
    close_float(win, buf)
  end
  vim.keymap.set('n', 'q', close_fn, { buffer = buf, nowait = true })
  vim.keymap.set('n', '<Esc>', close_fn, { buffer = buf, nowait = true })
end

function M.threads_at_cursor(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  local cursor_line = vim.api.nvim_win_get_cursor(0)[1]
  local threads = model.get_threads(bufnr)
  local result = {}
  for _, thread in ipairs(threads) do
    if thread.anchor and thread.anchor.line == cursor_line then
      table.insert(result, thread)
    end
  end
  return result
end

local function choose_thread(threads, callback)
  if #threads == 0 then
    vim.notify('nota: no threads at cursor', vim.log.levels.INFO)
    return
  end
  if #threads == 1 then
    callback(threads[1])
    return
  end
  local items = {}
  for _, thread in ipairs(threads) do
    local label = thread.goal or thread.title or thread.id
    table.insert(items, { thread = thread, label = '[' .. (thread.status or 'open') .. '] ' .. label })
  end
  vim.ui.select(items, {
    prompt = 'Select thread:',
    format_item = function(item)
      return item.label
    end,
  }, function(choice)
    if choice then
      callback(choice.thread)
    end
  end)
end

local function format_conversation(thread)
  local lines = {}
  local status = thread.status or 'open'
  local goal = thread.goal or thread.title or 'thread'
  table.insert(lines, '[' .. status .. '] ' .. goal)
  if thread.anchor then
    local anchor_info = 'Line ' .. thread.anchor.line
    if thread.anchor.outdated then
      anchor_info = anchor_info .. ' (outdated)'
    end
    table.insert(lines, anchor_info)
  end
  table.insert(lines, string.rep('─', 40))

  local comments = thread.comments or {}
  for i, comment in ipairs(comments) do
    if i > 1 then
      table.insert(lines, '')
    end
    local author = comment.author or 'unknown'
    local bodies = comment.bodies or {}
    local first_body = bodies[1]
    local time = first_body and relative_time(first_body.time) or ''
    local meta = author .. (time ~= '' and (' · ' .. time) or '')
    table.insert(lines, meta)
    local body = first_body and first_body.content or ''
    for line in body:gmatch('[^\n]+') do
      table.insert(lines, '  ' .. line)
    end
  end

  return lines
end

function M.open_conversation(thread)
  if not thread then
    local threads = M.threads_at_cursor()
    choose_thread(threads, function(t)
      M.open_conversation(t)
    end)
    return
  end

  define_highlights()

  local lines = format_conversation(thread)
  local max_width = 40
  for _, line in ipairs(lines) do
    max_width = math.max(max_width, #line)
  end
  local width = math.min(max_width + 4, 80)
  local height = math.min(#lines, 20)

  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.api.nvim_set_option_value('modifiable', false, { buf = buf })
  vim.api.nvim_set_option_value('bufhidden', 'wipe', { buf = buf })

  local win = create_float_win(buf, { width = width, height = height, title = 'Conversation' })
  set_close_keymaps(buf, win)
end

local function draft_key(target)
  if target.type == 'anchor' then
    return 'anchor:' .. target.file .. ':' .. target.line
  elseif target.type == 'reply' then
    return 'reply:' .. target.thread_id
  end
  return nil
end

local function save_draft(target, content)
  local key = draft_key(target)
  if key then
    if content and content ~= '' then
      drafts[key] = content
    else
      drafts[key] = nil
    end
  end
end

local function get_draft(target)
  local key = draft_key(target)
  return key and drafts[key] or nil
end

local function clear_draft(target)
  local key = draft_key(target)
  if key then
    drafts[key] = nil
  end
end

function M.open_compose(target, opts)
  opts = opts or {}

  if not target then
    local bufnr = vim.api.nvim_get_current_buf()
    local buf_state = model._get_state_internal(bufnr)
    if not buf_state then
      vim.notify('nota: buffer not attached to a nota repo', vim.log.levels.WARN)
      return
    end
    local cursor_line = vim.api.nvim_win_get_cursor(0)[1]
    target = {
      type = 'anchor',
      file = buf_state.file,
      line = cursor_line,
      repo = buf_state.repo,
    }
  end

  define_highlights()

  local title = target.type == 'reply' and 'Reply' or 'New Thread'
  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_set_option_value('bufhidden', 'wipe', { buf = buf })
  vim.api.nvim_set_option_value('buftype', 'nofile', { buf = buf })

  local draft = get_draft(target)
  if draft then
    local draft_lines = vim.split(draft, '\n', { plain = true })
    vim.api.nvim_buf_set_lines(buf, 0, -1, false, draft_lines)
  end

  local win = create_float_win(buf, { width = 60, height = 10, title = title })

  local compose_opts = vim.tbl_extend('force', {}, opts)
  compose_opts.target = target

  local function on_close()
    local lines = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
    local content = table.concat(lines, '\n')
    save_draft(target, content)
  end

  local function submit()
    local lines = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
    local body = table.concat(lines, '\n')
    if body == '' or body:match('^%s*$') then
      vim.notify('nota: empty message', vim.log.levels.WARN)
      return
    end

    vim.schedule(function()
      coroutine.wrap(function()
        local response
        if target.type == 'reply' then
          response = client.add_comment(target.repo, target.thread_id, body)
        else
          local params = {
            anchor = { file = target.file, line = target.line },
            body = body,
          }
          if compose_opts.goal then
            params.goal = compose_opts.goal
          end
          if compose_opts.group then
            params.group = compose_opts.group
          end
          if compose_opts.tags then
            params.tags = compose_opts.tags
          end
          response = client.create(target.repo, params)
        end

        if response.err then
          vim.notify('nota: ' .. (response.err.message or 'failed to submit'), vim.log.levels.ERROR)
          return
        end

        clear_draft(target)
        close_float(win, buf)

        local source_buf = vim.fn.bufnr(target.file)
        if source_buf ~= -1 and model.is_attached(source_buf) then
          model.refresh(source_buf)
        end
      end)()
    end)
  end

  set_close_keymaps(buf, win, on_close)
  vim.keymap.set('n', '<C-s>', submit, { buffer = buf, nowait = true })
  vim.keymap.set('i', '<C-s>', function()
    vim.cmd('stopinsert')
    submit()
  end, { buffer = buf, nowait = true })
end

function M.open_reply(thread)
  if not thread then
    local threads = M.threads_at_cursor()
    choose_thread(threads, function(t)
      M.open_reply(t)
    end)
    return
  end

  local bufnr = vim.api.nvim_get_current_buf()
  local buf_state = model._get_state_internal(bufnr)
  if not buf_state then
    vim.notify('nota: buffer not attached to a nota repo', vim.log.levels.WARN)
    return
  end

  local target = {
    type = 'reply',
    thread_id = thread.id,
    repo = buf_state.repo,
  }
  M.open_compose(target)
end

function M.open_new(opts)
  opts = opts or {}
  local bufnr = vim.api.nvim_get_current_buf()
  local buf_state = model._get_state_internal(bufnr)
  if not buf_state then
    vim.notify('nota: buffer not attached to a nota repo', vim.log.levels.WARN)
    return
  end

  local line
  local mode = vim.fn.mode()
  if mode == 'v' or mode == 'V' or mode == '\22' then
    line = vim.fn.getpos("'<")[2]
    vim.cmd('normal! ' .. vim.api.nvim_replace_termcodes('<Esc>', true, false, true))
  else
    line = vim.api.nvim_win_get_cursor(0)[1]
  end

  local target = {
    type = 'anchor',
    file = buf_state.file,
    line = line,
    repo = buf_state.repo,
  }
  M.open_compose(target, opts)
end

function M.change_status(thread)
  if not thread then
    local threads = M.threads_at_cursor()
    choose_thread(threads, function(t)
      M.change_status(t)
    end)
    return
  end

  local bufnr = vim.api.nvim_get_current_buf()
  local buf_state = model._get_state_internal(bufnr)
  if not buf_state then
    vim.notify('nota: buffer not attached to a nota repo', vim.log.levels.WARN)
    return
  end

  vim.ui.select(VALID_STATUSES, {
    prompt = 'Set status:',
  }, function(status)
    if not status then
      return
    end
    vim.schedule(function()
      coroutine.wrap(function()
        local response = client.set_status(buf_state.repo, thread.id, status)
        if response.err then
          vim.notify('nota: ' .. (response.err.message or 'failed to set status'), vim.log.levels.ERROR)
          return
        end
        model.refresh(bufnr)
      end)()
    end)
  end)
end

function M._reset()
  drafts = {}
end

function M._get_drafts()
  return drafts
end

return M
