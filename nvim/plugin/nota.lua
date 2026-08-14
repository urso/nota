if vim.g.loaded_nota then
  return
end
vim.g.loaded_nota = true

require('nota')

vim.api.nvim_create_user_command('NotaReset', function()
  require('nota.transport').reset()
end, { desc = 'Reset nota daemon connection' })
