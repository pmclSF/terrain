function startCheckout(cartId) {
  return { cartId, status: 'open' };
}

function applyDiscount(cartId, code) {
  return { cartId, code, applied: true };
}

function finalize(cartId) {
  return { cartId, total: 100, status: 'finalized' };
}

function refund(cartId) {
  return { cartId, status: 'refunded' };
}

module.exports = { startCheckout, applyDiscount, finalize, refund };
