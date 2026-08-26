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
