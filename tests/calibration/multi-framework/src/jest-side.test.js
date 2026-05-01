// Jest test file
const { add } = require('./math');

describe('add (jest)', () => {
  test('handles positives', () => {
    expect(add(1, 2)).toBe(3);
  });
});
