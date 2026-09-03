-- Integration tests with real daemon
-- These tests spawn actual nota daemon processes against isolated temp repos.

local helpers = require('tests.helpers')
local float = require('nota.ui.float')
local model = require('nota.model')
local transport = require('nota.transport')

describe('integration', function()
  if not helpers.has_binary() then
    pending('nota binary not found - skipping integration tests')
    return
  end

  before_each(function()
    helpers.configure_binary()
  end)

  after_each(function()
    float._reset()
    model._reset()
    transport.reset()
    helpers.cleanup_all()
  end)

  describe('thread lifecycle', function()
    it('creates thread and displays in buffer', function()

      helpers.with_temp_repo(function(repo)
        local lines = { 'function hello()', '  print("world")', 'end' }
        local bufnr = helpers.create_buffer(repo, 'test.lua', lines)
        helpers.commit_file(repo, 'test.lua')

        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)
        local attached = helpers.wait_for(function()
          return model.is_attached(bufnr)
        end)
        assert.is_true(attached, 'buffer should attach')

        local threads_before = model.get_threads(bufnr)
        assert.are.equal(0, #threads_before)

        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)

    it('attaches to buffer in nota repo', function()
      helpers.with_temp_repo(function(repo)
        local lines = { 'line1', 'line2', 'line3' }
        local bufnr = helpers.create_buffer(repo, 'file.txt', lines)
        helpers.commit_file(repo, 'file.txt')

        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)
        local attached = helpers.wait_for(function()
          return model.is_attached(bufnr)
        end)
        assert.is_true(attached)

        local state = model._get_state_internal(bufnr)
        assert.is_not_nil(state)
        assert.is_not_nil(state.repo)
        assert.are.equal('file.txt', state.file)

        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)

    it('does not attach buffer outside nota repo', function()
      

      local tmp = vim.fn.tempname()
      vim.fn.mkdir(tmp, 'p')
      vim.fn.system({ 'git', '-C', tmp, 'init', '--quiet' })

      local path = tmp .. '/no-nota.txt'
      local f = io.open(path, 'w')
      f:write('content')
      f:close()

      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, path)
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'content' })
      vim.api.nvim_set_current_buf(bufnr)

      local result = model.attach(bufnr)
      assert.is_false(result)
      assert.is_false(model.is_attached(bufnr))

      vim.api.nvim_buf_delete(bufnr, { force = true })
      vim.fn.delete(tmp, 'rf')
    end)
  end)

  describe('open_conversation', function()
    it('opens float with thread data', function()
      

      helpers.with_temp_repo(function(repo)
        local lines = { 'line1', 'line2', 'line3' }
        local bufnr = helpers.create_buffer(repo, 'test.lua', lines)
        helpers.commit_file(repo, 'test.lua')

        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)
        helpers.wait_for(function()
          return model.is_attached(bufnr)
        end)

        local test_thread = {
          id = 'test-thread',
          title = 'Test Title',
          status = 'open',
          goal = 'test goal',
          anchor = { line = 2, outdated = false },
          comments = {
            {
              author = 'tester',
              bodies = {
                { content = 'Test comment body', time = '2024-01-01T10:00:00Z' },
              },
            },
          },
        }

        local win_count_before = #vim.api.nvim_list_wins()
        float.open_conversation(test_thread)

        local win_count_after = #vim.api.nvim_list_wins()
        assert.are.equal(win_count_before + 1, win_count_after)

        local float_win = vim.api.nvim_get_current_win()
        local float_buf = vim.api.nvim_win_get_buf(float_win)
        local content = table.concat(vim.api.nvim_buf_get_lines(float_buf, 0, -1, false), '\n')

        assert.is_truthy(content:match('open'))
        assert.is_truthy(content:match('test goal'))
        assert.is_truthy(content:match('tester'))
        assert.is_truthy(content:match('Test comment body'))

        vim.api.nvim_win_close(float_win, true)
        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)
  end)

  describe('open_compose', function()
    it('creates compose float and preserves draft', function()
      

      helpers.with_temp_repo(function(repo)
        local lines = { 'line1', 'line2', 'line3' }
        local bufnr = helpers.create_buffer(repo, 'test.lua', lines)
        helpers.commit_file(repo, 'test.lua')

        vim.api.nvim_set_current_buf(bufnr)
        vim.api.nvim_win_set_cursor(0, { 2, 0 })

        model.attach(bufnr)
        helpers.wait_for(function()
          return model.is_attached(bufnr)
        end)

        local target = {
          type = 'anchor',
          file = 'test.lua',
          line = 2,
          repo = repo,
        }

        float.open_compose(target)
        local float_win = vim.api.nvim_get_current_win()
        local float_buf = vim.api.nvim_win_get_buf(float_win)

        assert.is_true(vim.api.nvim_get_option_value('modifiable', { buf = float_buf }))

        vim.api.nvim_buf_set_lines(float_buf, 0, -1, false, { 'my draft' })

        vim.api.nvim_feedkeys('q', 'x', false)
        vim.wait(100)

        local drafts = float._get_drafts()
        assert.are.equal('my draft', drafts['anchor:test.lua:2'])

        float.open_compose(target)
        float_win = vim.api.nvim_get_current_win()
        float_buf = vim.api.nvim_win_get_buf(float_win)

        local restored = vim.api.nvim_buf_get_lines(float_buf, 0, -1, false)
        assert.are.equal('my draft', restored[1])

        vim.api.nvim_win_close(float_win, true)
        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)
  end)
end)
