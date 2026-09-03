-- Error codes and helpers for nota client.
-- Codes match daemon's pkg/rpc/protocol.go.

local M = {}

-- JSON-RPC standard codes
M.PARSE_ERROR = -32700
M.INVALID_REQUEST = -32600
M.METHOD_NOT_FOUND = -32601
M.INVALID_PARAMS = -32602
M.INTERNAL = -32603

-- Application codes
M.NOT_FOUND = -32000

-- Client-side codes
M.CONNECTION = -1
M.ALREADY_REGISTERED = -2

function M.is_not_found(err)
  return type(err) == 'table' and err.code == M.NOT_FOUND
end

function M.is_validation(err)
  return type(err) == 'table' and err.code == M.INVALID_PARAMS
end

function M.is_internal(err)
  return type(err) == 'table' and err.code == M.INTERNAL
end

function M.is_connection(err)
  return type(err) == 'table' and err.code == M.CONNECTION
end

function M.is_already_registered(err)
  return type(err) == 'table' and err.code == M.ALREADY_REGISTERED
end

function M.new(code, message)
  return { code = code, message = message }
end

return M
