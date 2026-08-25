-- Client layer for nota daemon communication.
-- Provides typed wrappers over transport RPC calls.
-- Returns {err, result} tables matching vim.lsp.client:request_sync() pattern.

local transport = require('nota.transport')
local errors = require('nota.errors')

local M = {}

local change_subscribers = {}

local function rpc_call(repo, method, params)
  local co = coroutine.running()
  if not co then
    error(method .. ' must be called from a coroutine')
  end

  local response = { err = nil, result = nil }

  transport.request(repo, method, params, function(err, result)
    if err then
      if type(err) == 'table' then
        response.err = err
      else
        response.err = errors.new(errors.CONNECTION, tostring(err))
      end
    else
      response.result = result
    end
    coroutine.resume(co)
  end)

  coroutine.yield()
  return response
end

function M.list(repo, opts)
  return rpc_call(repo, 'thread/list', opts or {})
end

function M.get(repo, id)
  if not id or id == '' then
    return { err = errors.new(errors.INVALID_PARAMS, 'id is required'), result = nil }
  end
  return rpc_call(repo, 'thread/get', { id = id })
end

function M.create(repo, params)
  params = params or {}
  if not params.anchor then
    return { err = errors.new(errors.INVALID_PARAMS, 'anchor is required'), result = nil }
  end
  return rpc_call(repo, 'thread/create', params)
end

function M.add_comment(repo, thread_id, body)
  if not thread_id or thread_id == '' then
    return { err = errors.new(errors.INVALID_PARAMS, 'thread_id is required'), result = nil }
  end
  if not body or body == '' then
    return { err = errors.new(errors.INVALID_PARAMS, 'body is required'), result = nil }
  end
  return rpc_call(repo, 'thread/addComment', { id = thread_id, body = body })
end

function M.set_status(repo, thread_id, status)
  if not thread_id or thread_id == '' then
    return { err = errors.new(errors.INVALID_PARAMS, 'thread_id is required'), result = nil }
  end
  if not status or status == '' then
    return { err = errors.new(errors.INVALID_PARAMS, 'status is required'), result = nil }
  end
  return rpc_call(repo, 'thread/setStatus', { id = thread_id, status = status })
end

function M.on_change(callback)
  if type(callback) ~= 'function' then
    return nil, errors.new(errors.INVALID_PARAMS, 'callback must be a function')
  end
  for _, cb in ipairs(change_subscribers) do
    if cb == callback then
      return nil, errors.new(errors.ALREADY_REGISTERED, 'callback already registered')
    end
  end
  table.insert(change_subscribers, callback)
  return true, nil
end

function M.off_change(callback)
  for i, cb in ipairs(change_subscribers) do
    if cb == callback then
      table.remove(change_subscribers, i)
      return true
    end
  end
  return false
end

transport.on_notification('thread/didChange', function(params)
  local snapshot = { unpack(change_subscribers) }
  for _, cb in ipairs(snapshot) do
    local ok, err = pcall(cb, params)
    if not ok then
      vim.notify('nota: change subscriber error: ' .. tostring(err), vim.log.levels.WARN)
    end
  end
end)

return M
