local M = {}

local config = require('nota.config')
local transport = require('nota.transport')
local client = require('nota.client')

M.config = config
M.transport = transport
M.client = client

function M.setup(opts)
  config.setup(opts)
end

return M
