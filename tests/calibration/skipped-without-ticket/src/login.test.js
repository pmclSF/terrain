const { login } = require('./login');

describe('login', () => {
  test('returns user on valid credentials', () => {
    expect(login('alice', 'pw')).toEqual({ name: 'alice' });
  });

  test.skip('handles MFA challenge', () => {
    expect(login('alice', 'pw', { mfa: true })).toEqual({ name: 'alice', mfa: true });
  });
});
