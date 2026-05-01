const { add } = require('./math');

describe('add', () => {
  test.each([
    [1, 2, 3],
    [2, 3, 5],
    [-1, 1, 0],
  ])('add(%i, %i) = %i', (a, b, expected) => {
    expect(add(a, b)).toBe(expected);
  });
});
