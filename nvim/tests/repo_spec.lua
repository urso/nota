local repo = require('nota.repo')

local function with_mocks(mocks, fn)
  local original_systemlist = vim.fn.systemlist
  local original_isdirectory = vim.fn.isdirectory

  vim.fn.systemlist = mocks.systemlist or original_systemlist
  vim.fn.isdirectory = mocks.isdirectory or original_isdirectory

  local ok, err = pcall(fn)

  vim.fn.systemlist = original_systemlist
  vim.fn.isdirectory = original_isdirectory

  if not ok then
    error(err)
  end
end

describe('repo', function()
  describe('get_root', function()
    it('returns nil for non-git directory', function()
      with_mocks({
        systemlist = function()
          return {}
        end,
      }, function()
        local result = repo.get_root('/tmp/not-a-repo/file.lua')
        assert.is_nil(result)
      end)
    end)

    it('returns repo root for file in git repo', function()
      with_mocks({
        systemlist = function(cmd)
          if cmd:match('rev%-parse') then
            return { '/home/user/project' }
          end
          return {}
        end,
      }, function()
        local result = repo.get_root('/home/user/project/src/main.lua')
        assert.equals('/home/user/project', result)
      end)
    end)

    it('uses cwd when no path provided', function()
      local called_cmd = nil
      with_mocks({
        systemlist = function(cmd)
          called_cmd = cmd
          return { '/some/repo' }
        end,
      }, function()
        repo.get_root()
      end)
      assert.equals('git rev-parse --show-toplevel', called_cmd)
    end)

    it('uses -C flag when path provided', function()
      local called_cmd = nil
      with_mocks({
        systemlist = function(cmd)
          called_cmd = cmd
          return { '/some/repo' }
        end,
      }, function()
        repo.get_root('/some/repo/file.lua')
      end)
      assert.truthy(called_cmd:match('%-C'))
    end)
  end)

  describe('has_nota_dir', function()
    it('returns true when .nota exists', function()
      with_mocks({
        isdirectory = function(path)
          if path == '/repo/.nota' then
            return 1
          end
          return 0
        end,
      }, function()
        local result = repo.has_nota_dir('/repo')
        assert.is_true(result)
      end)
    end)

    it('returns false when .nota missing', function()
      with_mocks({
        isdirectory = function()
          return 0
        end,
      }, function()
        local result = repo.has_nota_dir('/repo')
        assert.is_false(result)
      end)
    end)
  end)

  describe('get_relative_path', function()
    it('strips repo prefix from absolute path', function()
      local result = repo.get_relative_path('/home/user/project/src/main.lua', '/home/user/project')
      assert.equals('src/main.lua', result)
    end)

    it('returns path unchanged if not under repo', function()
      local result = repo.get_relative_path('/other/path/file.lua', '/home/user/project')
      assert.equals('/other/path/file.lua', result)
    end)

    it('handles repo root file', function()
      local result = repo.get_relative_path('/repo/file.lua', '/repo')
      assert.equals('file.lua', result)
    end)

    it('handles repo with trailing slash', function()
      local result = repo.get_relative_path('/repo/src/file.lua', '/repo/')
      assert.equals('src/file.lua', result)
    end)
  end)
end)
