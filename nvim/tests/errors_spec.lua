local errors = require('nota.errors')

describe('errors', function()
  describe('codes', function()
    it('matches daemon protocol codes', function()
      assert.equals(-32700, errors.PARSE_ERROR)
      assert.equals(-32600, errors.INVALID_REQUEST)
      assert.equals(-32601, errors.METHOD_NOT_FOUND)
      assert.equals(-32602, errors.INVALID_PARAMS)
      assert.equals(-32603, errors.INTERNAL)
      assert.equals(-32000, errors.NOT_FOUND)
      assert.equals(-1, errors.CONNECTION)
      assert.equals(-2, errors.ALREADY_REGISTERED)
    end)
  end)

  describe('new', function()
    it('creates error with code and message', function()
      local err = errors.new(-32000, 'thread not found')
      assert.equals(-32000, err.code)
      assert.equals('thread not found', err.message)
    end)
  end)

  describe('is_not_found', function()
    it('returns true for NOT_FOUND code', function()
      local err = { code = -32000, message = 'not found' }
      assert.is_true(errors.is_not_found(err))
    end)

    it('returns false for other codes', function()
      assert.is_false(errors.is_not_found({ code = -32602, message = 'invalid' }))
      assert.is_false(errors.is_not_found({ code = -1, message = 'connection' }))
    end)

    it('returns false for non-table', function()
      assert.is_false(errors.is_not_found('string error'))
      assert.is_false(errors.is_not_found(nil))
    end)
  end)

  describe('is_validation', function()
    it('returns true for INVALID_PARAMS code', function()
      local err = { code = -32602, message = 'invalid params' }
      assert.is_true(errors.is_validation(err))
    end)

    it('returns false for other codes', function()
      assert.is_false(errors.is_validation({ code = -32000, message = 'not found' }))
    end)

    it('returns false for non-table', function()
      assert.is_false(errors.is_validation('string error'))
      assert.is_false(errors.is_validation(nil))
    end)
  end)

  describe('is_internal', function()
    it('returns true for INTERNAL code', function()
      local err = { code = -32603, message = 'internal error' }
      assert.is_true(errors.is_internal(err))
    end)

    it('returns false for other codes', function()
      assert.is_false(errors.is_internal({ code = -32602, message = 'invalid' }))
    end)
  end)

  describe('is_connection', function()
    it('returns true for CONNECTION code', function()
      local err = { code = -1, message = 'connection closed' }
      assert.is_true(errors.is_connection(err))
    end)

    it('returns false for other codes', function()
      assert.is_false(errors.is_connection({ code = -32603, message = 'internal' }))
    end)

    it('returns false for non-table', function()
      assert.is_false(errors.is_connection('connection closed'))
      assert.is_false(errors.is_connection(nil))
    end)
  end)

  describe('is_already_registered', function()
    it('returns true for ALREADY_REGISTERED code', function()
      local err = { code = -2, message = 'callback already registered' }
      assert.is_true(errors.is_already_registered(err))
    end)

    it('returns false for other codes', function()
      assert.is_false(errors.is_already_registered({ code = -1, message = 'connection' }))
    end)

    it('returns false for non-table', function()
      assert.is_false(errors.is_already_registered('already registered'))
      assert.is_false(errors.is_already_registered(nil))
    end)
  end)
end)
