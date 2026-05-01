const email = require('./email');
const sms = require('./sms');
const pager = require('./pager');
const slack = require('./slack');

function notify(user, opts) {
  switch (opts.channel) {
    case 'email': return email.send(user, opts.message);
    case 'sms':   return sms.send(user, opts.message);
    case 'pager': return pager.send(user, opts.message);
    case 'slack': return slack.send(user, opts.message);
  }
}

module.exports = { notify };
