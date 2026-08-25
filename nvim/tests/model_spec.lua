local model = require('nota.model')
local client = require('nota.client')

local function with_mocks(mocks, fn)
  local original_systemlist = vim.fn.systemlist
  local original_isdirectory = vim.fn.isdirectory
  local original_client_list = client.list

  vim.fn.systemlist = function(cmd)
    if cmd:match('rev%-parse') then
      if mocks.repo then
        return { mocks.repo }
      end
      return {}
    end
    return original_systemlist(cmd)
  end
  vim.fn.isdirectory = function(path)
    if mocks.repo and mocks.nota_dir and path == mocks.repo .. '/.nota' then
      return 1
    end
    return 0
  end
  client.list = function(_, _)
    if mocks.client_error then
      return { err = { message = mocks.client_error } }
    end
    return { result = mocks.threads or {} }
  end

  local ok, err = pcall(fn)

  vim.fn.systemlist = original_systemlist
  vim.fn.isdirectory = original_isdirectory
  client.list = original_client_list

  if not ok then
    error(err)
  end
end

describe('model', function()
  after_each(function()
    model._reset()
  end)

  describe('attach', function()
    it('returns false for unnamed buffer', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      local result = model.attach(bufnr)
      assert.is_false(result)
      assert.is_false(model.is_attached(bufnr))
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('uses current buffer when bufnr not provided', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-nota-repo/test.lua')
      vim.api.nvim_set_current_buf(bufnr)

      with_mocks({ repo = '/tmp/test-nota-repo', nota_dir = true }, function()
        local result = model.attach()
        assert.is_true(result)
        assert.is_true(model.is_attached())
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('is idempotent for already attached buffer', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-nota-repo/test.lua')

      with_mocks({ repo = '/tmp/test-nota-repo', nota_dir = true }, function()
        local result1 = model.attach(bufnr)
        local result2 = model.attach(bufnr)

        assert.is_true(result1)
        assert.is_true(result2)
        assert.is_true(model.is_attached(bufnr))
      end)

      model.detach(bufnr)
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('returns false for buffer outside git repo', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/not-a-repo/test.lua')

      with_mocks({ repo = nil }, function()
        local result = model.attach(bufnr)
        assert.is_false(result)
        assert.is_false(model.is_attached(bufnr))
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('returns false for repo without .nota directory', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/repo-no-nota/test.lua')

      with_mocks({ repo = '/tmp/repo-no-nota', nota_dir = false }, function()
        local result = model.attach(bufnr)
        assert.is_false(result)
        assert.is_false(model.is_attached(bufnr))
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('stores repo and file in state', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/src/main.lua')

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true }, function()
        model.attach(bufnr)
        local state = model._get_state_internal(bufnr)

        assert.is_not_nil(state)
        assert.equals('/tmp/test-repo', state.repo)
        assert.equals('src/main.lua', state.file)
        assert.same({}, state.threads)
        assert.same({}, state.marks)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('places marks from client.list response', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3', 'line4', 'line5' })

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 2 } },
        { id = 't2', title = 'Thread 2', resolvedAnchor = { file = 'test.lua', line = 4 } },
      }

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local state = model._get_state_internal(bufnr)
        assert.equals(2, #state.threads)
        assert.is_not_nil(state.marks['t1'])
        assert.is_not_nil(state.marks['t2'])
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('clamps thread line to buffer size', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 100 } },
      }

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local result = model.get_threads(bufnr)
        assert.equals(1, #result)
        assert.equals(2, result[1].line)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('ignores threads with line = 0', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 0 } },
        { id = 't2', title = 'Thread 2', resolvedAnchor = { file = 'test.lua', line = 1 } },
      }

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local state = model._get_state_internal(bufnr)
        assert.is_nil(state.marks['t1'])
        assert.is_not_nil(state.marks['t2'])
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('handles threads without resolvedAnchor', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })

      local threads = {
        { id = 't1', title = 'Thread 1' },
        { id = 't2', title = 'Thread 2', resolvedAnchor = { file = 'test.lua', line = 1 } },
      }

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local result = model.get_threads(bufnr)
        assert.equals(2, #result)
        assert.is_nil(result[1].line)
        assert.equals(1, result[2].line)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('notifies on client.list error', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/test.lua')

      local notified = false
      local original_notify = vim.notify
      vim.notify = function(msg, level)
        if msg:match('failed to list threads') and level == vim.log.levels.WARN then
          notified = true
        end
      end

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true, client_error = 'connection refused' }, function()
        model.attach(bufnr)
        vim.wait(10)
      end)

      vim.notify = original_notify
      assert.is_true(notified)
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)

  describe('detach', function()
    it('clears state for attached buffer', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/test.lua')

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true }, function()
        model.attach(bufnr)
        assert.is_true(model.is_attached(bufnr))

        local result = model.detach(bufnr)

        assert.is_true(result)
        assert.is_false(model.is_attached(bufnr))
        assert.is_nil(model._get_state_internal(bufnr))
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('returns false for unattached buffer', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      local result = model.detach(bufnr)
      assert.is_false(result)
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)

  describe('is_attached', function()
    it('returns false for unknown buffer', function()
      assert.is_false(model.is_attached(99999))
    end)
  end)

  describe('get_threads', function()
    it('returns empty table for unattached buffer', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      local threads = model.get_threads(bufnr)
      assert.same({}, threads)
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('returns threads with current mark positions', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3', 'line4', 'line5' })

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true }, function()
        model.attach(bufnr)
        local state = model._get_state_internal(bufnr)
        state.threads = {
          { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 2 } },
          { id = 't2', title = 'Thread 2', resolvedAnchor = { file = 'test.lua', line = 4 } },
        }

        local ns = model._get_namespace()
        state.marks['t1'] = vim.api.nvim_buf_set_extmark(bufnr, ns, 1, 0, {})
        state.marks['t2'] = vim.api.nvim_buf_set_extmark(bufnr, ns, 3, 0, {})

        local threads = model.get_threads(bufnr)

        assert.equals(2, #threads)
        assert.equals('t1', threads[1].id)
        assert.equals(2, threads[1].line)
        assert.equals('t2', threads[2].id)
        assert.equals(4, threads[2].line)
      end)

      model.detach(bufnr)
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('tracks mark position after line insert', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3' })

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true }, function()
        model.attach(bufnr)
        local state = model._get_state_internal(bufnr)
        state.threads = {
          { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 2 } },
        }

        local ns = model._get_namespace()
        state.marks['t1'] = vim.api.nvim_buf_set_extmark(bufnr, ns, 1, 0, {})

        vim.api.nvim_buf_set_lines(bufnr, 0, 0, false, { 'new line at top' })

        local threads = model.get_threads(bufnr)
        assert.equals(3, threads[1].line)
      end)

      model.detach(bufnr)
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('returns copies not internal references', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true }, function()
        model.attach(bufnr)
        local state = model._get_state_internal(bufnr)
        state.threads = {
          { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 1 } },
        }

        local threads1 = model.get_threads(bufnr)
        threads1[1].title = 'Modified'

        local threads2 = model.get_threads(bufnr)
        assert.equals('Thread 1', threads2[1].title)
      end)

      model.detach(bufnr)
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)

  describe('buffer validity', function()
    it('detach handles already deleted buffer', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/test.lua')

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true }, function()
        model.attach(bufnr)
        assert.is_true(model.is_attached(bufnr))
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })

      local result = model.detach(bufnr)
      assert.is_true(result)
      assert.is_false(model.is_attached(bufnr))
    end)

    it('get_threads returns empty for deleted buffer', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/test.lua')

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true }, function()
        model.attach(bufnr)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })

      local threads = model.get_threads(bufnr)
      assert.same({}, threads)
    end)
  end)

  describe('extmark cleanup', function()
    it('clears marks on detach', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, '/tmp/test-repo/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })

      with_mocks({ repo = '/tmp/test-repo', nota_dir = true }, function()
        model.attach(bufnr)
        local state = model._get_state_internal(bufnr)
        state.threads = {
          { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 1 } },
        }

        local ns = model._get_namespace()
        state.marks['t1'] = vim.api.nvim_buf_set_extmark(bufnr, ns, 0, 0, {})

        local marks_before = vim.api.nvim_buf_get_extmarks(bufnr, ns, 0, -1, {})
        assert.equals(1, #marks_before)

        model.detach(bufnr)

        local marks_after = vim.api.nvim_buf_get_extmarks(bufnr, ns, 0, -1, {})
        assert.equals(0, #marks_after)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
end)
