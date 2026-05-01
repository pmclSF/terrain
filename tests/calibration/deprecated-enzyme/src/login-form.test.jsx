const { shallow } = require('enzyme');
const LoginForm = require('./login-form');

describe('LoginForm', () => {
  test('renders the username field', () => {
    const wrapper = shallow(<LoginForm />);
    expect(wrapper.find('#username').exists()).toBe(true);
  });

  test('renders the password field', () => {
    const wrapper = shallow(<LoginForm />);
    expect(wrapper.find('#password').exists()).toBe(true);
  });
});
