const { processPayment } = require('./payment');

expect.extend({
  toBeApprovedPayment(received) {
    const pass = received && received.status === 'approved';
    return {
      pass,
      message: () =>
        pass
          ? `expected ${JSON.stringify(received)} not to be approved`
          : `expected ${JSON.stringify(received)} to be approved`,
    };
  },
});

describe('payment', () => {
  test('approves a valid card', () => {
    expect(processPayment({ card: 'ok' })).toBeApprovedPayment();
  });

  test('rejects a declined card', () => {
    expect(processPayment({ card: 'declined' })).not.toBeApprovedPayment();
  });
});
