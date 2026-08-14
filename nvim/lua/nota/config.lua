local M = {}

local defaults = {
  binary = nil,
}

M._config = vim.deepcopy(defaults)

function M.setup(opts)
  M._config = vim.tbl_deep_extend('force', defaults, opts or {})
end

function M.get()
  return M._config
end

return M
