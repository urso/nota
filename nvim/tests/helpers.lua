-- Test helpers for nota.nvim integration tests
-- Provides isolated temp repos with .nota/ directories for real daemon testing.

local M = {}

local test_counter = 0
local created_repos = {}

local function get_nota_root()
  for _, path in ipairs(vim.api.nvim_list_runtime_paths()) do
    if path:match('nota/nvim$') or path:match('nota[\\/]nvim$') then
      return vim.fn.fnamemodify(path, ':h')
    end
  end
  local src = debug.getinfo(1, 'S').source:sub(2)
  return vim.fn.fnamemodify(src, ':h:h:h')
end

function M.get_binary_path()
  local nota_root = get_nota_root()
  local binary = nota_root .. '/nota'
  if vim.fn.filereadable(binary) == 1 then
    return binary
  end
  binary = vim.fn.getcwd() .. '/nota'
  if vim.fn.filereadable(binary) == 1 then
    return binary
  end
  binary = vim.fn.getcwd() .. '/../nota'
  if vim.fn.filereadable(binary) == 1 then
    return vim.fn.fnamemodify(binary, ':p')
  end
  local path_binary = vim.fn.exepath('nota')
  if path_binary ~= '' then
    return path_binary
  end
  return nil
end

function M.has_binary()
  return M.get_binary_path() ~= nil
end

function M.create_temp_repo()
  test_counter = test_counter + 1
  local tmp = vim.fn.tempname()
  local repo = tmp .. '-nota-test-' .. test_counter

  vim.fn.mkdir(repo, 'p')
  vim.fn.mkdir(repo .. '/.nota', 'p')
  vim.fn.mkdir(repo .. '/.nota/threads', 'p')

  vim.fn.system({ 'git', '-C', repo, 'init', '--quiet' })
  vim.fn.system({ 'git', '-C', repo, 'config', 'user.email', 'test@test.com' })
  vim.fn.system({ 'git', '-C', repo, 'config', 'user.name', 'Test' })

  table.insert(created_repos, repo)
  return repo
end

function M.write_file(repo, path, content)
  local full_path = repo .. '/' .. path
  local dir = vim.fn.fnamemodify(full_path, ':h')
  if vim.fn.isdirectory(dir) == 0 then
    vim.fn.mkdir(dir, 'p')
  end
  local f = io.open(full_path, 'w')
  if f then
    f:write(content)
    f:close()
  end
  return full_path
end

function M.commit_file(repo, path)
  vim.fn.system({ 'git', '-C', repo, 'add', path })
  vim.fn.system({ 'git', '-C', repo, 'commit', '-m', 'Add ' .. path, '--quiet' })
end

function M.cleanup_repo(repo)
  if repo and vim.fn.isdirectory(repo) == 1 then
    vim.fn.delete(repo, 'rf')
  end
end

function M.cleanup_all()
  for _, repo in ipairs(created_repos) do
    M.cleanup_repo(repo)
  end
  created_repos = {}
end

function M.with_temp_repo(fn)
  local repo = M.create_temp_repo()
  local ok, err = pcall(fn, repo)
  M.cleanup_repo(repo)
  if not ok then
    error(err)
  end
end

function M.configure_binary()
  local binary = M.get_binary_path()
  if binary then
    require('nota.config').setup({ binary = binary })
    return true
  end
  return false
end

function M.skip_without_binary()
  if not M.has_binary() then
    pending('nota binary not found')
    return true
  end
  return false
end

function M.create_buffer(repo, path, lines)
  local full_path = M.write_file(repo, path, table.concat(lines, '\n'))
  local bufnr = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_name(bufnr, full_path)
  vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, lines)
  return bufnr
end

function M.wait_for(condition, timeout_ms)
  timeout_ms = timeout_ms or 5000
  return vim.wait(timeout_ms, condition, 50)
end

return M
