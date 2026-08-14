local config = require('nota.config')

local M = {}

local connections = {}

local STATE_IDLE = 'idle'
local STATE_SPAWNING = 'spawning'
local STATE_CONNECTED = 'connected'
local STATE_ERROR = 'error'

local MAX_ATTEMPTS = 3
local RETRY_DELAYS = { 0, 500, 2000 }

local function get_repo_root()
  local result = vim.fn.systemlist('git rev-parse --show-toplevel')
  if vim.v.shell_error ~= 0 or #result == 0 then
    return nil
  end
  return result[1]
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

        if line ~= '' and conn.on_message then
          local ok, decoded = pcall(vim.json.decode, line)
          if ok then
            conn.on_message(decoded)
          else
            vim.notify('nota: invalid JSON: ' .. line, vim.log.levels.WARN)
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

return M
