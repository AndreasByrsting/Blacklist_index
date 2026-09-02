(function () {
  'use strict';
  const App = window.App;
  const $ = function (id) { return document.getElementById(id); };
  const esc = App.escapeHTML;

  const EMAIL_RE = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/;
  const URL_RE = /^https?:\/\//i;

  const state = {
    bl: { mode: 'list', query: '', page: 1, pageSize: 10 },
    audit: { action: '', page: 1, pageSize: 20 },
    delete: { id: null, email: '', mode: 'soft' }
  };

  // ===== 视图切换 =====
  function showLogin() {
    $('dashboardView').style.display = 'none';
    $('loginView').style.display = 'flex';
  }
  function showDashboard() {
    $('loginView').style.display = 'none';
    $('dashboardView').style.display = 'flex';
    switchTab('blacklist');
    loadBlacklist();
  }

  // ===== 统一请求（401 自动回到登录页） =====
  async function adminApi(url, options) {
    try {
      return await App.api(url, options);
    } catch (e) {
      if (e.status === 401) showLogin();
      throw e;
    }
  }

  function qs(params) {
    return Object.keys(params)
      .filter(function (k) { return params[k] !== '' && params[k] != null; })
      .map(function (k) { return encodeURIComponent(k) + '=' + encodeURIComponent(params[k]); })
      .join('&');
  }

  // ===== Tab 切换 =====
  function switchTab(name) {
    document.querySelectorAll('.tab').forEach(function (t) {
      t.classList.toggle('active', t.getAttribute('data-tab') === name);
    });
    document.querySelectorAll('.panel').forEach(function (p) {
      p.classList.toggle('active', p.id === 'tab-' + name);
    });
    if (name === 'announcement') loadAnnouncement();
    if (name === 'audit') loadAudit();
  }

  // ===== 弹窗 =====
  function openModal(id) { $(id).classList.add('show'); }
  function closeModal(id) { $(id).classList.remove('show'); }

  // ===== 登录 =====
  $('loginForm').addEventListener('submit', async function (e) {
    e.preventDefault();
    const password = $('loginPassword').value;
    const totp = $('loginTotp').value.trim();
    if (!password) { $('loginError').textContent = '请输入密码'; return; }
    if (!/^\d{6}$/.test(totp)) { $('loginError').textContent = '请输入 6 位动态口令'; return; }

    $('loginError').textContent = '';
    App.setLoading($('loginBtn'), true, '登录中…');
    try {
      await App.api('/api/v1/admin/login', { method: 'POST', body: { password: password, totp: totp } });
      showDashboard();
    } catch (err) {
      $('loginError').textContent = err.message;
    } finally {
      App.setLoading($('loginBtn'), false);
    }
  });

  // ===== 退出登录 =====
  $('logoutBtn').addEventListener('click', async function () {
    try { await App.api('/api/v1/admin/logout', { method: 'POST' }); } catch (e) { /* noop */ }
    showLogin();
  });

  // ===== Tab 按钮 =====
  document.querySelectorAll('.tab').forEach(function (t) {
    t.addEventListener('click', function () { switchTab(t.getAttribute('data-tab')); });
  });

  // ===== 黑名单：分页绑定 =====
  $('blPagination').addEventListener('click', function (e) {
    const b = e.target.closest('[data-page]');
    if (!b) return;
    const p = Number(b.getAttribute('data-page'));
    if (!p || p === state.bl.page) return;
    state.bl.page = p;
    loadBlacklist();
  });

  // ===== 黑名单：操作委托 =====
  $('tab-blacklist').addEventListener('click', function (e) {
    const btn = e.target.closest('[data-action]');
    if (!btn) return;
    const action = btn.getAttribute('data-action');
    const id = Number(btn.getAttribute('data-id'));
    const email = btn.getAttribute('data-email') || '';
    if (action === 'delete') openDelete(id, email, 'soft');
    else if (action === 'permanent') openDelete(id, email, 'permanent');
    else if (action === 'restore') restoreItem(id);
  });

  // ===== 黑名单：搜索 =====
  $('blSearchBtn').addEventListener('click', function () {
    state.bl.query = $('blSearch').value.trim();
    state.bl.page = 1;
    loadBlacklist();
  });
  $('blSearch').addEventListener('keydown', function (e) {
    if (e.key === 'Enter') { state.bl.query = $('blSearch').value.trim(); state.bl.page = 1; loadBlacklist(); }
  });

  // ===== 黑名单：添加 / 回收站 =====
  $('addBtn').addEventListener('click', function () {
    $('addForm').reset();
    openModal('addModal');
  });
  $('trashBtn').addEventListener('click', function () {
    state.bl.mode = state.bl.mode === 'trash' ? 'list' : 'trash';
    state.bl.page = 1;
    $('trashBtn').textContent = state.bl.mode === 'trash' ? '返回列表' : '回收站';
    loadBlacklist();
  });

  function loadBlacklist() {
    const tbody = $('blTableWrap');
    tbody.innerHTML = '<div class="skeleton" style="height:120px;margin:16px;"></div>';
    const url = '/api/v1/admin/list?' + qs({
      q: state.bl.query, deleted: state.bl.mode === 'trash' ? 1 : 0,
      page: state.bl.page, page_size: state.bl.pageSize
    });
    adminApi(url).then(function (data) {
      renderBlacklist(data.list, data.total);
    }).catch(function (e) {
      App.toast('error', e.message);
      renderBlacklist([], 0);
    });
  }

  function renderBlacklist(list, total) {
    const isTrash = state.bl.mode === 'trash';
    $('blTableWrap').innerHTML = tableHTML(list, isTrash);
    $('blCardList').innerHTML = cardHTML(list, isTrash);
    $('blEmpty').style.display = list.length === 0 ? '' : 'none';
    $('blEmptyText').textContent = isTrash ? '回收站为空' : '暂无黑名单记录';
    $('blPagination').innerHTML = paginationHTML(state.bl.page, total, state.bl.pageSize);
  }

  function actionButtons(item, isTrash) {
    if (isTrash) {
      return '<button class="btn btn-secondary btn-sm" data-action="restore" data-id="' + item.id + '" data-email="' + esc(item.email) + '">还原</button>' +
        '<button class="btn btn-danger btn-sm" data-action="permanent" data-id="' + item.id + '" data-email="' + esc(item.email) + '">永久删除</button>';
    }
    return '<button class="btn btn-danger btn-sm" data-action="delete" data-id="' + item.id + '" data-email="' + esc(item.email) + '">删除</button>';
  }

  function tableHTML(list, isTrash) {
    const head = '<thead><tr><th>邮箱</th><th>拉黑原因</th><th>相关人</th><th>Ban 时间</th><th style="text-align:right;">操作</th></tr></thead>';
    const rows = list.map(function (item) {
      return '<tr>' +
        '<td class="cell-email">' + esc(item.email) + '</td>' +
        '<td class="cell-reason" title="' + esc(item.ban_reason_raw || '') + '">' + esc(item.ban_reason_raw || '—') + '</td>' +
        '<td>' + esc(item.event_related_people || '—') + '</td>' +
        '<td>' + esc(item.banned_at || '—') + '</td>' +
        '<td class="cell-actions">' + actionButtons(item, isTrash) + '</td>' +
      '</tr>';
    }).join('');
    return '<table class="data-table">' + head + '<tbody>' + rows + '</tbody></table>';
  }

  function cardHTML(list, isTrash) {
    return list.map(function (item) {
      return '<div class="list-item">' +
        '<div class="li-field"><span class="li-label">邮箱</span><span class="li-value">' + esc(item.email) + '</span></div>' +
        '<div class="li-field"><span class="li-label">原因</span><span class="li-value">' + esc(item.ban_reason_raw || '—') + '</span></div>' +
        '<div class="li-field"><span class="li-label">相关人</span><span class="li-value">' + esc(item.event_related_people || '—') + '</span></div>' +
        '<div class="li-field"><span class="li-label">Ban 时间</span><span class="li-value">' + esc(item.banned_at || '—') + '</span></div>' +
        '<div class="li-actions">' + actionButtons(item, isTrash) + '</div>' +
      '</div>';
    }).join('');
  }

  // ===== 添加流程 =====
  $('addNextBtn').addEventListener('click', function () {
    const email = $('addEmail').value.trim().toLowerCase();
    const reason = $('addReason').value.trim();
    const link = $('addLink').value.trim();
    const people = $('addPeople').value.trim();
    const bannedAt = $('addBannedAt').value.trim();

    if (!EMAIL_RE.test(email)) { App.toast('warning', '请输入有效的邮箱地址'); return; }
    if (link && !URL_RE.test(link)) { App.toast('warning', '事件链接需以 http(s):// 开头'); return; }

    const summary = '<div><strong>邮箱：</strong>' + esc(email) + '</div>' +
      '<div><strong>拉黑原因：</strong>' + (reason ? esc(reason) : '（未填写）') + '</div>' +
      '<div><strong>事件链接：</strong>' + (link ? esc(link) : '（未填写）') + '</div>' +
      '<div><strong>相关人：</strong>' + (people ? esc(people) : '（未填写）') + '</div>' +
      '<div><strong>Ban 时间：</strong>' + (bannedAt ? esc(bannedAt) : '当前时间') + '</div>';
    $('confirmSummary').innerHTML = summary;
    $('confirmSubmitModal')._payload = { email: email, reason: reason, link: link, people: people, bannedAt: bannedAt };
    openModal('confirmSubmitModal');
  });

  $('confirmSubmitBtn').addEventListener('click', async function () {
    const p = $('confirmSubmitModal')._payload;
    if (!p) return;
    App.setLoading($('confirmSubmitBtn'), true, '提交中…');
    try {
      await adminApi('/api/v1/admin/add', {
        method: 'POST',
        body: { email: p.email, ban_reason: p.reason, event_link: p.link, event_related_people: p.people, banned_at: p.bannedAt }
      });
      App.toast('success', '已添加到黑名单');
      closeModal('confirmSubmitModal');
      closeModal('addModal');
      state.bl.page = 1;
      loadBlacklist();
    } catch (err) {
      App.toast('error', err.message);
    } finally {
      App.setLoading($('confirmSubmitBtn'), false);
    }
  });

  // ===== 删除 / 还原 =====
  function openDelete(id, email, mode) {
    state.delete = { id: id, email: email, mode: mode };
    $('deleteTitle').textContent = mode === 'soft' ? '移入回收站' : '永久删除';
    $('deleteText').innerHTML = mode === 'soft'
      ? '此操作将把 <strong>' + esc(email) + '</strong> 移入回收站，可随时还原。请输入邮箱以确认：'
      : '此操作将<strong>永久删除</strong> ' + esc(email) + '，且<strong>不可恢复</strong>。请输入邮箱以确认：';
    $('deleteConfirmInput').value = '';
    $('deleteConfirmBtn').disabled = true;
    openModal('deleteModal');
    setTimeout(function () { $('deleteConfirmInput').focus(); }, 50);
  }

  $('deleteConfirmInput').addEventListener('input', function () {
    $('deleteConfirmBtn').disabled = this.value.trim().toLowerCase() !== state.delete.email.toLowerCase();
  });

  $('deleteConfirmBtn').addEventListener('click', async function () {
    const d = state.delete;
    if (!d.id) return;
    const url = d.mode === 'soft'
      ? '/api/v1/admin/delete/' + d.id
      : '/api/v1/admin/permanent/' + d.id;
    App.setLoading($('deleteConfirmBtn'), true, '处理中…');
    try {
      await adminApi(url, { method: 'DELETE' });
      App.toast('success', d.mode === 'soft' ? '已移入回收站' : '已永久删除');
      closeModal('deleteModal');
      loadBlacklist();
    } catch (err) {
      App.toast('error', err.message);
    } finally {
      App.setLoading($('deleteConfirmBtn'), false);
    }
  });

  async function restoreItem(id) {
    try {
      await adminApi('/api/v1/admin/restore/' + id, { method: 'POST' });
      App.toast('success', '已还原');
      loadBlacklist();
    } catch (err) {
      App.toast('error', err.message);
    }
  }

  // ===== 公告 =====
  async function loadAnnouncement() {
    try {
      const data = await adminApi('/api/v1/admin/announcement');
      $('annContent').value = data.content || '';
      renderPreview();
    } catch (e) { /* 401 或其他已在 adminApi 处理 */ }
  }

  function renderPreview() {
    const val = $('annContent').value;
    const preview = $('annPreview');
    if (!val.trim()) {
      preview.innerHTML = '<div class="preview-empty">暂无内容</div>';
      return;
    }
    preview.innerHTML = App.renderMarkdown(val);
  }

  $('annContent').addEventListener('input', renderPreview);
  $('annTemplate').addEventListener('click', function () {
    $('annContent').value = '欢迎使用社区黑名单查询工具。\n\n- 请遵守社区规范，友好交流。\n- 被拉黑用户将无法参与社区活动。\n\n如有疑问，请联系管理员。\n';
    renderPreview();
  });
  $('annSave').addEventListener('click', async function () {
    const content = $('annContent').value;
    App.setLoading($('annSave'), true, '保存中…');
    try {
      await adminApi('/api/v1/admin/announcement', { method: 'PUT', body: { content: content } });
      App.toast('success', '公告已保存');
    } catch (err) {
      App.toast('error', err.message);
    } finally {
      App.setLoading($('annSave'), false);
    }
  });

  // ===== 审计日志 =====
  $('auditPagination').addEventListener('click', function (e) {
    const b = e.target.closest('[data-page]');
    if (!b) return;
    const p = Number(b.getAttribute('data-page'));
    if (!p || p === state.audit.page) return;
    state.audit.page = p;
    loadAudit();
  });
  $('auditFilter').addEventListener('change', function () {
    state.audit.action = this.value;
    state.audit.page = 1;
    loadAudit();
  });

  function loadAudit() {
    const url = '/api/v1/admin/audit-logs?' + qs({ action: state.audit.action, page: state.audit.page, page_size: state.audit.pageSize });
    adminApi(url).then(function (data) {
      renderAudit(data.list, data.total);
    }).catch(function (e) {
      App.toast('error', e.message);
      renderAudit([], 0);
    });
  }

  function tagClass(action) {
    const known = { login: 'tag-login', logout: 'tag-logout', add: 'tag-add', delete: 'tag-delete', restore: 'tag-restore', edit_announcement: 'tag-edit_announcement' };
    return known[action] || 'tag-logout';
  }

  function renderAudit(list, total) {
    const head = '<thead><tr><th>时间</th><th>操作人</th><th>操作类型</th><th>操作目标</th><th>IP</th></tr></thead>';
    const rows = list.map(function (item) {
      return '<tr>' +
        '<td>' + esc(item.created_at) + '</td>' +
        '<td>' + esc(item.user) + '</td>' +
        '<td><span class="tag ' + tagClass(item.action) + '">' + esc(item.action) + '</span></td>' +
        '<td>' + esc(item.target || '—') + '</td>' +
        '<td>' + esc(item.ip) + '</td>' +
      '</tr>';
    }).join('');
    $('auditTableWrap').innerHTML = '<table class="data-table">' + head + '<tbody>' + rows + '</tbody></table>';

    $('auditCardList').innerHTML = list.map(function (item) {
      return '<div class="list-item">' +
        '<div class="li-field"><span class="li-label">时间</span><span class="li-value">' + esc(item.created_at) + '</span></div>' +
        '<div class="li-field"><span class="li-label">操作人</span><span class="li-value">' + esc(item.user) + '</span></div>' +
        '<div class="li-field"><span class="li-label">类型</span><span class="li-value"><span class="tag ' + tagClass(item.action) + '">' + esc(item.action) + '</span></span></div>' +
        '<div class="li-field"><span class="li-label">目标</span><span class="li-value">' + esc(item.target || '—') + '</span></div>' +
        '<div class="li-field"><span class="li-label">IP</span><span class="li-value">' + esc(item.ip) + '</span></div>' +
      '</div>';
    }).join('');

    $('auditEmpty').style.display = list.length === 0 ? '' : 'none';
    $('auditPagination').innerHTML = paginationHTML(state.audit.page, total, state.audit.pageSize);
  }

  // ===== 分页通用 =====
  function pageWindow(page, totalPages) {
    const set = { 1: true, [totalPages]: true };
    for (let i = Math.max(1, page - 2); i <= Math.min(totalPages, page + 2); i++) set[i] = true;
    const arr = Object.keys(set).map(Number).sort(function (a, b) { return a - b; });
    const out = [];
    let prev = 0;
    arr.forEach(function (p) {
      if (prev && p - prev > 1) out.push('…');
      out.push(p);
      prev = p;
    });
    return out;
  }

  function paginationHTML(page, totalCount, pageSize) {
    const totalPages = Math.max(1, Math.ceil(totalCount / pageSize));
    let html = '<span class="page-total">共 ' + totalCount + ' 条</span>';
    if (totalPages > 1) {
      const pages = pageWindow(page, totalPages);
      html += '<div class="page-btns">' +
        '<button class="page-btn" data-page="' + (page - 1) + '"' + (page <= 1 ? ' disabled' : '') + '>上一页</button>';
      pages.forEach(function (p) {
        if (p === '…') { html += '<span style="color:#94A3B8;padding:0 4px;">…</span>'; }
        else { html += '<button class="page-btn' + (p === page ? ' active' : '') + '" data-page="' + p + '">' + p + '</button>'; }
      });
      html += '<button class="page-btn" data-page="' + (page + 1) + '"' + (page >= totalPages ? ' disabled' : '') + '>下一页</button></div>';
    }
    return html;
  }

  // ===== 弹窗关闭绑定 =====
  document.querySelectorAll('.modal-overlay').forEach(function (overlay) {
    overlay.addEventListener('click', function (e) {
      if (e.target === overlay) closeModal(overlay.id);
    });
  });
  document.querySelectorAll('[data-close]').forEach(function (btn) {
    btn.addEventListener('click', function () { closeModal(btn.getAttribute('data-close')); });
  });

  // ===== 初始化 =====
  (async function init() {
    try {
      const data = await App.api('/api/v1/admin/status');
      if (data.logged_in) showDashboard();
      else showLogin();
    } catch (e) {
      showLogin();
    }
  })();
})();