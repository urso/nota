local M = {}

local config = require('nota.config')
local transport = require('nota.transport')

M.config = config
M.transport = transport

function M.setup(opts)
  config.setup(opts)
end

return M
