local model = require('nota.model')
local client = require('nota.client')

local test_counter = 0
local function unique_repo()
  test_counter = test_counter + 1
  return '/tmp/test-repo-' .. test_counter
end

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

local function setup_repo_threads(repo, file, threads)
  local repo_state = model._get_repo_state(repo)
  if not repo_state then
    return
  end
  repo_state.threads = threads
  repo_state.threads_by_id = {}
  repo_state.threads_by_file = {}
  for _, thread in ipairs(threads) do
    repo_state.threads_by_id[thread.id] = thread
    local f = thread.resolvedAnchor and thread.resolvedAnchor.file or file
    if not repo_state.threads_by_file[f] then
      repo_state.threads_by_file[f] = {}
    end
    table.insert(repo_state.threads_by_file[f], thread)
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
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
      vim.api.nvim_set_current_buf(bufnr)

      with_mocks({ repo = repo, nota_dir = true }, function()
        local result = model.attach()
        assert.is_true(result)
        assert.is_true(model.is_attached())
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('is idempotent for already attached buffer', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')

      with_mocks({ repo = repo, nota_dir = true }, function()
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
      vim.api.nvim_buf_set_name(bufnr, '/tmp/not-a-repo-unique/test.lua')

      with_mocks({ repo = nil }, function()
        local result = model.attach(bufnr)
        assert.is_false(result)
        assert.is_false(model.is_attached(bufnr))
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('returns false for repo without .nota directory', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')

      with_mocks({ repo = repo, nota_dir = false }, function()
        local result = model.attach(bufnr)
        assert.is_false(result)
        assert.is_false(model.is_attached(bufnr))
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('stores repo and file in state', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/src/main.lua')

      with_mocks({ repo = repo, nota_dir = true }, function()
        model.attach(bufnr)
        local state = model._get_state_internal(bufnr)

        assert.is_not_nil(state)
        assert.equals(repo, state.repo)
        assert.equals('src/main.lua', state.file)
        assert.same({}, state.marks)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('places marks from client.list response', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3', 'line4', 'line5' })

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 2 } },
        { id = 't2', title = 'Thread 2', resolvedAnchor = { file = 'test.lua', line = 4 } },
      }

      with_mocks({ repo = repo, nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local state = model._get_state_internal(bufnr)
        assert.is_not_nil(state.marks['t1'])
        assert.is_not_nil(state.marks['t2'])
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('clamps thread line to buffer size', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 100 } },
      }

      with_mocks({ repo = repo, nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local result = model.get_threads(bufnr)
        assert.equals(1, #result)
        assert.equals(2, result[1].anchor.line)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('ignores threads with line = 0', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 0 } },
        { id = 't2', title = 'Thread 2', resolvedAnchor = { file = 'test.lua', line = 1 } },
      }

      with_mocks({ repo = repo, nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local state = model._get_state_internal(bufnr)
        assert.is_nil(state.marks['t1'])
        assert.is_not_nil(state.marks['t2'])
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('handles threads without resolvedAnchor', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })

      local threads = {
        { id = 't1', title = 'Thread 1' },
        { id = 't2', title = 'Thread 2', resolvedAnchor = { file = 'test.lua', line = 1 } },
      }

      with_mocks({ repo = repo, nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local result = model.get_threads(bufnr)
        assert.equals(1, #result)
        assert.equals(1, result[1].anchor.line)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('notifies on client.list error', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')

      local notified = false
      local original_notify = vim.notify
      vim.notify = function(msg, level)
        if msg:match('failed to list threads') and level == vim.log.levels.WARN then
          notified = true
        end
      end

      with_mocks({ repo = repo, nota_dir = true, client_error = 'connection refused' }, function()
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
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')

      with_mocks({ repo = repo, nota_dir = true }, function()
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
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3', 'line4', 'line5' })

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 2 } },
        { id = 't2', title = 'Thread 2', resolvedAnchor = { file = 'test.lua', line = 4 } },
      }

      with_mocks({ repo = repo, nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local result = model.get_threads(bufnr)

        assert.equals(2, #result)
        assert.equals('t1', result[1].id)
        assert.equals(2, result[1].anchor.line)
        assert.equals('t2', result[2].id)
        assert.equals(4, result[2].anchor.line)
      end)

      model.detach(bufnr)
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('tracks mark position after line insert', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3' })

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 2 } },
      }

      with_mocks({ repo = repo, nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        vim.api.nvim_buf_set_lines(bufnr, 0, 0, false, { 'new line at top' })

        local result = model.get_threads(bufnr)
        assert.equals(3, result[1].anchor.line)
      end)

      model.detach(bufnr)
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('returns thread with full public API shape', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })

      local threads = {
        {
          id = 't1',
          title = 'Thread 1',
          status = 'open',
          goal = 'fix',
          comments = {
            { author = 'alice', body = 'First comment', createdAt = '2026-01-01' },
            { author = 'bob', body = 'Second comment', createdAt = '2026-01-02' },
          },
          resolvedAnchor = { file = 'test.lua', line = 1, outdated = true },
        },
      }

      with_mocks({ repo = repo, nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local result = model.get_threads(bufnr)
        assert.equals(1, #result)
        local t = result[1]

        assert.equals('t1', t.id)
        assert.equals('Thread 1', t.title)
        assert.equals('open', t.status)
        assert.equals('fix', t.goal)

        assert.equals(1, t.anchor.line)
        assert.is_true(t.anchor.outdated)

        assert.equals(2, #t.comments)
        assert.equals('alice', t.comments[1].author)
        assert.equals('First comment', t.comments[1].body)
        assert.equals('bob', t.comments[2].author)
      end)

      model.detach(bufnr)
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('returns fresh copies that do not share anchor state', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3' })

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 2 } },
      }

      with_mocks({ repo = repo, nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local threads1 = model.get_threads(bufnr)
        threads1[1].anchor.line = 999

        local threads2 = model.get_threads(bufnr)
        assert.equals(2, threads2[1].anchor.line)
      end)

      model.detach(bufnr)
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)

  describe('buffer validity', function()
    it('detach handles already deleted buffer', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')

      with_mocks({ repo = repo, nota_dir = true }, function()
        model.attach(bufnr)
        assert.is_true(model.is_attached(bufnr))
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })

      local result = model.detach(bufnr)
      assert.is_true(result)
      assert.is_false(model.is_attached(bufnr))
    end)

    it('get_threads returns empty for deleted buffer', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')

      with_mocks({ repo = repo, nota_dir = true }, function()
        model.attach(bufnr)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })

      local threads = model.get_threads(bufnr)
      assert.same({}, threads)
    end)
  end)

  describe('extmark cleanup', function()
    it('clears marks on detach', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 1 } },
      }

      with_mocks({ repo = repo, nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local ns = model._get_namespace()
        local marks_before = vim.api.nvim_buf_get_extmarks(bufnr, ns, 0, -1, {})
        assert.equals(1, #marks_before)

        model.detach(bufnr)

        local marks_after = vim.api.nvim_buf_get_extmarks(bufnr, ns, 0, -1, {})
        assert.equals(0, #marks_after)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)

  describe('get_all_threads', function()
    it('returns empty when repo not loaded', function()
      local threads = model.get_all_threads('/tmp/unknown-repo')
      assert.same({}, threads)
    end)

    it('returns all threads for loaded repo', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 1 } },
        { id = 't2', title = 'Thread 2', resolvedAnchor = { file = 'other.lua', line = 5 } },
      }

      with_mocks({ repo = repo, nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local result = model.get_all_threads(repo)
        assert.equals(2, #result)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('uses extmark positions for attached buffers', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3' })

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 2 } },
      }

      with_mocks({ repo = repo, nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        vim.api.nvim_buf_set_lines(bufnr, 0, 0, false, { 'new line' })

        local result = model.get_all_threads(repo)
        assert.equals(1, #result)
        assert.equals(3, result[1].anchor.line)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)

    it('uses resolved anchor for unattached files', function()
      local repo = unique_repo()
      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')

      local threads = {
        { id = 't1', title = 'Thread 1', resolvedAnchor = { file = 'test.lua', line = 1 } },
        { id = 't2', title = 'Thread 2', resolvedAnchor = { file = 'other.lua', line = 10 } },
      }

      with_mocks({ repo = repo, nota_dir = true, threads = threads }, function()
        model.attach(bufnr)
        vim.wait(10)

        local result = model.get_all_threads(repo)
        local t2 = nil
        for _, t in ipairs(result) do
          if t.id == 't2' then t2 = t end
        end
        assert.is_not_nil(t2)
        assert.equals(10, t2.anchor.line)
      end)

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
end)
