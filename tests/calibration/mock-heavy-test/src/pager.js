function send(user, message) { return { user, message, via: 'pager' }; }
module.exports = { send };
