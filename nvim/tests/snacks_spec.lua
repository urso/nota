local model = require('nota.model')

local test_counter = 0
local function unique_name(base)
  test_counter = test_counter + 1
  return base .. '_' .. test_counter
end

local function mock_snacks()
  local m = {
    picker = {
      pick = function(opts)
        m.last_pick_opts = opts
      end,
    },
    last_pick_opts = nil,
  }
  package.loaded['snacks'] = m
  return m
end

local function clear_snacks_mock()
  package.loaded['snacks'] = nil
  package.loaded['nota.integrations.snacks'] = nil
end

describe('snacks integration', function()
  local snacks_mock
  local test_repo

  before_each(function()
    test_repo = '/tmp/test-snacks-repo-' .. unique_name('repo')
    model._reset()
    snacks_mock = mock_snacks()
  end)

  after_each(function()
    clear_snacks_mock()
    model._reset()
  end)

  describe('when snacks not installed', function()
    it('returns stub functions that notify error', function()
      clear_snacks_mock()
      package.loaded['snacks'] = nil

      local notified = false
      local orig_notify = vim.notify
      vim.notify = function(msg, level)
        if msg:match('snacks.nvim') and level == vim.log.levels.ERROR then
          notified = true
        end
      end

      local snacks_nota = require('nota.integrations.snacks')
      snacks_nota.threads()
      assert.is_true(notified)

      vim.notify = orig_notify
      package.loaded['nota.integrations.snacks'] = nil
    end)
  end)

  describe('threads()', function()
    it('notifies when no threads in buffer', function()
      local snacks_nota = require('nota.integrations.snacks')

      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_name(bufnr, test_repo .. '/' .. unique_name('empty') .. '.lua')
      vim.api.nvim_set_current_buf(bufnr)

      local notified = false
      local orig_notify = vim.notify
      vim.notify = function(msg, level)
        if msg:match('No threads') and level == vim.log.levels.INFO then
          notified = true
        end
      end

      snacks_nota.threads()
      assert.is_true(notified)
      assert.is_nil(snacks_mock.last_pick_opts)

      vim.notify = orig_notify
      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)

  describe('threads_source', function()
    it('provides snacks-compatible source config', function()
      local snacks_nota = require('nota.integrations.snacks')
      local source = snacks_nota.threads_source

      assert.equals('Nota Threads', source.title)
      assert.is_function(source.finder)
      assert.is_function(source.preview)
      assert.is_function(source.confirm)
    end)
  end)

  describe('threads_all_source', function()
    it('uses async finder with ensure_loaded', function()
      local snacks_nota = require('nota.integrations.snacks')
      local source = snacks_nota.threads_all_source

      assert.equals('Nota Threads (All)', source.title)
      assert.is_function(source.finder)

      local finder_result = source.finder({}, {})
      assert.is_function(finder_result)
    end)

    it('returns async finder function', function()
      local snacks_nota = require('nota.integrations.snacks')
      local source = snacks_nota.threads_all_source
      local async_finder = source.finder({}, {})

      assert.is_function(async_finder)
    end)
  end)

  describe('preview', function()
    it('returns false for item without thread', function()
      local snacks_nota = require('nota.integrations.snacks')
      local source = snacks_nota.threads_source

      local result = source.preview({
        item = {},
        preview = {
          set_lines = function() end,
          highlight = function() end,
        },
      })
      assert.is_false(result)
    end)

    it('returns true and calls set_lines for valid thread', function()
      local snacks_nota = require('nota.integrations.snacks')
      local source = snacks_nota.threads_all_source

      local set_lines_called = false
      local highlight_called = false
      local result = source.preview({
        item = {
          file = '/tmp/test.lua',
          thread = {
            id = 't1',
            title = 'Test',
            status = 'open',
            comments = {},
          },
        },
        preview = {
          set_lines = function(_)
            set_lines_called = true
          end,
          highlight = function()
            highlight_called = true
          end,
        },
      })

      assert.is_true(result)
      assert.is_true(set_lines_called)
      assert.is_true(highlight_called)
    end)
  end)

  describe('confirm', function()
    it('closes picker and sets cursor', function()
      local snacks_nota = require('nota.integrations.snacks')
      local source = snacks_nota.threads_source

      local closed = false
      local mock_picker = {
        close = function()
          closed = true
        end,
      }

      local bufnr = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, { 'line 1', 'line 2', 'line 3' })
      vim.api.nvim_set_current_buf(bufnr)

      source.confirm(mock_picker, { pos = { 2, 0 } })

      assert.is_true(closed)
      local cursor = vim.api.nvim_win_get_cursor(0)
      assert.equals(2, cursor[1])

      vim.api.nvim_buf_delete(bufnr, { force = true })
    end)
  end)
end)
