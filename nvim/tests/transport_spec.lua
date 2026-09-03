local transport = require('nota.transport')

-- Shared test helpers for message framing simulation
local function create_message_parser()
  local state = { messages = {}, errors = {}, buffer = '' }

  function state.read(data)
    state.buffer = state.buffer .. data
    while true do
      local newline = state.buffer:find('\n')
      if not newline then break end
      local line = state.buffer:sub(1, newline - 1)
      state.buffer = state.buffer:sub(newline + 1)
      if line ~= '' then
        local ok, decoded = pcall(vim.json.decode, line)
        if ok then
          table.insert(state.messages, decoded)
        else
          table.insert(state.errors, line)
        end
      end
    end
  end

  return state
end

-- Shared test helpers for request correlation simulation
local function create_request_handler()
  local state = { pending = {}, handlers = {} }

  function state.request(id, callback)
    state.pending[id] = { callback = callback }
  end

  function state.request_coroutine(id)
    local co = coroutine.running()
    state.pending[id] = { thread = co }
    local res, err = coroutine.yield()
    if err then error(err) end
    return res
  end

  function state.respond(msg)
    if msg.id ~= nil then
      local req = state.pending[msg.id]
      if req then
        state.pending[msg.id] = nil
        if req.thread then
          if msg.error then
            coroutine.resume(req.thread, nil, msg.error)
          else
            coroutine.resume(req.thread, msg.result, nil)
          end
        elseif req.callback then
          if msg.error then
            req.callback(msg.error, nil)
          else
            req.callback(nil, msg.result)
          end
        end
      end
    elseif msg.method then
      local h = state.handlers[msg.method]
      if h then h(msg.params) end
    end
  end

  return state
end

describe('transport', function()
  describe('send', function()
    it('returns error when not connected (auto-detected repo)', function()
      local ok, err = transport.send(nil, { method = 'test' })
      assert.is_nil(ok)
      assert.equals('not connected', err)
    end)

    it('returns error when not connected', function()
      local ok, err = transport.send('/fake/repo', { method = 'test' })
      assert.is_nil(ok)
      assert.equals('not connected', err)
    end)
  end)

  describe('message framing', function()
    it('accumulates partial messages', function()
      local parser = create_message_parser()

      parser.read('{"id":1,"met')
      assert.equals(0, #parser.messages)

      parser.read('hod":"test"}\n')
      assert.equals(1, #parser.messages)
      assert.equals(1, parser.messages[1].id)
      assert.equals('test', parser.messages[1].method)
    end)

    it('handles multiple messages in one chunk', function()
      local parser = create_message_parser()

      parser.read('{"id":1}\n{"id":2}\n{"id":3}\n')
      assert.equals(3, #parser.messages)
      assert.equals(1, parser.messages[1].id)
      assert.equals(2, parser.messages[2].id)
      assert.equals(3, parser.messages[3].id)
    end)

    it('handles invalid JSON gracefully', function()
      local parser = create_message_parser()

      parser.read('not json\n{"id":1}\n')
      assert.equals(1, #parser.messages)
      assert.equals(1, #parser.errors)
      assert.equals('not json', parser.errors[1])
    end)
  end)

  describe('json encoding', function()
    it('encodes messages correctly', function()
      local msg = { jsonrpc = '2.0', id = 1, method = 'test', params = { foo = 'bar' } }
      local encoded = vim.json.encode(msg)
      local decoded = vim.json.decode(encoded)
      assert.equals('2.0', decoded.jsonrpc)
      assert.equals(1, decoded.id)
      assert.equals('test', decoded.method)
      assert.equals('bar', decoded.params.foo)
    end)
  end)

  describe('request correlation', function()
    it('request returns error when not in git repo', function()
      local called = false
      local err_received = nil
      transport.request('/nonexistent/repo', 'test', {}, function(err, result)
        called = true
        err_received = err
      end)
      assert.is_true(called)
      assert.is_not_nil(err_received)
    end)

    it('routes response to correct pending request', function()
      local handler = create_request_handler()
      local results = {}

      handler.request(1, function(err, result) results[1] = { err = err, result = result } end)
      handler.request(2, function(err, result) results[2] = { err = err, result = result } end)
      handler.request(3, function(err, result) results[3] = { err = err, result = result } end)

      handler.respond({ id = 2, result = 'two' })
      assert.is_nil(results[1])
      assert.equals('two', results[2].result)
      assert.is_nil(results[3])

      handler.respond({ id = 1, result = 'one' })
      assert.equals('one', results[1].result)

      handler.respond({ id = 3, error = { message = 'failed' } })
      assert.equals('failed', results[3].err.message)
    end)

    it('routes notifications to handlers', function()
      local handler = create_request_handler()
      local received = nil
      handler.handlers['test/notify'] = function(params) received = params end

      handler.respond({ method = 'test/notify', params = { data = 'hello' } })
      assert.is_not_nil(received)
      assert.equals('hello', received.data)
    end)

    it('ignores unknown notifications', function()
      local handler = create_request_handler()
      assert.has_no.errors(function()
        handler.respond({ method = 'unknown/method', params = {} })
      end)
    end)
  end)

  describe('notification handlers', function()
    it('registers and unregisters handlers', function()
      local called = false
      local handler = function() called = true end

      transport.on_notification('test/event', handler)
      transport.off_notification('test/event')
    end)
  end)

  describe('coroutine await', function()
    it('detects coroutine context', function()
      local in_coroutine = false
      local co = coroutine.create(function()
        in_coroutine = coroutine.running() ~= nil
      end)
      coroutine.resume(co)
      assert.is_true(in_coroutine)
    end)

    it('resumes coroutine with result on success', function()
      local handler = create_request_handler()
      local result = nil

      local co = coroutine.create(function()
        result = handler.request_coroutine(1)
      end)
      coroutine.resume(co)
      assert.is_nil(result)

      handler.respond({ id = 1, result = 'success' })
      assert.equals('success', result)
    end)

    it('resumes coroutine with error on failure', function()
      local handler = create_request_handler()
      local caught_error = nil

      local co = coroutine.create(function()
        local ok, err = pcall(function()
          handler.request_coroutine(1)
        end)
        if not ok then
          caught_error = err
        end
      end)
      coroutine.resume(co)

      handler.respond({ id = 1, error = 'request failed' })
      assert.is_not_nil(caught_error)
      assert.is_true(caught_error:find('request failed') ~= nil)
    end)

    it('uses callback when not in coroutine', function()
      local handler = create_request_handler()
      local callback_result = nil

      handler.request(1, function(err, result)
        callback_result = result
      end)

      handler.respond({ id = 1, result = 'callback works' })
      assert.equals('callback works', callback_result)
    end)
  end)

  describe('state management', function()
    it('reports not connected for unknown repo', function()
      assert.is_false(transport.is_connected('/nonexistent/repo'))
    end)

    it('reset on unknown repo does not error', function()
      assert.has_no.errors(function()
        transport.reset('/nonexistent/repo')
      end)
    end)

    it('shutdown on unknown repo does not error', function()
      assert.has_no.errors(function()
        transport.shutdown('/nonexistent/repo')
      end)
    end)

    it('get_state returns nil when repo detection fails', function()
      local cwd = vim.fn.getcwd()
      vim.fn.chdir('/tmp')
      local state = transport.get_state(nil)
      vim.fn.chdir(cwd)
      assert.is_nil(state)
    end)
  end)

  describe('spawn', function()
    before_each(function()
      transport.reset('/tmp/fake-repo')
    end)

    it('returns error when spawn fails with bad binary', function()
      local config = require('nota.config')
      local original_binary = config.get().binary

      config.setup({ binary = '/nonexistent/nota-binary' })
      local conn, err = transport.spawn('/tmp/fake-repo')
      config.setup({ binary = original_binary })

      assert.is_nil(conn)
      assert.is_not_nil(err)
      assert.is_truthy(err:find('spawn failed'))
    end)

    it('returns error when repo is nil and not in git dir', function()
      local cwd = vim.fn.getcwd()
      vim.fn.chdir('/tmp')
      local conn, err = transport.spawn(nil)
      vim.fn.chdir(cwd)
      assert.is_nil(conn)
      assert.equals('not a git repository', err)
    end)
  end)
end)
