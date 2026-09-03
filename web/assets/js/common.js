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

  App.copyToClipboard = function (text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(function () {
        App.toast('success', '已复制到剪贴板');
      }, function () {
        fallbackCopy(text);
      });
    } else {
      fallbackCopy(text);
    }
    function fallbackCopy(t) {
      try {
        var ta = document.createElement('textarea');
        ta.value = t;
        ta.style.position = 'fixed';
        ta.style.left = '-9999px';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
        App.toast('success', '已复制到剪贴板');
      } catch (e) {
        App.toast('error', '复制失败，请手动复制');
      }
    }
  };

  // 按 Unicode 码点计数，避免把汉字/emoji 按字节或 UTF-16 单元误算。
  App.countChars = function (s) {
    return Array.from(String(s == null ? '' : s)).length;
  };

  // 校验链接：需 HTTPS 且域名为 tieba.baidu.com（含子域名）。返回 null 表示通过，否则返回错误文案。
  App.validateLink = function (url) {
    var u = String(url == null ? '' : url).trim();
    if (!u) return '链接不能为空';
    var withScheme = u;
    if (!/^[a-z][a-z0-9+.-]*:\/\//i.test(withScheme)) withScheme = 'https://' + withScheme;
    var scheme, host;
    try {
      var parsed = new URL(withScheme);
      scheme = parsed.protocol;
      host = parsed.hostname;
    } catch (e) {
      return '链接格式无效';
    }
    if (scheme !== 'https:') return '链接必须使用 HTTPS 协议';
    if (host !== 'tieba.baidu.com' && !/\.tieba\.baidu\.com$/i.test(host)) {
      return '为了用户安全暂不支持该网站，仅支持 tieba.baidu.com，后续将逐步适配';
    }
    return null;
  };

  // 复制按钮（全局事件委托）：点击带 data-copy 属性的按钮，复制对应元素文本。
  document.addEventListener('click', function (e) {
    var btn = e.target.closest('.copy-btn');
    if (!btn) return;
    var targetId = btn.getAttribute('data-copy');
    if (!targetId) return;
    var el = document.getElementById(targetId);
    if (!el) return;
    var text = el.textContent || '';
    if (!text) return;
    App.copyToClipboard(text);
  });
})();