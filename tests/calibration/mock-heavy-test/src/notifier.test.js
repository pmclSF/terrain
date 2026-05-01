jest.mock('./email');
jest.mock('./sms');
jest.mock('./pager');
jest.mock('./slack');

const { notify } = require('./notifier');
const email = require('./email');
const sms = require('./sms');

describe('notifier', () => {
  test('routes to email channel', () => {
    notify('alice', { channel: 'email', message: 'hi' });
    expect(email.send).toHaveBeenCalled();
  });

  test('routes to sms channel', () => {
    notify('bob', { channel: 'sms', message: 'hi' });
    expect(sms.send).toHaveBeenCalled();
  });
});
