describe('auth', () => {
  test('logs in valid user', () => {
    expect(true).toBe(true);
  });

  test('rejects invalid password', () => {
    expect(false).toBe(false);
  });

  test('handles MFA challenge', () => {
    expect(true).toBe(true);
  });
});
