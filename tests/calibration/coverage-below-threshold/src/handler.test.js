const { handle } = require('./handler');

describe('handle', () => {
  test('returns ok', () => {
    expect(handle()).toBe('ok');
  });
});
