local model = require('nota.model')

local function with_mocks(mocks, fn)
  local original_systemlist = vim.fn.systemlist
  local original_isdirectory = vim.fn.isdirectory

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

  local ok, err = pcall(fn)

  vim.fn.systemlist = original_systemlist
  vim.fn.isdirectory = original_isdirectory

  if not ok then
    error(err)
  end
end

describe('model', function()
  describe('attach', function()
    it('returns false for unnamed buffer', function()
      local bufnr = vim.api.nvim_create_buf(false, true)
      local result = model.attach(bufnr)
      assert.is_false(result)
      assert.is_false(model.is_attached(bufnr))
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
        local state = model.get_state(bufnr)

        assert.is_not_nil(state)
        assert.equals('/tmp/test-repo', state.repo)
        assert.equals('src/main.lua', state.file)
        assert.same({}, state.threads)
        assert.same({}, state.marks)
      end)

      model.detach(bufnr)
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
        assert.is_nil(model.get_state(bufnr))
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
end)
