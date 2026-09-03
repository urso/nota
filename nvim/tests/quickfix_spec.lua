local model = require('nota.model')
local client = require('nota.client')
local repo_utils = require('nota.repo')

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

local function setup_repo_threads(repo, threads)
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
    if thread.resolvedAnchor and thread.resolvedAnchor.file then
      local f = thread.resolvedAnchor.file
      if not repo_state.threads_by_file[f] then
        repo_state.threads_by_file[f] = {}
      end
      table.insert(repo_state.threads_by_file[f], thread)
    end
  end
end

local function parse_quickfix_args(args_str)
  local filters = {}
  for _, arg in ipairs(vim.split(args_str, '%s+')) do
    local key, value = arg:match('^(%w+)=(.+)$')
    if key and value then
      filters[key] = value
    end
  end
  return filters
end

local function filter_threads(threads, filters)
  local result = {}
  for _, t in ipairs(threads) do
    local match = true
    if filters.status and t.status ~= filters.status then
      match = false
    end
    if filters.goal and t.goal ~= filters.goal then
      match = false
    end
    if match then
      table.insert(result, t)
    end
  end
  return result
end

local function format_quickfix_items(threads, repo)
  local items = {}
  for _, t in ipairs(threads) do
    if t.anchor and t.anchor.line then
      local file = t.resolvedAnchor and t.resolvedAnchor.file or ''
      local filename = repo and (repo .. '/' .. file) or file
      table.insert(items, {
        filename = filename,
        lnum = t.anchor.line,
        text = t.title or t.id,
      })
    end
  end
  table.sort(items, function(a, b)
    if a.filename ~= b.filename then
      return a.filename < b.filename
    end
    return a.lnum < b.lnum
  end)
  return items
end

describe('quickfix', function()
  after_each(function()
    model._reset()
    vim.fn.setqflist({}, 'r')
  end)

  describe('parse_quickfix_args', function()
    it('parses empty args', function()
      local filters = parse_quickfix_args('')
      assert.same({}, filters)
    end)

    it('parses single filter', function()
      local filters = parse_quickfix_args('status=open')
      assert.same({ status = 'open' }, filters)
    end)

    it('parses multiple filters', function()
      local filters = parse_quickfix_args('status=open goal=bug')
      assert.same({ status = 'open', goal = 'bug' }, filters)
    end)

    it('parses scope filter', function()
      local filters = parse_quickfix_args('scope=buffer status=open')
      assert.same({ scope = 'buffer', status = 'open' }, filters)
    end)
  end)

  describe('filter_threads', function()
    local threads = {
      { id = '1', status = 'open', goal = 'bug', title = 'Bug 1' },
      { id = '2', status = 'open', goal = 'todo', title = 'Todo 1' },
      { id = '3', status = 'resolved', goal = 'bug', title = 'Bug 2' },
    }

    it('returns all threads with no filters', function()
      local result = filter_threads(threads, {})
      assert.equals(3, #result)
    end)

    it('filters by status', function()
      local result = filter_threads(threads, { status = 'open' })
      assert.equals(2, #result)
      assert.equals('1', result[1].id)
      assert.equals('2', result[2].id)
    end)

    it('filters by goal', function()
      local result = filter_threads(threads, { goal = 'bug' })
      assert.equals(2, #result)
      assert.equals('1', result[1].id)
      assert.equals('3', result[2].id)
    end)

    it('combines filters', function()
      local result = filter_threads(threads, { status = 'open', goal = 'bug' })
      assert.equals(1, #result)
      assert.equals('1', result[1].id)
    end)
  end)

  describe('format_quickfix_items', function()
    it('formats threads as quickfix items', function()
      local threads = {
        {
          id = '1',
          title = 'Bug in foo',
          anchor = { line = 10 },
          resolvedAnchor = { file = 'src/foo.lua', line = 10 },
        },
        {
          id = '2',
          title = 'Issue in bar',
          anchor = { line = 5 },
          resolvedAnchor = { file = 'src/bar.lua', line = 5 },
        },
      }
      local items = format_quickfix_items(threads, '/repo')
      assert.equals(2, #items)
      assert.equals('/repo/src/bar.lua', items[1].filename)
      assert.equals(5, items[1].lnum)
      assert.equals('Issue in bar', items[1].text)
      assert.equals('/repo/src/foo.lua', items[2].filename)
      assert.equals(10, items[2].lnum)
    end)

    it('sorts by filename then line', function()
      local threads = {
        { id = '1', title = 'A', anchor = { line = 20 }, resolvedAnchor = { file = 'b.lua', line = 20 } },
        { id = '2', title = 'B', anchor = { line = 10 }, resolvedAnchor = { file = 'b.lua', line = 10 } },
        { id = '3', title = 'C', anchor = { line = 5 }, resolvedAnchor = { file = 'a.lua', line = 5 } },
      }
      local items = format_quickfix_items(threads, '/repo')
      assert.equals('/repo/a.lua', items[1].filename)
      assert.equals('/repo/b.lua', items[2].filename)
      assert.equals(10, items[2].lnum)
      assert.equals('/repo/b.lua', items[3].filename)
      assert.equals(20, items[3].lnum)
    end)

    it('skips threads without anchor', function()
      local threads = {
        { id = '1', title = 'Has anchor', anchor = { line = 10 }, resolvedAnchor = { file = 'a.lua', line = 10 } },
        { id = '2', title = 'No anchor' },
      }
      local items = format_quickfix_items(threads, '/repo')
      assert.equals(1, #items)
      assert.equals('Has anchor', items[1].text)
    end)

    it('uses id as fallback text when no title', function()
      local threads = {
        { id = 'abc123', anchor = { line = 10 }, resolvedAnchor = { file = 'a.lua', line = 10 } },
      }
      local items = format_quickfix_items(threads, '/repo')
      assert.equals('abc123', items[1].text)
    end)
  end)

  describe('get_threads for buffer scope', function()
    it('returns threads for attached buffer', function()
      local repo = unique_repo()
      local file = 'test.lua'

      with_mocks({ repo = repo, nota_dir = true, threads = {} }, function()
        local bufnr = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_name(bufnr, repo .. '/' .. file)
        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)

        setup_repo_threads(repo, {
          {
            id = '1',
            title = 'Test thread',
            status = 'open',
            resolvedAnchor = { file = file, line = 5 },
          },
        })

        local threads = model.get_threads(bufnr)
        assert.equals(1, #threads)
        assert.equals('1', threads[1].id)

        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)
  end)

  describe('get_all_threads for repo scope', function()
    it('returns all threads in repo', function()
      local repo = unique_repo()

      with_mocks({ repo = repo, nota_dir = true, threads = {} }, function()
        local bufnr = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_name(bufnr, repo .. '/test.lua')
        vim.api.nvim_set_current_buf(bufnr)

        model.attach(bufnr)

        setup_repo_threads(repo, {
          { id = '1', title = 'Thread 1', status = 'open', resolvedAnchor = { file = 'a.lua', line = 5 } },
          { id = '2', title = 'Thread 2', status = 'open', resolvedAnchor = { file = 'b.lua', line = 10 } },
        })

        local threads = model.get_all_threads(repo)
        assert.equals(2, #threads)

        vim.api.nvim_buf_delete(bufnr, { force = true })
      end)
    end)
  end)
end)
