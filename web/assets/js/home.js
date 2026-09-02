(function () {
  'use strict';
  const App = window.App;
  const $ = function (id) { return document.getElementById(id); };

  const emailInput = $('emailInput');
  const searchBtn = $('searchBtn');
  const resultArea = $('resultArea');
  const announcementCard = $('announcementCard');
  const announcementContent = $('announcementContent');

  const EMAIL_RE = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/;

  function setResult(html) {
    resultArea.innerHTML = html;
    resultArea.classList.add('show');
  }

  function emptyState() {
    setResult(
      '<div class="result-empty">' +
        '<div class="status-icon" style="background:#F1F5F9;color:#94A3B8;">⌕</div>' +
        '<p>输入邮箱地址开始查询</p>' +
      '</div>'
    );
  }

  function invalidState() {
    setResult(
      '<div class="result-state result--invalid">' +
        '<div class="headline"><span class="status-icon">⚠</span><span>请输入有效的邮箱地址</span></div>' +
      '</div>'
    );
  }

  function safeState() {
    setResult(
      '<div class="result-state result--safe">' +
        '<div class="headline"><span class="status-icon">✓</span>' +
        '<div><div class="headline-text">该邮箱不在黑名单中</div>' +
        '<div class="desc">该邮箱在社区中是安全的</div></div></div>' +
      '</div>'
    );
  }

  function blockedState(d) {
    let detail = '';
    if (d.reason_html) {
      detail += '<div class="detail-row"><span class="detail-label">拉黑原因</span>' +
        '<div class="detail-value markdown-body">' + App.sanitizeHTML(d.reason_html) + '</div></div>';
    }
    if (d.event_link) {
      const url = String(d.event_link);
      const isSafe = /^https?:\/\//i.test(url);
      detail += '<div class="detail-row"><span class="detail-label">事件链接</span><div class="detail-value">' +
        (isSafe
          ? '<a href="' + App.escapeHTML(url) + '" target="_blank" rel="noopener noreferrer">' + App.escapeHTML(url) + '</a>'
          : App.escapeHTML(url)) +
        '</div></div>';
    }
    if (d.related_people) {
      detail += '<div class="detail-row"><span class="detail-label">相关人</span>' +
        '<div class="detail-value">' + App.escapeHTML(d.related_people) + '</div></div>';
    }
    if (d.banned_at) {
      detail += '<div class="detail-row"><span class="detail-label">拉黑时间</span>' +
        '<div class="detail-value">' + App.escapeHTML(d.banned_at) + '</div></div>';
    }
    setResult(
      '<div class="result-state result--blocked">' +
        '<div class="headline"><span class="status-icon">✕</span>' +
        '<div><div class="headline-text">该邮箱已被拉黑</div></div></div>' +
        (detail ? '<div class="blocked-detail">' + detail + '</div>' : '') +
      '</div>'
    );
  }

  async function search() {
    const email = emailInput.value.trim().toLowerCase();
    if (email === '') { emptyState(); return; }
    if (!EMAIL_RE.test(email)) { invalidState(); return; }

    App.setLoading(searchBtn, true, '查询中…');
    try {
      const data = await App.api('/api/v1/check?email=' + encodeURIComponent(email));
      if (data.blocked) blockedState(data);
      else safeState();
    } catch (e) {
      App.toast('error', e.message);
      emptyState();
    } finally {
      App.setLoading(searchBtn, false);
    }
  }

  searchBtn.addEventListener('click', search);
  emailInput.addEventListener('keydown', function (e) { if (e.key === 'Enter') search(); });
  document.addEventListener('keydown', function (e) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      emailInput.focus();
    }
  });

  (async function loadAnnouncement() {
    try {
      const data = await App.api('/api/v1/announcement');
      if (data.content_html) {
        announcementContent.innerHTML = App.sanitizeHTML(data.content_html);
        announcementCard.style.display = '';
      }
    } catch (e) { /* 静默忽略公告加载失败 */ }
  })();
})();