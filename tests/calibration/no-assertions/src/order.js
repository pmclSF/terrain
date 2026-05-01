function createOrder(userId, payload) {
  return { userId, payload, status: 'created' };
}

function cancelOrder(orderId) {
  return { orderId, status: 'cancelled' };
}

function refundOrder(orderId) {
  return { orderId, status: 'refunded' };
}

module.exports = { createOrder, cancelOrder, refundOrder };
