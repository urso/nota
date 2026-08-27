--- nota.nvim - Thread display and authoring for Neovim
---
--- Public API:
---   nota.get_threads(bufnr?)      Returns threads anchored in the buffer.
---   nota.get_all_threads(repo?)   Returns all threads in the repo (uses extmarks for open buffers).
---   nota.ensure_loaded(repo?, cb) Ensures repo cache is loaded, calls cb(ok) when done.
---   nota.on_update(callback)      Subscribe to thread updates. Callback receives (bufnr, threads).
---   nota.off_update(callback)     Unsubscribe from thread updates.
---
--- Thread shape:
---   {
---     id       = string,   -- thread identifier
---     title    = string,   -- first line of first comment
---     status   = string,   -- "open", "resolved", etc.
---     goal     = string?,  -- thread goal if set
---     anchor   = {         -- resolved position in buffer
---       line     = number, -- 1-indexed line number (tracks edits via extmarks)
---       outdated = bool,   -- true if anchor resolution is uncertain
---     }?,
---     comments = table[],  -- list of {author, body, createdAt}
---   }

local M = {}

local config = require('nota.config')
local model = require('nota.model')

M.config = config

local augroup = nil

function M.setup(opts)
  config.setup(opts)

  if augroup then
    vim.api.nvim_del_augroup_by_id(augroup)
  end
  augroup = vim.api.nvim_create_augroup('nota', { clear = true })

  vim.api.nvim_create_autocmd('BufWipeout', {
    group = augroup,
    callback = function(args)
      model.detach(args.buf)
    end,
  })
end

function M.get_threads(bufnr)
  return model.get_threads(bufnr)
end

function M.on_update(callback)
  return model.on_update(callback)
end

function M.off_update(callback)
  return model.off_update(callback)
end

function M.get_all_threads(repo)
  return model.get_all_threads(repo)
end

function M.ensure_loaded(repo, callback)
  return model.ensure_loaded(repo, callback)
end

return M
