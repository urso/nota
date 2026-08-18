-- Client layer for nota daemon communication.
-- Provides typed wrappers over transport RPC calls.
-- Returns {err, result} tables matching vim.lsp.client:request_sync() pattern.

local transport = require('nota.transport')

local M = {}

function M.list(opts)
  local co = coroutine.running()
  if not co then
    error('client.list() must be called from a coroutine')
  end

  local response = { err = nil, result = nil }

  transport.request(nil, 'thread/list', opts or {}, function(err, result)
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

function M.get(id)
  local co = coroutine.running()
  if not co then
    error('client.get() must be called from a coroutine')
  end

  if not id or id == '' then
    return { err = { code = -32602, message = 'id is required' }, result = nil }
  end

  local response = { err = nil, result = nil }

  transport.request(nil, 'thread/get', { id = id }, function(err, result)
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

return M
