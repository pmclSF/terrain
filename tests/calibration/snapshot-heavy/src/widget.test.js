const { renderWidget, summarizeWidget } = require('./widget');

describe('widget', () => {
  test('renders the default widget', () => {
    expect(renderWidget()).toMatchSnapshot();
  });

  test('renders a customised widget', () => {
    expect(renderWidget({ theme: 'dark' })).toMatchSnapshot();
  });

  test('renders a widget with content', () => {
    expect(renderWidget({ content: 'hi' })).toMatchSnapshot();
  });

  test('summarises an empty widget', () => {
    expect(summarizeWidget()).toMatchSnapshot();
  });
});
