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
local float = require('nota.ui.float')

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

function M.open_conversation(thread)
  return float.open_conversation(thread)
end

function M.open_reply(thread)
  return float.open_reply(thread)
end

function M.open_new(opts)
  return float.open_new(opts)
end

function M.change_status(thread)
  return float.change_status(thread)
end

function M.threads_at_cursor(bufnr)
  return float.threads_at_cursor(bufnr)
end

function M.restart(repo)
  local transport = require('nota.transport')
  local repo_utils = require('nota.repo')

  repo = repo or repo_utils.get_root()
  if not repo then
    vim.notify('nota: not in a repository', vim.log.levels.WARN)
    return false
  end

  transport.reset(repo)

  vim.defer_fn(function()
    transport.spawn(repo)

    for bufnr, buf_state in pairs(model._get_attached_buffers()) do
      if vim.api.nvim_buf_is_valid(bufnr) and buf_state.repo == repo then
        model.refresh(bufnr)
      end
    end

    model.ensure_loaded(repo, function(ok)
      if ok then
        vim.notify('nota: daemon restarted', vim.log.levels.INFO)
      else
        vim.notify('nota: failed to reconnect', vim.log.levels.ERROR)
      end
    end)
  end, 100)

  return true
end

return M
