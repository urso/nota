local float = require('nota.ui.float')
local model = require('nota.model')
local client = require('nota.client')

local test_counter = 0
local function unique_repo()
  test_counter = test_counter + 1
  return '/tmp/test-repo-' .. test_counter
end

local function with_model_mocks(mocks, fn)
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

local function setup_repo_threads(repo, file, threads, bufnr)
  local repo_state = model._get_repo_state(repo)
  if not repo_state then
    return
  end
  repo_state.threads = threads
  repo_state.threads_by_id = {}
  repo_state.threads_by_file = {}
  repo_state.loaded = true
  for _, thread in ipairs(threads) do
    repo_state.threads_by_id[thread.id] = thread
    local f = thread.resolvedAnchor and thread.resolvedAnchor.file or file
    if not repo_state.threads_by_file[f] then
      repo_state.threads_by_file[f] = {}
    end
    table.insert(repo_state.threads_by_file[f], thread)
  end
  if bufnr then
    local ns = model._get_namespace()
    local buf_state = model._get_state_internal(bufnr)
    if buf_state then
      vim.api.nvim_buf_clear_namespace(bufnr, ns, 0, -1)
      buf_state.marks = {}
      local line_count = vim.api.nvim_buf_line_count(bufnr)
      for _, thread in ipairs(threads) do
        if thread.resolvedAnchor and thread.resolvedAnchor.line and thread.resolvedAnchor.line > 0 then
          local line = math.max(1, math.min(thread.resolvedAnchor.line, math.max(1, line_count)))
          local mark_id = vim.api.nvim_buf_set_extmark(bufnr, ns, line - 1, 0, {})
          buf_state.marks[thread.id] = mark_id
        end
      end
    end
  end
end

describe('float', function()
  after_each(function()
    float._reset()
    model._reset()
  end)

  describe('threads_at_cursor', function()
    it('returns empty table when no threads at cursor', function()
      local repo = unique_repo()
      local file = 'test.lua'

      with_model_mocks({ repo = repo, nota_dir = true, threads = {} }, function()
        local bufnr = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_name(bufnr, repo .. '/' .. file)
        vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3' })
        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)
        vim.wait(500, function()
          return model.is_attached(bufnr)
        end)

        local threads = float.threads_at_cursor(bufnr)
        assert.are.equal(0, #threads)

        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)

    it('returns threads anchored at cursor line', function()
      local repo = unique_repo()
      local file = 'test.lua'
      local test_threads = {
        {
          id = 't1',
          title = 'Thread 1',
          status = 'open',
          resolvedAnchor = { file = file, line = 2 },
          comments = {},
        },
        {
          id = 't2',
          title = 'Thread 2',
          status = 'open',
          resolvedAnchor = { file = file, line = 5 },
          comments = {},
        },
      }

      with_model_mocks({ repo = repo, nota_dir = true, threads = test_threads }, function()
        local bufnr = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_name(bufnr, repo .. '/' .. file)
        vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3', 'line4', 'line5' })
        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)
        vim.wait(500, function()
          return model.is_attached(bufnr)
        end)

        setup_repo_threads(repo, file, test_threads, bufnr)

        vim.api.nvim_win_set_cursor(0, { 2, 0 })
        local threads = float.threads_at_cursor(bufnr)
        assert.are.equal(1, #threads)
        assert.are.equal('t1', threads[1].id)

        vim.api.nvim_win_set_cursor(0, { 5, 0 })
        threads = float.threads_at_cursor(bufnr)
        assert.are.equal(1, #threads)
        assert.are.equal('t2', threads[1].id)

        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)

    it('returns multiple threads when anchored to same line', function()
      local repo = unique_repo()
      local file = 'test.lua'
      local test_threads = {
        {
          id = 't1',
          title = 'Thread 1',
          status = 'open',
          resolvedAnchor = { file = file, line = 2 },
          comments = {},
        },
        {
          id = 't2',
          title = 'Thread 2',
          status = 'resolved',
          resolvedAnchor = { file = file, line = 2 },
          comments = {},
        },
      }

      with_model_mocks({ repo = repo, nota_dir = true, threads = test_threads }, function()
        local bufnr = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_name(bufnr, repo .. '/' .. file)
        vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3' })
        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)
        vim.wait(500, function()
          return model.is_attached(bufnr)
        end)

        setup_repo_threads(repo, file, test_threads, bufnr)

        vim.api.nvim_win_set_cursor(0, { 2, 0 })
        local threads = float.threads_at_cursor(bufnr)
        assert.are.equal(2, #threads)

        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)
  end)

  describe('drafts', function()
    it('stores and retrieves draft by anchor key', function()
      float._reset()

      local drafts = float._get_drafts()
      assert.is_nil(drafts['anchor:test.lua:5'])

      drafts['anchor:test.lua:5'] = 'draft content'
      assert.are.equal('draft content', drafts['anchor:test.lua:5'])
    end)

    it('stores and retrieves draft by reply key', function()
      float._reset()

      local drafts = float._get_drafts()
      drafts['reply:thread-123'] = 'reply draft'
      assert.are.equal('reply draft', drafts['reply:thread-123'])
    end)

    it('reset clears all drafts', function()
      local drafts = float._get_drafts()
      drafts['anchor:test.lua:5'] = 'content'
      drafts['reply:t1'] = 'reply'

      float._reset()
      drafts = float._get_drafts()
      assert.is_nil(drafts['anchor:test.lua:5'])
      assert.is_nil(drafts['reply:t1'])
    end)
  end)

  describe('open_conversation', function()
    it('creates floating window with thread content', function()
      local repo = unique_repo()
      local file = 'test.lua'
      local test_thread = {
        id = 't1',
        title = 'Test Thread',
        status = 'open',
        goal = 'fix bug',
        resolvedAnchor = { file = file, line = 2 },
        comments = {
          { author = 'alice', body = 'First comment', createdAt = '2024-01-01T10:00:00Z' },
          { author = 'bob', body = 'Second comment', createdAt = '2024-01-01T11:00:00Z' },
        },
      }

      with_model_mocks({ repo = repo, nota_dir = true, threads = { test_thread } }, function()
        local bufnr = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_name(bufnr, repo .. '/' .. file)
        vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3' })
        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)
        vim.wait(100, function()
          return model.is_attached(bufnr)
        end)

        setup_repo_threads(repo, file, { test_thread }, bufnr)

        local win_count_before = #vim.api.nvim_list_wins()
        float.open_conversation(test_thread)

        local win_count_after = #vim.api.nvim_list_wins()
        assert.are.equal(win_count_before + 1, win_count_after)

        local float_win = vim.api.nvim_get_current_win()
        local float_buf = vim.api.nvim_win_get_buf(float_win)
        local lines = vim.api.nvim_buf_get_lines(float_buf, 0, -1, false)

        assert.is_true(#lines > 0)
        assert.is_truthy(lines[1]:match('%[open%]'))
        assert.is_truthy(lines[1]:match('fix bug'))

        local content = table.concat(lines, '\n')
        assert.is_truthy(content:match('alice'))
        assert.is_truthy(content:match('First comment'))
        assert.is_truthy(content:match('bob'))

        vim.api.nvim_win_close(float_win, true)
        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)

    it('closes with q keymap', function()
      local test_thread = {
        id = 't1',
        title = 'Test',
        status = 'open',
        comments = {},
      }

      float.open_conversation(test_thread)
      local float_win = vim.api.nvim_get_current_win()
      assert.is_true(vim.api.nvim_win_is_valid(float_win))

      vim.api.nvim_feedkeys('q', 'x', false)
      vim.wait(50)

      assert.is_false(vim.api.nvim_win_is_valid(float_win))
    end)

    it('closes with Esc keymap', function()
      local test_thread = {
        id = 't1',
        title = 'Test',
        status = 'open',
        comments = {},
      }

      float.open_conversation(test_thread)
      local float_win = vim.api.nvim_get_current_win()
      assert.is_true(vim.api.nvim_win_is_valid(float_win))

      vim.api.nvim_feedkeys(vim.api.nvim_replace_termcodes('<Esc>', true, false, true), 'x', false)
      vim.wait(50)

      assert.is_false(vim.api.nvim_win_is_valid(float_win))
    end)
  end)

  describe('open_compose', function()
    it('creates editable floating window for new thread', function()
      local repo = unique_repo()
      local file = 'test.lua'

      with_model_mocks({ repo = repo, nota_dir = true, threads = {} }, function()
        local bufnr = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_name(bufnr, repo .. '/' .. file)
        vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3' })
        vim.api.nvim_set_current_buf(bufnr)
        vim.api.nvim_win_set_cursor(0, { 2, 0 })

        model.attach(bufnr)
        vim.wait(100, function()
          return model.is_attached(bufnr)
        end)

        local target = {
          type = 'anchor',
          file = file,
          line = 2,
          repo = repo,
        }

        float.open_compose(target)

        local float_win = vim.api.nvim_get_current_win()
        local float_buf = vim.api.nvim_win_get_buf(float_win)

        local modifiable = vim.api.nvim_get_option_value('modifiable', { buf = float_buf })
        assert.is_true(modifiable)

        vim.api.nvim_win_close(float_win, true)
        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)

    it('saves draft on close', function()
      local repo = unique_repo()
      local file = 'test.lua'

      with_model_mocks({ repo = repo, nota_dir = true, threads = {} }, function()
        local bufnr = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_name(bufnr, repo .. '/' .. file)
        vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3' })
        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)
        vim.wait(100, function()
          return model.is_attached(bufnr)
        end)

        local target = {
          type = 'anchor',
          file = file,
          line = 2,
          repo = repo,
        }

        float.open_compose(target)
        local float_win = vim.api.nvim_get_current_win()
        local float_buf = vim.api.nvim_win_get_buf(float_win)

        vim.api.nvim_buf_set_lines(float_buf, 0, -1, false, { 'my draft content' })

        vim.api.nvim_feedkeys('q', 'x', false)
        vim.wait(50)

        local drafts = float._get_drafts()
        assert.are.equal('my draft content', drafts['anchor:' .. file .. ':2'])

        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)

    it('restores draft on reopen', function()
      local repo = unique_repo()
      local file = 'test.lua'

      with_model_mocks({ repo = repo, nota_dir = true, threads = {} }, function()
        local bufnr = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_name(bufnr, repo .. '/' .. file)
        vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3' })
        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)
        vim.wait(100, function()
          return model.is_attached(bufnr)
        end)

        local target = {
          type = 'anchor',
          file = file,
          line = 2,
          repo = repo,
        }

        local drafts = float._get_drafts()
        drafts['anchor:' .. file .. ':2'] = 'restored draft'

        float.open_compose(target)
        local float_win = vim.api.nvim_get_current_win()
        local float_buf = vim.api.nvim_win_get_buf(float_win)

        local lines = vim.api.nvim_buf_get_lines(float_buf, 0, -1, false)
        assert.are.equal('restored draft', lines[1])

        vim.api.nvim_win_close(float_win, true)
        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)
  end)

  describe('open_new', function()
    it('anchors to cursor line', function()
      local repo = unique_repo()
      local file = 'test.lua'

      with_model_mocks({ repo = repo, nota_dir = true, threads = {} }, function()
        local bufnr = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_name(bufnr, repo .. '/' .. file)
        vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2', 'line3', 'line4', 'line5' })
        vim.api.nvim_set_current_buf(bufnr)
        vim.api.nvim_win_set_cursor(0, { 3, 0 })

        model.attach(bufnr)
        vim.wait(100, function()
          return model.is_attached(bufnr)
        end)

        float.open_new()
        local float_win = vim.api.nvim_get_current_win()
        local float_buf = vim.api.nvim_win_get_buf(float_win)

        vim.api.nvim_buf_set_lines(float_buf, 0, -1, false, { 'draft at line 3' })
        vim.api.nvim_feedkeys('q', 'x', false)
        vim.wait(50)

        local drafts = float._get_drafts()
        assert.are.equal('draft at line 3', drafts['anchor:' .. file .. ':3'])

        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)

    it('accepts goal option', function()
      local repo = unique_repo()
      local file = 'test.lua'
      local captured_params = nil

      with_model_mocks({ repo = repo, nota_dir = true, threads = {} }, function()
        local original_create = client.create
        client.create = function(_, params)
          captured_params = params
          return { result = { id = 'new-thread' } }
        end

        local bufnr = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_name(bufnr, repo .. '/' .. file)
        vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })
        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)
        vim.wait(100, function()
          return model.is_attached(bufnr)
        end)

        float.open_new({ goal = 'fix bug' })
        local float_win = vim.api.nvim_get_current_win()
        local float_buf = vim.api.nvim_win_get_buf(float_win)

        vim.api.nvim_buf_set_lines(float_buf, 0, -1, false, { 'test message' })

        vim.api.nvim_feedkeys(vim.api.nvim_replace_termcodes('<C-s>', true, false, true), 'x', false)
        vim.wait(200)

        if captured_params then
          assert.are.equal('fix bug', captured_params.goal)
        end

        client.create = original_create
        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)
  end)

  describe('change_status', function()
    it('calls set_status on selection', function()
      local repo = unique_repo()
      local file = 'test.lua'
      local captured_status = nil
      local test_thread = {
        id = 't1',
        title = 'Test',
        status = 'open',
        resolvedAnchor = { file = file, line = 2 },
        comments = {},
      }

      with_model_mocks({ repo = repo, nota_dir = true, threads = { test_thread } }, function()
        local original_set_status = client.set_status
        client.set_status = function(_, thread_id, status)
          captured_status = { thread_id = thread_id, status = status }
          return { result = {} }
        end

        local original_select = vim.ui.select
        vim.ui.select = function(items, _, on_choice)
          on_choice(items[2])
        end

        local bufnr = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_name(bufnr, repo .. '/' .. file)
        vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line1', 'line2' })
        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)
        vim.wait(100, function()
          return model.is_attached(bufnr)
        end)

        setup_repo_threads(repo, file, { test_thread })

        float.change_status(test_thread)
        vim.wait(200)

        assert.is_not_nil(captured_status)
        assert.are.equal('t1', captured_status.thread_id)
        assert.are.equal('resolved', captured_status.status)

        client.set_status = original_set_status
        vim.ui.select = original_select
        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)
  end)
end)
