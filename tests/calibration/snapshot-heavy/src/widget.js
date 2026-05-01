function renderWidget(opts = {}) {
  return { theme: opts.theme || 'light', content: opts.content || '' };
}

function summarizeWidget() {
  return { count: 0, last: null };
}

module.exports = { renderWidget, summarizeWidget };
