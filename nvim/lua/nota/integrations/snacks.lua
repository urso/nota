local ok, snacks = pcall(require, 'snacks')
if not ok or not snacks.picker then
  return {
    threads = function()
      vim.notify('snacks.nvim with picker not installed', vim.log.levels.ERROR)
    end,
    threads_all = function()
      vim.notify('snacks.nvim with picker not installed', vim.log.levels.ERROR)
    end,
  }
end

local nota = require('nota')
local display = require('nota.ui.display')

local M = {}

local function format_thread(thread)
  local status = thread.status or 'open'
  local line = thread.anchor and thread.anchor.line or '?'
  local title = thread.title or '(no title)'
  local goal = thread.goal and (' [' .. thread.goal .. ']') or ''
  return string.format('L%-4d %s (%s)%s', line, title, status, goal)
end

local function format_thread_with_file(thread)
  local file = thread.resolvedAnchor and thread.resolvedAnchor.file or '?'
  local line = thread.anchor and thread.anchor.line or (thread.resolvedAnchor and thread.resolvedAnchor.line) or '?'
  local title = thread.title or '(no title)'
  local status = thread.status or 'open'
  return string.format('%s:%s %s (%s)', file, line, title, status)
end

local function build_preview_lines(thread, file_path, anchor_line)
  local lines = {}

  if file_path and anchor_line and anchor_line > 0 then
    local file_lines = vim.fn.readfile(file_path)
    if #file_lines > 0 then
      local start_line = math.max(1, anchor_line - 3)
      local end_line = math.min(#file_lines, anchor_line + 3)

      table.insert(lines, string.format('── %s:%d ──', vim.fn.fnamemodify(file_path, ':t'), anchor_line))
      for i = start_line, end_line do
        local prefix = (i == anchor_line) and '> ' or '  '
        table.insert(lines, prefix .. (file_lines[i] or ''))
      end
      table.insert(lines, '')
    end
  end

  table.insert(lines, '── Thread ──')
  table.insert(lines, string.format('Status: %s', thread.status or 'open'))
  if thread.goal then
    table.insert(lines, string.format('Goal: %s', thread.goal))
  end
  table.insert(lines, '')

  local comment_lines = display.format_thread_comments(thread)
  for _, line in ipairs(comment_lines) do
    table.insert(lines, line)
  end

  return lines
end

local function create_previewer(get_file_path)
  return function(ctx)
    local item = ctx.item
    if not item or not item.thread then
      return false
    end

    local thread = item.thread
    local file_path = get_file_path(item)
    local anchor_line = thread.anchor and thread.anchor.line
      or (thread.resolvedAnchor and thread.resolvedAnchor.line)

    local lines = build_preview_lines(thread, file_path, anchor_line)
    ctx.preview:set_lines(lines)
    ctx.preview:highlight({ ft = 'markdown' })
    return true
  end
end

function M.threads(opts)
  opts = opts or {}
  local bufnr = vim.api.nvim_get_current_buf()
  local file_path = vim.api.nvim_buf_get_name(bufnr)
  local threads = nota.get_threads(bufnr)

  if #threads == 0 then
    vim.notify('No threads in current buffer', vim.log.levels.INFO)
    return
  end

  local items = {}
  for _, thread in ipairs(threads) do
    table.insert(items, {
      text = format_thread(thread),
      file = file_path,
      pos = thread.anchor and { thread.anchor.line, 0 } or nil,
      thread = thread,
    })
  end

  snacks.picker.pick(vim.tbl_extend('force', {
    title = 'Nota Threads',
    items = items,
    preview = create_previewer(function(_) return file_path end),
    confirm = function(picker, item)
      picker:close()
      if item and item.pos then
        vim.api.nvim_win_set_cursor(0, { item.pos[1], 0 })
      end
    end,
  }, opts))
end

function M.threads_all(opts)
  opts = opts or {}

  nota.ensure_loaded(nil, function(loaded)
    if not loaded then
      vim.notify('Failed to load nota threads', vim.log.levels.ERROR)
      return
    end

    vim.schedule(function()
      local threads = nota.get_all_threads()

      if #threads == 0 then
        vim.notify('No threads in repository', vim.log.levels.INFO)
        return
      end

      local items = {}
      local cwd = vim.fn.getcwd()
      for _, thread in ipairs(threads) do
        local file = thread.resolvedAnchor and thread.resolvedAnchor.file
        local abs_file = file and (cwd .. '/' .. file) or nil
        local line = thread.anchor and thread.anchor.line
          or (thread.resolvedAnchor and thread.resolvedAnchor.line)

        table.insert(items, {
          text = format_thread_with_file(thread),
          file = abs_file,
          pos = line and { line, 0 } or nil,
          thread = thread,
        })
      end

      snacks.picker.pick(vim.tbl_extend('force', {
        title = 'Nota Threads (All)',
        items = items,
        preview = create_previewer(function(item) return item.file end),
        confirm = function(picker, item)
          picker:close()
          if item and item.file then
            vim.cmd('edit ' .. vim.fn.fnameescape(item.file))
            if item.pos then
              vim.api.nvim_win_set_cursor(0, { item.pos[1], 0 })
            end
          end
        end,
      }, opts))
    end)
  end)
end

M.threads_source = {
  title = 'Nota Threads',
  finder = function(_, _)
    local bufnr = vim.api.nvim_get_current_buf()
    local file_path = vim.api.nvim_buf_get_name(bufnr)
    local threads = nota.get_threads(bufnr)
    local items = {}
    for _, thread in ipairs(threads) do
      table.insert(items, {
        text = format_thread(thread),
        file = file_path,
        pos = thread.anchor and { thread.anchor.line, 0 } or nil,
        thread = thread,
      })
    end
    return items
  end,
  preview = create_previewer(function(_)
    return vim.api.nvim_buf_get_name(vim.api.nvim_get_current_buf())
  end),
  confirm = function(picker, item)
    picker:close()
    if item and item.pos then
      vim.api.nvim_win_set_cursor(0, { item.pos[1], 0 })
    end
  end,
}

M.threads_all_source = {
  title = 'Nota Threads (All)',
  finder = function(_, _)
    return function(cb)
      nota.ensure_loaded(nil, function(loaded)
        if not loaded then
          cb({})
          return
        end
        vim.schedule(function()
          local threads = nota.get_all_threads()
          local items = {}
          local cwd = vim.fn.getcwd()
          for _, thread in ipairs(threads) do
            local file = thread.resolvedAnchor and thread.resolvedAnchor.file
            local abs_file = file and (cwd .. '/' .. file) or nil
            local line = thread.anchor and thread.anchor.line
              or (thread.resolvedAnchor and thread.resolvedAnchor.line)
            table.insert(items, {
              text = format_thread_with_file(thread),
              file = abs_file,
              pos = line and { line, 0 } or nil,
              thread = thread,
            })
          end
          cb(items)
        end)
      end)
    end
  end,
  preview = create_previewer(function(item) return item.file end),
  confirm = function(picker, item)
    picker:close()
    if item and item.file then
      vim.cmd('edit ' .. vim.fn.fnameescape(item.file))
      if item.pos then
        vim.api.nvim_win_set_cursor(0, { item.pos[1], 0 })
      end
    end
  end,
}

return M
