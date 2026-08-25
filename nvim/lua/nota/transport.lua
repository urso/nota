-- Transport layer for nota daemon communication.
-- All callbacks from libuv (stdout, exit) are wrapped with vim.schedule,
-- so all dispatched handlers — notification handlers, request callbacks,
-- and coroutine resumes — run in the main Neovim event loop, never in
-- fast event context. Higher layers can safely call buffer/mark APIs.

local config = require('nota.config')
local repo_utils = require('nota.repo')

local M = {}

local connections = {}

local STATE_IDLE = 'idle'
local STATE_SPAWNING = 'spawning'
local STATE_CONNECTED = 'connected'
local STATE_ERROR = 'error'

local MAX_ATTEMPTS = 3
local RETRY_DELAYS = { 0, 500, 2000 }
local DEFAULT_TIMEOUT_MS = 30000

local notification_handlers = {}

local function get_repo_root()
  return repo_utils.get_root()
end

local function resolve_binary()
  local cfg = config.get()
  if cfg.binary then
    return cfg.binary
  end
  local path = vim.fn.exepath('nota')
  if path == '' then
    return nil
  end
  return path
end

local function new_connection()
  return {
    handle = nil,
    stdin = nil,
    stdout = nil,
    state = STATE_IDLE,
    attempts = 0,
    buffer = '',
    next_id = 0,
    pending = {},
    on_message = nil,
    on_exit = nil,
  }
end

local function get_or_create_connection(repo)
  if not connections[repo] then
    connections[repo] = new_connection()
  end
  return connections[repo]
end

local function schedule_retry(repo, delay_ms)
  if delay_ms == 0 then
    M.spawn(repo)
  else
    vim.defer_fn(function()
      local conn = connections[repo]
      if conn and conn.state ~= STATE_ERROR then
        M.spawn(repo)
      end
    end, delay_ms)
  end
end

local function handle_exit(repo)
  return function(code, signal)
    vim.schedule(function()
      local conn = connections[repo]
      if not conn then
        return
      end

      if conn.stdin then
        conn.stdin:close()
        conn.stdin = nil
      end
      if conn.stdout then
        conn.stdout:close()
        conn.stdout = nil
      end
      conn.handle = nil

      for _, req in pairs(conn.pending) do
        if req.timer then
          req.timer:stop()
          req.timer:close()
        end
        if req.thread then
          coroutine.resume(req.thread, nil, 'connection closed')
        elseif req.callback then
          req.callback('connection closed', nil)
        end
      end
      conn.pending = {}

      if conn.on_exit then
        conn.on_exit(code, signal)
      end

      if conn.state == STATE_ERROR then
        return
      end

      conn.attempts = conn.attempts + 1
      if conn.attempts >= MAX_ATTEMPTS then
        conn.state = STATE_ERROR
        vim.notify(
          string.format('nota: daemon failed after %d attempts (exit code %d)', conn.attempts, code or -1),
          vim.log.levels.ERROR
        )
        return
      end

      conn.state = STATE_IDLE
      local delay = RETRY_DELAYS[conn.attempts + 1] or RETRY_DELAYS[#RETRY_DELAYS]
      schedule_retry(repo, delay)
    end)
  end
end

local function handle_stdout(repo)
  return function(err, data)
    vim.schedule(function()
      local conn = connections[repo]
      if not conn then
        return
      end

      if err then
        vim.notify('nota: read error: ' .. tostring(err), vim.log.levels.ERROR)
        return
      end

      if not data then
        return
      end

      conn.buffer = conn.buffer .. data
      while true do
        local newline = conn.buffer:find('\n')
        if not newline then
          break
        end

        local line = conn.buffer:sub(1, newline - 1)
        conn.buffer = conn.buffer:sub(newline + 1)

        if line ~= '' then
          local ok, decoded = pcall(vim.json.decode, line)
          if not ok then
            vim.notify('nota: invalid JSON: ' .. line, vim.log.levels.WARN)
          elseif decoded.id ~= nil then
            local req = conn.pending[decoded.id]
            if req then
              conn.pending[decoded.id] = nil
              if req.timer then
                req.timer:stop()
                req.timer:close()
              end
              if req.thread then
                if decoded.error then
                  coroutine.resume(req.thread, nil, decoded.error)
                else
                  coroutine.resume(req.thread, decoded.result, nil)
                end
              elseif req.callback then
                if decoded.error then
                  req.callback(decoded.error, nil)
                else
                  req.callback(nil, decoded.result)
                end
              end
            end
          elseif decoded.method then
            local handler = notification_handlers[decoded.method]
            if handler then
              local handler_ok, handler_err = pcall(handler, decoded.params)
              if not handler_ok then
                vim.notify('nota: notification handler error: ' .. tostring(handler_err), vim.log.levels.WARN)
              end
            end
          end
          if conn.on_message then
            conn.on_message(decoded)
          end
        end
      end
    end)
  end
end

function M.spawn(repo)
  repo = repo or get_repo_root()
  if not repo then
    vim.notify('nota: not in a git repository', vim.log.levels.ERROR)
    return nil, 'not a git repository'
  end

  local conn = get_or_create_connection(repo)

  if conn.state == STATE_CONNECTED then
    return conn
  end

  if conn.state == STATE_ERROR then
    return nil, 'daemon in error state, call reset() first'
  end

  if conn.state == STATE_SPAWNING then
    return nil, 'spawn already in progress'
  end

  local binary = resolve_binary()
  if not binary then
    conn.state = STATE_ERROR
    local msg = 'nota: binary not found. Expected "nota" on PATH or set config.binary'
    vim.notify(msg, vim.log.levels.ERROR)
    return nil, msg
  end

  conn.state = STATE_SPAWNING
  conn.buffer = ''

  local stdin = vim.uv.new_pipe(false)
  local stdout = vim.uv.new_pipe(false)

  local handle, pid = vim.uv.spawn(binary, {
    args = { 'daemon' },
    stdio = { stdin, stdout, nil },
    cwd = repo,
  }, handle_exit(repo))

  if not handle then
    stdin:close()
    stdout:close()
    conn.state = STATE_IDLE
    conn.attempts = conn.attempts + 1

    if conn.attempts >= MAX_ATTEMPTS then
      conn.state = STATE_ERROR
      vim.notify(
        string.format('nota: failed to spawn daemon: %s', tostring(pid)),
        vim.log.levels.ERROR
      )
      return nil, 'spawn failed'
    end

    local delay = RETRY_DELAYS[conn.attempts + 1] or RETRY_DELAYS[#RETRY_DELAYS]
    schedule_retry(repo, delay)
    return nil, 'spawn failed, retrying'
  end

  conn.handle = handle
  conn.stdin = stdin
  conn.stdout = stdout
  conn.state = STATE_CONNECTED
  conn.attempts = 0

  stdout:read_start(handle_stdout(repo))

  return conn
end

function M.reset(repo)
  repo = repo or get_repo_root()
  if not repo then
    return
  end

  local conn = connections[repo]
  if not conn then
    return
  end

  if conn.handle then
    if conn.stdin then
      conn.stdin:close()
      conn.stdin = nil
    end
    if conn.stdout then
      conn.stdout:read_stop()
      conn.stdout:close()
      conn.stdout = nil
    end
    conn.handle:close()
    conn.handle = nil
  end

  for _, req in pairs(conn.pending) do
    if req.timer then
      req.timer:stop()
      req.timer:close()
    end
    if req.thread then
      coroutine.resume(req.thread, nil, 'connection reset')
    elseif req.callback then
      req.callback('connection reset', nil)
    end
  end
  conn.pending = {}

  conn.state = STATE_IDLE
  conn.attempts = 0
  conn.buffer = ''
end

function M.shutdown(repo)
  repo = repo or get_repo_root()
  if not repo then
    return
  end

  local conn = connections[repo]
  if not conn or not conn.handle then
    return
  end

  if conn.stdin then
    conn.stdin:close()
    conn.stdin = nil
  end
end

function M.get_connection(repo)
  repo = repo or get_repo_root()
  if not repo then
    return nil, 'not a git repository'
  end

  local conn = connections[repo]
  if conn and conn.state == STATE_CONNECTED then
    return conn
  end

  return M.spawn(repo)
end

function M.is_connected(repo)
  repo = repo or get_repo_root()
  if not repo then
    return false
  end

  local conn = connections[repo]
  return conn and conn.state == STATE_CONNECTED
end

function M.get_state(repo)
  repo = repo or get_repo_root()
  if not repo then
    return nil
  end

  local conn = connections[repo]
  return conn and conn.state or STATE_IDLE
end

function M.send(repo, message)
  repo = repo or get_repo_root()
  if not repo then
    return nil, 'not a git repository'
  end

  local conn = connections[repo]
  if not conn or conn.state ~= STATE_CONNECTED then
    return nil, 'not connected'
  end

  if not conn.stdin then
    return nil, 'stdin not available'
  end

  local ok, encoded = pcall(vim.json.encode, message)
  if not ok then
    return nil, 'failed to encode message: ' .. tostring(encoded)
  end

  local data = encoded .. '\n'
  conn.stdin:write(data, function(err)
    if err then
      vim.schedule(function()
        vim.notify('nota: write error: ' .. tostring(err), vim.log.levels.ERROR)
      end)
    end
  end)

  return true
end

function M.request(repo, method, params, callback)
  repo = repo or get_repo_root()

  local co = coroutine.running()
  local use_coroutine = co ~= nil and callback == nil

  if not repo then
    local err_msg = 'not a git repository'
    if use_coroutine then
      error(err_msg)
    end
    if callback then
      callback(err_msg, nil)
    end
    return nil, err_msg
  end

  local conn, err = M.get_connection(repo)
  if not conn then
    if use_coroutine then
      error(err)
    end
    if callback then
      callback(err, nil)
    end
    return nil, err
  end

  local id = conn.next_id
  conn.next_id = conn.next_id + 1

  local message = {
    jsonrpc = '2.0',
    id = id,
    method = method,
    params = params,
  }

  local timer = vim.uv.new_timer()
  timer:start(DEFAULT_TIMEOUT_MS, 0, function()
    vim.schedule(function()
      local req = conn.pending[id]
      if req then
        conn.pending[id] = nil
        if req.timer then
          req.timer:stop()
          req.timer:close()
        end
        if req.thread then
          coroutine.resume(req.thread, nil, 'request timeout')
        elseif req.callback then
          req.callback('request timeout', nil)
        end
      end
    end)
  end)

  if use_coroutine then
    conn.pending[id] = {
      thread = co,
      timer = timer,
    }
  else
    conn.pending[id] = {
      callback = callback,
      timer = timer,
    }
  end

  local ok, send_err = M.send(repo, message)
  if not ok then
    conn.pending[id] = nil
    timer:stop()
    timer:close()
    if use_coroutine then
      error(send_err)
    end
    if callback then
      callback(send_err, nil)
    end
    return nil, send_err
  end

  if use_coroutine then
    local result, resume_err = coroutine.yield()
    if resume_err then
      error(resume_err)
    end
    return result
  end

  return id
end

function M.on_notification(method, handler)
  notification_handlers[method] = handler
end

function M.off_notification(method)
  notification_handlers[method] = nil
end

return M
