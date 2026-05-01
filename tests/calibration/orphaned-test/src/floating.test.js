describe('floating tests with no linked source', () => {
  test('basic arithmetic still works', () => {
    expect(1 + 1).toBe(2);
  });

  test('strings concatenate', () => {
    expect('a' + 'b').toBe('ab');
  });

  test('arrays push', () => {
    const a = [1];
    a.push(2);
    expect(a).toEqual([1, 2]);
  });
});
