// Mocha test file
const { expect } = require('chai');
const { add } = require('./math');

describe('add (mocha)', function () {
  it('handles positives', function () {
    expect(add(1, 2)).to.equal(3);
  });
});
