function processPayment(input) {
  if (input.card === 'declined') return { status: 'declined' };
  return { status: 'approved', amount: input.amount || 0 };
}
module.exports = { processPayment };
