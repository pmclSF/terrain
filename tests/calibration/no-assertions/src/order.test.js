const { createOrder, cancelOrder, refundOrder } = require('./order');

describe('order', () => {
  test('creates an order', () => {
    createOrder('user-1', { items: ['a', 'b'] });
  });

  test('cancels an order', () => {
    cancelOrder('order-9');
  });

  test('refunds an order', () => {
    refundOrder('order-9');
  });
});
