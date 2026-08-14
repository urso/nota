local transport = require('nota.transport')

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
      local messages = {}
      local buffer = ''

      local function simulate_read(data)
        buffer = buffer .. data
        while true do
          local newline = buffer:find('\n')
          if not newline then break end
          local line = buffer:sub(1, newline - 1)
          buffer = buffer:sub(newline + 1)
          if line ~= '' then
            local ok, decoded = pcall(vim.json.decode, line)
            if ok then
              table.insert(messages, decoded)
            end
          end
        end
      end

      simulate_read('{"id":1,"met')
      assert.equals(0, #messages)

      simulate_read('hod":"test"}\n')
      assert.equals(1, #messages)
      assert.equals(1, messages[1].id)
      assert.equals('test', messages[1].method)
    end)

    it('handles multiple messages in one chunk', function()
      local messages = {}
      local buffer = ''

      local function simulate_read(data)
        buffer = buffer .. data
        while true do
          local newline = buffer:find('\n')
          if not newline then break end
          local line = buffer:sub(1, newline - 1)
          buffer = buffer:sub(newline + 1)
          if line ~= '' then
            local ok, decoded = pcall(vim.json.decode, line)
            if ok then
              table.insert(messages, decoded)
            end
          end
        end
      end

      simulate_read('{"id":1}\n{"id":2}\n{"id":3}\n')
      assert.equals(3, #messages)
      assert.equals(1, messages[1].id)
      assert.equals(2, messages[2].id)
      assert.equals(3, messages[3].id)
    end)

    it('handles invalid JSON gracefully', function()
      local messages = {}
      local errors = {}
      local buffer = ''

      local function simulate_read(data)
        buffer = buffer .. data
        while true do
          local newline = buffer:find('\n')
          if not newline then break end
          local line = buffer:sub(1, newline - 1)
          buffer = buffer:sub(newline + 1)
          if line ~= '' then
            local ok, decoded = pcall(vim.json.decode, line)
            if ok then
              table.insert(messages, decoded)
            else
              table.insert(errors, line)
            end
          end
        end
      end

      simulate_read('not json\n{"id":1}\n')
      assert.equals(1, #messages)
      assert.equals(1, #errors)
      assert.equals('not json', errors[1])
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
end)
