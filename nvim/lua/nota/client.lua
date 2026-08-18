-- Client layer for nota daemon communication.
-- Provides typed wrappers over transport RPC calls.
-- Returns {err, result} tables matching vim.lsp.client:request_sync() pattern.

local transport = require('nota.transport')

local M = {}

local function rpc_call(method, params)
  local co = coroutine.running()
  if not co then
    error(method .. ' must be called from a coroutine')
  end

  local response = { err = nil, result = nil }

  transport.request(nil, method, params, function(err, result)
    if err then
      if type(err) == 'table' then
        response.err = err
      else
        response.err = { code = -1, message = tostring(err) }
      end
    else
      response.result = result
    end
    coroutine.resume(co)
  end)

  coroutine.yield()
  return response
end

function M.list(opts)
  return rpc_call('thread/list', opts or {})
end

function M.get(id)
  if not id or id == '' then
    return { err = { code = -32602, message = 'id is required' }, result = nil }
  end
  return rpc_call('thread/get', { id = id })
end

return M
