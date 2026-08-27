if vim.g.loaded_nota then
  return
end
vim.g.loaded_nota = true

require('nota')

vim.api.nvim_create_user_command('NotaReset', function()
  require('nota.transport').reset()
end, { desc = 'Reset nota daemon connection' })

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
