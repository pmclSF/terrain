function send(user, message) { return { user, message, via: 'slack' }; }
module.exports = { send };
