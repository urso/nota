if vim.g.loaded_nota then
  return
end
vim.g.loaded_nota = true

require('nota')

vim.api.nvim_create_user_command('NotaReset', function()
  require('nota.transport').reset()
end, { desc = 'Reset nota daemon connection' })

vim.api.nvim_create_user_command('NotaRestart', function()
  require('nota').restart()
end, { desc = 'Restart nota daemon and refresh threads' })

local augroup = vim.api.nvim_create_augroup('Nota', { clear = true })

vim.api.nvim_create_autocmd('BufEnter', {
  group = augroup,
  callback = function(args)
    local model = require('nota.model')
    if not model.is_attached(args.buf) then
      model.attach(args.buf)
    end
  end,
})

vim.api.nvim_create_autocmd('BufWritePost', {
  group = augroup,
  callback = function(args)
    local model = require('nota.model')
    if model.is_attached(args.buf) then
      model.refresh(args.buf)
    end
  end,
})

vim.api.nvim_create_autocmd('CursorMoved', {
  group = augroup,
  callback = function(args)
    local display = require('nota.ui.display')
    display.on_cursor_moved(args.buf)
  end,
})

vim.api.nvim_create_autocmd('BufWipeout', {
  group = augroup,
  callback = function(args)
    local display = require('nota.ui.display')
    display.cleanup(args.buf)
  end,
})

vim.api.nvim_create_user_command('NotaInline', function(opts)
  local display = require('nota.ui.display')
  local scope = opts.args
  if scope == '' then
    local current = display.get_inline_scope()
    vim.notify('nota: inline scope is ' .. current)
  else
    display.set_inline_scope(scope)
  end
end, {
  nargs = '?',
  complete = function()
    return { 'off', 'cursor', 'buffer' }
  end,
  desc = 'Set nota inline conversation scope (off, cursor, buffer)',
})

vim.api.nvim_create_user_command('NotaShowResolved', function()
  local display = require('nota.ui.display')
  display.toggle_resolved()
  local statuses = display.get_show_statuses()
  vim.notify('nota: showing statuses: ' .. table.concat(statuses, ', '))
end, { desc = 'Toggle showing resolved nota threads' })

vim.api.nvim_create_user_command('NotaOpen', function()
  local float = require('nota.ui.float')
  float.open_conversation()
end, { desc = 'Open nota conversation at cursor' })

vim.api.nvim_create_user_command('NotaNew', function(opts)
  local float = require('nota.ui.float')
  local goal = opts.args ~= '' and opts.args or nil
  float.open_new({ goal = goal })
end, { nargs = '?', desc = 'Create new nota thread at cursor' })

vim.api.nvim_create_user_command('NotaReply', function()
  local float = require('nota.ui.float')
  float.open_reply()
end, { desc = 'Reply to nota thread at cursor' })

vim.api.nvim_create_user_command('NotaStatus', function()
  local float = require('nota.ui.float')
  float.change_status()
end, { desc = 'Change nota thread status at cursor' })

vim.api.nvim_create_user_command('NotaQuickfix', function(opts)
  local nota = require('nota')
  local repo_utils = require('nota.repo')

  local filters = {}
  for _, arg in ipairs(vim.split(opts.args, '%s+')) do
    local key, value = arg:match('^(%w+)=(.+)$')
    if key and value then
      filters[key] = value
    end
  end

  local scope = filters.scope or 'repo'
  filters.scope = nil

  if not filters.status then
    filters.status = 'open'
  elseif filters.status == 'all' then
    filters.status = nil
  end

  local function filter_threads(threads)
    local result = {}
    for _, t in ipairs(threads) do
      local match = true
      if filters.status and t.status ~= filters.status then
        match = false
      end
      if filters.goal and t.goal ~= filters.goal then
        match = false
      end
      if match then
        table.insert(result, t)
      end
    end
    return result
  end

  local function populate_quickfix(threads, repo)
    local items = {}
    for _, t in ipairs(threads) do
      if t.anchor and t.anchor.line then
        local file = t.resolvedAnchor and t.resolvedAnchor.file or ''
        local filename = repo and (repo .. '/' .. file) or file
        table.insert(items, {
          filename = filename,
          lnum = t.anchor.line,
          text = t.title or t.id,
        })
      end
    end
    table.sort(items, function(a, b)
      if a.filename ~= b.filename then
        return a.filename < b.filename
      end
      return a.lnum < b.lnum
    end)
    vim.fn.setqflist(items, 'r')
    vim.fn.setqflist({}, 'a', { title = 'Nota Threads' })
    if #items > 0 then
      vim.cmd('copen')
    else
      vim.notify('nota: no threads found', vim.log.levels.INFO)
    end
  end

  if scope == 'buffer' then
    local threads = nota.get_threads()
    threads = filter_threads(threads)
    local bufname = vim.api.nvim_buf_get_name(0)
    local repo = repo_utils.get_root(bufname)
    local items = {}
    for _, t in ipairs(threads) do
      if t.anchor and t.anchor.line then
        table.insert(items, {
          filename = bufname,
          lnum = t.anchor.line,
          text = t.title or t.id,
        })
      end
    end
    table.sort(items, function(a, b)
      return a.lnum < b.lnum
    end)
    vim.fn.setqflist(items, 'r')
    vim.fn.setqflist({}, 'a', { title = 'Nota Threads (buffer)' })
    if #items > 0 then
      vim.cmd('copen')
    else
      vim.notify('nota: no threads found in buffer', vim.log.levels.INFO)
    end
  else
    local bufname = vim.api.nvim_buf_get_name(0)
    local repo
    if bufname ~= '' then
      repo = repo_utils.get_root(bufname)
    end
    if not repo then
      repo = repo_utils.get_root(vim.fn.getcwd())
    end
    if not repo then
      vim.notify('nota: not in a repository', vim.log.levels.WARN)
      return
    end
    nota.ensure_loaded(repo, function(ok)
      if not ok then
        vim.notify('nota: failed to load threads', vim.log.levels.WARN)
        return
      end
      local threads = nota.get_all_threads(repo)
      threads = filter_threads(threads)
      populate_quickfix(threads, repo)
    end)
  end
end, {
  nargs = '*',
  desc = 'Populate quickfix with nota threads (scope=repo|buffer, status=X, goal=X)',
})
