local root = vim.fn.fnamemodify(debug.getinfo(1, 'S').source:sub(2), ':h:h')
local vendor = root .. '/vendor'
local plenary_path = vendor .. '/plenary.nvim'

if vim.fn.isdirectory(plenary_path) == 0 then
  vim.fn.mkdir(vendor, 'p')
  vim.fn.system({ 'git', 'clone', '--depth=1', 'https://github.com/nvim-lua/plenary.nvim', plenary_path })
end

vim.opt.rtp:prepend(root)
vim.opt.rtp:prepend(plenary_path)
vim.cmd('runtime plugin/plenary.vim')

