(function () {
  'use strict';
  const App = (window.App = window.App || {});

  App.escapeHTML = function (s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  };

  App.sanitizeHTML = function (html) {
    if (window.DOMPurify) return DOMPurify.sanitize(html);
    return html;
  };

  App.renderMarkdown = function (md) {
    if (window.marked && window.DOMPurify) {
      return DOMPurify.sanitize(marked.parse(String(md == null ? '' : md)));
    }
    return App.escapeHTML(md);
  };

  App.api = async function (url, options) {
    options = options || {};
    const opts = Object.assign({}, options);
    const headers = Object.assign({}, opts.headers || {});
    if (opts.body != null && typeof opts.body !== 'string') {
      opts.body = JSON.stringify(opts.body);
      if (!headers['Content-Type']) headers['Content-Type'] = 'application/json';
    }
    opts.headers = headers;
    const resp = await fetch(url, opts);
    let data = {};
    try { data = await resp.json(); } catch (e) { /* noop */ }
    if (!resp.ok) {
      const err = new Error(data.message || ('请求失败 (HTTP ' + resp.status + ')'));
      err.status = resp.status;
      throw err;
    }
    return data;
  };

  App.toast = function (type, message) {
    const el = document.getElementById('toast');
    if (!el) return;
    const iconMap = { success: '✓', error: '✕', warning: '!', info: 'ℹ' };
    el.className = 'toast toast-' + type;
    el.querySelector('.toast-icon').textContent = iconMap[type] || 'ℹ';
    el.querySelector('.toast-msg').textContent = message;
    void el.offsetWidth;
    el.classList.add('show');
    clearTimeout(App._toastTimer);
    App._toastTimer = setTimeout(function () { el.classList.remove('show'); }, 2500);
  };

  App.setLoading = function (btn, loading, text) {
    if (loading) {
      if (!btn.dataset.prev) btn.dataset.prev = btn.innerHTML;
      btn.disabled = true;
      btn.innerHTML = '<span class="spinner"></span>' + App.escapeHTML(text || '处理中…');
    } else {
      btn.disabled = false;
      if (btn.dataset.prev) { btn.innerHTML = btn.dataset.prev; delete btn.dataset.prev; }
    }
  };
})();