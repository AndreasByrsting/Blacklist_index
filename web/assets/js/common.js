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
    if (opts.body != null && !(opts.body instanceof FormData) && typeof opts.body !== 'string') {
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

  // 校验链接：需 HTTPS，域名为空则不限制；否则需命中白名单（含子域名）。
  // 返回 null 表示通过，否则返回错误文案。
  App.validateLink = function (url, domains) {
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
    if (domains && domains.length > 0) {
      var allowed = false;
      for (var i = 0; i < domains.length; i++) {
        if (host === domains[i] || host.lastIndexOf('.' + domains[i]) === host.length - domains[i].length - 1) {
          allowed = true; break;
        }
      }
      if (!allowed) {
        return '为了用户安全暂不支持该网站，仅支持 ' + domains.join('、') + '，后续将逐步适配';
      }
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

  // ===== 图片放大预览（Lightbox，全项目统一查看方式）=====
  // 点击带 data-img-url 的缩略图在当页弹层放大，支持左右切换与圆点定位当前图片。
  function ensureLightbox() {
    var box = document.getElementById('lightbox');
    if (box && box.querySelector('.lightbox-stage')) return box;
    if (box) box.remove();
    box = document.createElement('div');
    box.id = 'lightbox';
    box.className = 'lightbox-overlay';
    box.style.display = 'none';
    box.innerHTML =
      '<button class="lightbox-close" type="button" aria-label="关闭预览">✕</button>' +
      '<button class="lightbox-nav lightbox-prev" type="button" aria-label="上一张">‹</button>' +
      '<div class="lightbox-stage">' +
      '<img class="lightbox-img" src="" alt="预览">' +
      '<div class="lightbox-dots"></div>' +
      '</div>' +
      '<button class="lightbox-nav lightbox-next" type="button" aria-label="下一张">›</button>';
    document.body.appendChild(box);
    return box;
  }

  function renderLightbox(box, st) {
    box.querySelector('.lightbox-img').src = st.urls[st.index];
    box.querySelector('.lightbox-prev').style.display = st.index === 0 ? 'none' : '';
    box.querySelector('.lightbox-next').style.display = st.index === st.urls.length - 1 ? 'none' : '';
    var dots = box.querySelector('.lightbox-dots');
    if (st.urls.length > 1) {
      dots.innerHTML = st.urls.map(function (_, i) {
        return '<button class="lightbox-dot' + (i === st.index ? ' active' : '') +
          '" type="button" data-index="' + i + '" aria-label="第 ' + (i + 1) + ' 张"></button>';
      }).join('');
      dots.style.display = '';
    } else {
      dots.innerHTML = '';
      dots.style.display = 'none';
    }
  }

  App.openLightbox = function (urls, index) {
    if (!Array.isArray(urls)) urls = [urls];
    urls = urls.filter(Boolean);
    if (!urls.length) return;
    var box = ensureLightbox();
    var i = Math.max(0, Math.min(index || 0, urls.length - 1));
    box._lb = { urls: urls, index: i };
    renderLightbox(box, box._lb);
    box.style.display = 'flex';
  };

  App.stepLightbox = function (delta) {
    var box = document.getElementById('lightbox');
    var st = box && box._lb;
    if (!st) return;
    var i = st.index + delta;
    if (i < 0 || i >= st.urls.length) return;
    st.index = i;
    renderLightbox(box, st);
  };

  App.closeLightbox = function () {
    var box = document.getElementById('lightbox');
    if (!box) return;
    box.style.display = 'none';
    box._lb = null;
    var img = box.querySelector('.lightbox-img');
    if (img) img.src = '';
  };

  document.addEventListener('click', function (e) {
    var thumb = e.target.closest('[data-img-url]');
    if (thumb) {
      var grid = thumb.parentElement;
      var nodes = grid ? Array.prototype.slice.call(grid.querySelectorAll('[data-img-url]')) : [thumb];
      var urls = nodes.map(function (n) { return n.getAttribute('data-img-url'); });
      var idx = nodes.indexOf(thumb);
      App.openLightbox(urls, idx < 0 ? 0 : idx);
      return;
    }
    if (e.target.closest('.lightbox-close')) { App.closeLightbox(); return; }
    if (e.target.closest('.lightbox-prev')) { App.stepLightbox(-1); return; }
    if (e.target.closest('.lightbox-next')) { App.stepLightbox(1); return; }
    var dot = e.target.closest('.lightbox-dot');
    if (dot) {
      var box = document.getElementById('lightbox');
      var st = box && box._lb;
      if (st) {
        var target = parseInt(dot.getAttribute('data-index'), 10);
        if (!isNaN(target)) App.stepLightbox(target - st.index);
      }
      return;
    }
    var overlay = document.getElementById('lightbox');
    if (overlay && e.target === overlay) App.closeLightbox();
  });

  document.addEventListener('keydown', function (e) {
    var box = document.getElementById('lightbox');
    if (!box || box.style.display === 'none') return;
    if (e.key === 'Escape') App.closeLightbox();
    else if (e.key === 'ArrowLeft') App.stepLightbox(-1);
    else if (e.key === 'ArrowRight') App.stepLightbox(1);
  });
})();