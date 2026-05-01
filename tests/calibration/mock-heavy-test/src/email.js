function send(user, message) { return { user, message, via: 'email' }; }
module.exports = { send };
