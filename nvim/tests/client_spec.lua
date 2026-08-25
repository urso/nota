local client = require('nota.client')
local transport = require('nota.transport')

describe('client', function()
  describe('get', function()
    it('validates empty id without yielding', function()
      local response = client.get('/repo', '')
      assert.is_not_nil(response)
      assert.is_not_nil(response.err)
      assert.equals(-32602, response.err.code)
      assert.equals('id is required', response.err.message)
    end)

    it('validates nil id without yielding', function()
      local response = client.get('/repo', nil)
      assert.is_not_nil(response)
      assert.equals(-32602, response.err.code)
    end)
  end)

  describe('create', function()
    it('validates missing anchor', function()
      local response = client.create('/repo', {})
      assert.is_not_nil(response.err)
      assert.equals(-32602, response.err.code)
      assert.equals('anchor is required', response.err.message)
    end)

    it('validates nil params', function()
      local response = client.create('/repo', nil)
      assert.is_not_nil(response.err)
      assert.equals(-32602, response.err.code)
    end)
  end)

  describe('add_comment', function()
    it('validates empty thread_id', function()
      local response = client.add_comment('/repo', '', 'body')
      assert.is_not_nil(response.err)
      assert.equals(-32602, response.err.code)
      assert.equals('thread_id is required', response.err.message)
    end)

    it('validates nil thread_id', function()
      local response = client.add_comment('/repo', nil, 'body')
      assert.is_not_nil(response.err)
      assert.equals(-32602, response.err.code)
    end)

    it('validates empty body', function()
      local response = client.add_comment('/repo', 'thread-1', '')
      assert.is_not_nil(response.err)
      assert.equals(-32602, response.err.code)
      assert.equals('body is required', response.err.message)
    end)

    it('validates nil body', function()
      local response = client.add_comment('/repo', 'thread-1', nil)
      assert.is_not_nil(response.err)
      assert.equals(-32602, response.err.code)
    end)
  end)

  describe('set_status', function()
    it('validates empty thread_id', function()
      local response = client.set_status('/repo', '', 'resolved')
      assert.is_not_nil(response.err)
      assert.equals(-32602, response.err.code)
      assert.equals('thread_id is required', response.err.message)
    end)

    it('validates nil thread_id', function()
      local response = client.set_status('/repo', nil, 'resolved')
      assert.is_not_nil(response.err)
      assert.equals(-32602, response.err.code)
    end)

    it('validates empty status', function()
      local response = client.set_status('/repo', 'thread-1', '')
      assert.is_not_nil(response.err)
      assert.equals(-32602, response.err.code)
      assert.equals('status is required', response.err.message)
    end)

    it('validates nil status', function()
      local response = client.set_status('/repo', 'thread-1', nil)
      assert.is_not_nil(response.err)
      assert.equals(-32602, response.err.code)
    end)
  end)

  describe('on_change', function()
    it('rejects non-function callback', function()
      local ok, err = client.on_change('not a function')
      assert.is_nil(ok)
      assert.equals(-32602, err.code)
      assert.equals('callback must be a function', err.message)
    end)

    it('rejects nil callback', function()
      local ok, err = client.on_change(nil)
      assert.is_nil(ok)
      assert.equals(-32602, err.code)
    end)

    it('rejects duplicate registration', function()
      local cb = function() end
      local ok1 = client.on_change(cb)
      assert.is_true(ok1)

      local ok2, err = client.on_change(cb)
      assert.is_nil(ok2)
      assert.equals(-2, err.code)
      assert.equals('callback already registered', err.message)

      client.off_change(cb)
    end)
  end)

  describe('notification dispatch', function()
    local captured_handler = nil
    local original_on_notification = nil
    local original_client = nil

    before_each(function()
      original_on_notification = transport.on_notification
      original_client = package.loaded['nota.client']

      transport.on_notification = function(method, handler)
        if method == 'thread/didChange' then
          captured_handler = handler
        end
        return original_on_notification(method, handler)
      end
      package.loaded['nota.client'] = nil
      client = require('nota.client')
    end)

    after_each(function()
      transport.on_notification = original_on_notification
      package.loaded['nota.client'] = original_client
      captured_handler = nil
    end)

    it('dispatches to multiple subscribers', function()
      local received1, received2 = nil, nil
      local cb1 = function(params) received1 = params end
      local cb2 = function(params) received2 = params end

      client.on_change(cb1)
      client.on_change(cb2)

      local test_params = { threadIds = { 'id1' }, anchoredFiles = { 'file.lua' } }
      captured_handler(test_params)

      assert.same(test_params, received1)
      assert.same(test_params, received2)

      client.off_change(cb1)
      client.off_change(cb2)
    end)

    it('isolates failing subscriber from others', function()
      local received = nil
      local failing_cb = function() error('subscriber error') end
      local working_cb = function(params) received = params end

      client.on_change(failing_cb)
      client.on_change(working_cb)

      local test_params = { threadIds = { 'id2' } }
      captured_handler(test_params)

      assert.same(test_params, received)

      client.off_change(failing_cb)
      client.off_change(working_cb)
    end)

    it('off_change removes only the target callback', function()
      local count1, count2 = 0, 0
      local cb1 = function() count1 = count1 + 1 end
      local cb2 = function() count2 = count2 + 1 end

      client.on_change(cb1)
      client.on_change(cb2)

      captured_handler({})
      assert.equals(1, count1)
      assert.equals(1, count2)

      local removed = client.off_change(cb1)
      assert.is_true(removed)

      captured_handler({})
      assert.equals(1, count1)
      assert.equals(2, count2)

      client.off_change(cb2)
    end)

    it('off_change returns false for unknown callback', function()
      local unknown_cb = function() end
      local result = client.off_change(unknown_cb)
      assert.is_false(result)
    end)
  end)
end)
