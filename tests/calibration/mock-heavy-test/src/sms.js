function send(user, message) { return { user, message, via: 'sms' }; }
module.exports = { send };
