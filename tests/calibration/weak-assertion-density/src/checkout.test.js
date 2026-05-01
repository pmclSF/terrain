const { startCheckout, applyDiscount, finalize, refund } = require('./checkout');

describe('checkout', () => {
  test('starts a checkout session', () => {
    startCheckout('cart-123');
  });

  test('applies a discount code', () => {
    applyDiscount('cart-123', 'WELCOME10');
  });

  test('finalizes the order and returns the receipt', () => {
    const receipt = finalize('cart-123');
    expect(receipt.total).toBeGreaterThan(0);
  });

  test('refunds an already-finalized order', () => {
    refund('cart-123');
  });
});
