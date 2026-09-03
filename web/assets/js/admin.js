(function () {
  'use strict';
  const App = window.App;
  const $ = function (id) { return document.getElementById(id); };
  const esc = App.escapeHTML;

  const EMAIL_RE = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/;
  const URL_RE = /^https?:\/\//i;

  // 审计操作：英文编码 → 中文标签
  const ACTION_LABELS = {
    login: '登录',
    logout: '退出登录',
    add: '标记为不可信',
    update: '修改不可信邮箱',
    delete: '删除不可信邮箱',
    restore: '还原不可信邮箱',
    edit_announcement: '编辑公告',
    edit_settings: '修改设置',
    change_password: '修改密码',
    create_admin: '创建管理员',
    reset_password: '重置密码',
    reset_totp: '重置TOTP密钥',
    delete_admin: '删除管理员',
    approve_submission: '通过申请',
    reject_submission: '驳回申请'
  };
  const ACTION_TAGS = {
    login: 'tag-login',
    logout: 'tag-logout',
    add: 'tag-add',
    update: 'tag-update',
    delete: 'tag-delete',
    restore: 'tag-restore',
    edit_announcement: 'tag-edit_announcement',
    edit_settings: 'tag-edit_settings',
    change_password: 'tag-change_password',
    create_admin: 'tag-edit_settings',
    reset_password: 'tag-restore',
    reset_totp: 'tag-login',
    delete_admin: 'tag-delete',
    approve_submission: 'tag-login',
    reject_submission: 'tag-delete'
  };

  const state = {
    bl: { mode: 'list', query: '', page: 1, pageSize: 10 },
    audit: { action: '', page: 1, pageSize: 20 },
    delete: { id: null, email: '', mode: 'soft' },
    editing: null,
    currentUser: '',
    isSuper: false,
    users: []
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
    loadReviewBadge();
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
    if (name === 'settings') loadSettings();
    if (name === 'users') loadUsers();
    if (name === 'review') loadReview();
  }

  // ===== 更新管理员相关 UI（显示/隐藏用户管理 Tab、收件箱配置、显示用户名） =====
  function updateAdminUI() {
    $('usersTab').style.display = state.isSuper ? '' : 'none';
    $('inboxSettingsCard').style.display = state.isSuper ? '' : 'none';
    const adminNameEl = document.querySelector('.admin-user');
    if (adminNameEl) {
      adminNameEl.innerHTML = '<span class="admin-user-dot"></span>' + esc(state.currentUser || 'admin') +
        (state.isSuper ? ' <span class="user-role-tag user-role-super" style="margin-left:6px;">超级管理员</span>' : '');
    }
  }

  // ===== 弹窗 =====
  function openModal(id) { $(id).classList.add('show'); }
  function closeModal(id) { $(id).classList.remove('show'); }

  function setLoginError(msg) {
    const el = $('loginError');
    el.textContent = msg;
    el.classList.toggle('show', !!msg);
  }

  // ===== 登录 =====
  $('loginForm').addEventListener('submit', async function (e) {
    e.preventDefault();
    const username = $('loginUsername').value.trim();
    const password = $('loginPassword').value;
    const totp = $('loginTotp').value.trim();
    if (!username) { setLoginError('请输入用户名'); return; }
    if (!password) { setLoginError('请输入密码'); return; }
    if (!/^\d{6}$/.test(totp)) { setLoginError('请输入 6 位动态口令'); return; }

    setLoginError('');
    App.setLoading($('loginBtn'), true, '登录中…');
    try {
      const data = await App.api('/api/v1/admin/login', { method: 'POST', body: { username: username, password: password, totp: totp } });
      state.currentUser = data.username || username;
      state.isSuper = !!data.is_super;
      updateAdminUI();
      showDashboard();
    } catch (err) {
      setLoginError(err.message);
    } finally {
      App.setLoading($('loginBtn'), false);
    }
  });

  // ===== 退出登录 =====
  $('logoutBtn').addEventListener('click', async function () {
    try { await App.api('/api/v1/admin/logout', { method: 'POST' }); } catch (e) { /* noop */ }
    window.location.href = '/';
  });

  // ===== 返回首页（不退出登录） =====
  $('homeBtn').addEventListener('click', function () {
    window.location.href = '/';
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
    if (action === 'edit') openEdit(id);
    else if (action === 'delete') openDelete(id, email, 'soft');
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
    state.editing = null;
    $('addForm').reset();
    updateReasonCount();
    $('addModalTitle').textContent = '添加不可信邮箱';
    $('addNextBtn').textContent = '下一步';
    openModal('addModal');
    setTimeout(function () { $('addEmail').focus(); }, 60);
  });
  $('trashBtn').addEventListener('click', function () {
    state.bl.mode = state.bl.mode === 'trash' ? 'list' : 'trash';
    state.bl.page = 1;
    $('trashBtn').textContent = state.bl.mode === 'trash' ? '返回列表' : '回收站';
    loadBlacklist();
  });

  function loadBlacklist() {
    $('blTableWrap').innerHTML = '<div class="skeleton" style="height:120px;margin:16px;"></div>';
    const url = '/api/v1/admin/list?' + qs({
      q: state.bl.query, deleted: state.bl.mode === 'trash' ? 1 : 0,
      page: state.bl.page, page_size: state.bl.pageSize
    });
    adminApi(url).then(function (data) {
      currentList = data.list || [];
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
    $('blEmptyText').textContent = isTrash ? '回收站为空' : '暂无不可信邮箱记录';
    $('blPagination').innerHTML = paginationHTML(state.bl.page, total, state.bl.pageSize);
  }

  function actionButtons(item, isTrash) {
    if (isTrash) {
      return '<button class="btn btn-secondary btn-sm" data-action="restore" data-id="' + item.id + '" data-email="' + esc(item.email) + '">还原</button>' +
        '<button class="btn btn-danger btn-sm" data-action="permanent" data-id="' + item.id + '" data-email="' + esc(item.email) + '">永久删除</button>';
    }
    return '<button class="btn btn-secondary btn-sm" data-action="edit" data-id="' + item.id + '" data-email="' + esc(item.email) + '">编辑</button>' +
      '<button class="btn btn-danger btn-sm" data-action="delete" data-id="' + item.id + '" data-email="' + esc(item.email) + '">删除</button>';
  }

  function tableHTML(list, isTrash) {
    const head = '<thead><tr><th>邮箱</th><th>标记原因</th><th>相关人</th><th>标记时间</th><th style="text-align:right;">操作</th></tr></thead>';
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
        '<div class="li-field"><span class="li-label">标记时间</span><span class="li-value">' + esc(item.banned_at || '—') + '</span></div>' +
        '<div class="li-actions">' + actionButtons(item, isTrash) + '</div>' +
        '</div>';
    }).join('');
  }

  // ===== 标记原因字符计数 =====
  function updateReasonCount() {
    const len = App.countChars($('addReason').value);
    $('reasonCount').textContent = len;
    $('reasonCounter').classList.toggle('warn', len > 450);
  }
  $('addReason').addEventListener('input', updateReasonCount);

  // ===== 事件链接自动补全 https =====
  function normalizeLink() {
    const input = $('addLink');
    const v = input.value.trim();
    if (v && !URL_RE.test(v)) input.value = 'https://' + v;
  }
  $('addLink').addEventListener('blur', normalizeLink);

  // ===== 编辑流程 =====
  function toDatetimeLocal(s) {
    if (!s) return '';
    return s.replace(' ', 'T').slice(0, 16);
  }

  function openEdit(id) {
    const item = findItemById(id);
    if (!item) { App.toast('error', '未找到该记录，请刷新列表'); return; }
    state.editing = item;
    $('addEmail').value = item.email;
    $('addReason').value = item.ban_reason || '';
    $('addLink').value = item.event_link || '';
    $('addPeople').value = item.event_related_people || '';
    $('addBannedAt').value = toDatetimeLocal(item.banned_at);
    updateReasonCount();
    $('addModalTitle').textContent = '编辑不可信邮箱';
    $('addNextBtn').textContent = '保存修改';
    openModal('addModal');
    setTimeout(function () { $('addEmail').focus(); }, 60);
  }

  let currentList = [];
  function findItemById(id) {
    return currentList.find(function (x) { return x.id === id; }) || null;
  }

  // ===== 申请审核 =====
  const reviewState = { status: 'pending', type: '', page: 1, pageSize: 10, total: 0 };

  async function loadReview() {
    try {
      var params = [];
      if (reviewState.status) params.push('status=' + encodeURIComponent(reviewState.status));
      if (reviewState.type) params.push('type=' + encodeURIComponent(reviewState.type));
      params.push('page=' + reviewState.page);
      params.push('page_size=' + reviewState.pageSize);
      const data = await adminApi('/api/v1/admin/submissions?' + params.join('&'), { method: 'GET' });
      reviewState.total = data.total || 0;
      renderReview(data.list || []);
      renderReviewPagination();
    } catch (e) {
      App.toast('error', '加载失败：' + e.message);
    }
  }

  function escapeLink(url) {
    var s = String(url || '').trim();
    if (!s) return '—';
    if (!/^https?:\/\//i.test(s)) return esc(s);
    return '<a href="' + esc(s) + '" target="_blank" rel="noopener noreferrer">查看证据</a>';
  }

  function truncate(s, n) {
    return s.length > n ? s.slice(0, n) + '…' : s;
  }

  function reviewMeta(s) {
    var typeTag = s.type === 'appeal'
      ? '<span class="user-role-tag user-role-super">申诉</span>'
      : '<span class="user-role-tag user-role-normal">举报</span>';
    var statusClass = 'sub-status-tag sub-status-' + s.status;
    var statusText = { pending: '待审核', approved: '已通过', rejected: '已驳回' }[s.status] || s.status;
    var reason = s.type === 'appeal' ? (s.appeal_reason || '') : (s.ban_reason || '');
    var evidence = s.type === 'appeal' ? (s.appeal_evidence || '') : (s.event_link || '');
    var canReview = s.status === 'pending';
    return { typeTag: typeTag, statusClass: statusClass, statusText: statusText, reason: reason, evidence: evidence, canReview: canReview };
  }

  function reviewActionButtons(s, canReview) {
    return canReview
      ? '<button class="btn btn-success btn-sm" data-review-action="approve" data-id="' + s.id + '">通过</button>' +
      '<button class="btn btn-danger btn-sm" data-review-action="reject" data-id="' + s.id + '">驳回</button>'
      : '<button class="btn btn-secondary btn-sm" data-review-action="view" data-id="' + s.id + '">已处理</button>';
  }

  function reviewRowHTML(s) {
    var m = reviewMeta(s);
    var reasonShort = truncate(m.reason, 40) || '—';
    var mainRow =
      '<tr class="review-row" data-id="' + s.id + '">' +
      '<td><button class="row-toggle" type="button" data-toggle="' + s.id + '" aria-expanded="false">' +
        '<svg class="row-toggle-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 6 15 12 9 18"></polyline></svg>' +
        '</button></td>' +
      '<td>' + esc(s.id) + '</td>' +
      '<td>' + m.typeTag + '</td>' +
      '<td class="cell-email">' + esc(s.email) + '</td>' +
      '<td class="cell-reason" title="' + esc(m.reason) + '">' + esc(reasonShort) + '</td>' +
      '<td>' + escapeLink(m.evidence) + '</td>' +
      '<td><span class="' + m.statusClass + '">' + m.statusText + '</span></td>' +
      '<td>' + esc(s.created_at || '—') + '</td>' +
      '<td class="cell-actions">' + reviewActionButtons(s, m.canReview) + '</td>' +
      '</tr>';
    return mainRow + renderReviewDetail(s);
  }

  function reviewCardHTML(s) {
    var m = reviewMeta(s);
    var rejectNote = (s.status === 'rejected' && s.reject_reason)
      ? '<div class="li-field"><span class="li-label">驳回原因</span><span class="li-value" style="color:var(--danger);">' + esc(s.reject_reason) + '</span></div>'
      : '';
    return '<div class="list-item">' +
      '<div class="li-field"><span class="li-label">类型</span><span class="li-value">' + m.typeTag + ' <span class="' + m.statusClass + '">' + m.statusText + '</span></span></div>' +
      '<div class="li-field"><span class="li-label">邮箱</span><span class="li-value">' + esc(s.email) + '</span></div>' +
      '<div class="li-field"><span class="li-label">原因/理由</span><span class="li-value">' + esc(m.reason || '—') + '</span></div>' +
      '<div class="li-field"><span class="li-label">证据链接</span><span class="li-value">' + escapeLink(m.evidence) + '</span></div>' +
      rejectNote +
      '<div class="li-field"><span class="li-label">提交时间</span><span class="li-value">' + esc(s.created_at || '—') + '</span></div>' +
      '<div class="li-actions">' + reviewActionButtons(s, m.canReview) + '</div>' +
      '</div>';
  }

  function renderReview(list) {
    var tbody = $('reviewTableBody');
    var cardList = $('reviewCardList');
    var empty = $('reviewEmpty');
    if (!list.length) {
      tbody.innerHTML = '';
      cardList.innerHTML = '';
      empty.style.display = 'block';
      return;
    }
    empty.style.display = 'none';
    tbody.innerHTML = list.map(reviewRowHTML).join('');
    cardList.innerHTML = list.map(reviewCardHTML).join('');
  }

  function renderReviewDetail(s) {
    var reason = s.type === 'appeal' ? (s.appeal_reason || '') : (s.ban_reason || '');
    var evidence = s.type === 'appeal' ? (s.appeal_evidence || '') : (s.event_link || '');
    var people = s.event_related_people || '';

    var rows = '';
    rows += '<div class="rd-item rd-item-full"><div class="rd-label">' + (s.type === 'appeal' ? '申诉理由' : '标记原因') + '</div><div class="rd-value">' + esc(reason) + '</div></div>';
    rows += '<div class="rd-item rd-item-full"><div class="rd-label">证据链接</div><div class="rd-value">' + escapeLink(evidence) + '</div></div>';
    if (people) {
      rows += '<div class="rd-item rd-item-full"><div class="rd-label">相关人员</div><div class="rd-value">' + esc(people) + '</div></div>';
    }
    if (s.status === 'rejected' && s.reject_reason) {
      rows += '<div class="rd-item rd-item-full rd-item-reject"><div class="rd-label">驳回原因</div><div class="rd-value">' + esc(s.reject_reason) + '</div></div>';
    }
    rows += '<div class="rd-item"><div class="rd-label">查询码</div><div class="rd-value rd-mono">' + esc(s.query_code || '—') + '</div></div>';

    return '<tr class="review-detail" id="reviewDetail-' + s.id + '" style="display:none;">' +
      '<td colspan="9"><div class="review-detail-grid">' + rows + '</div></td>' +
      '</tr>';
  }

  function renderReviewPagination() {
    var el = $('reviewPagination');
    var total = reviewState.total;
    var page = reviewState.page;
    var pageSize = reviewState.pageSize;
    var totalPages = Math.max(1, Math.ceil(total / pageSize));
    if (totalPages <= 1 && total === 0) { el.innerHTML = ''; return; }
    var html = '<span class="page-total">共 ' + total + ' 条</span>';
    if (totalPages > 1) {
      html += '<div class="page-btns">' +
        '<button class="page-btn" data-review-page="' + (page - 1) + '"' + (page <= 1 ? ' disabled' : '') + '>上一页</button>';
      var pages = pageWindow(page, totalPages);
      pages.forEach(function (p) {
        if (p === '…') { html += '<span style="color:#94A3B8;padding:0 4px;">…</span>'; }
        else { html += '<button class="page-btn' + (p === page ? ' active' : '') + '" data-review-page="' + p + '">' + p + '</button>'; }
      });
      html += '<button class="page-btn" data-review-page="' + (page + 1) + '"' + (page >= totalPages ? ' disabled' : '') + '>下一页</button>';
      html += '</div>';
    }
    el.innerHTML = html;
  }

  $('reviewFilter').addEventListener('change', function () {
    reviewState.status = this.value;
    reviewState.page = 1;
    loadReview();
  });
  $('reviewTypeFilter').addEventListener('change', function () {
    reviewState.type = this.value;
    reviewState.page = 1;
    loadReview();
  });
  $('reviewPagination').addEventListener('click', function (e) {
    var btn = e.target.closest('[data-review-page]');
    if (!btn || btn.disabled) return;
    var p = parseInt(btn.getAttribute('data-review-page'), 10);
    if (p > 0) { reviewState.page = p; loadReview(); }
  });

  // 审核操作（通过/驳回）
  var currentReviewId = null;

  $('reviewTableBody').addEventListener('click', function (e) {
    var toggle = e.target.closest('[data-toggle]');
    if (toggle) {
      var tid = toggle.getAttribute('data-toggle');
      var detail = $('reviewDetail-' + tid);
      if (detail) {
        var shown = detail.style.display !== 'none';
        detail.style.display = shown ? 'none' : 'table-row';
        toggle.classList.toggle('open', !shown);
        toggle.setAttribute('aria-expanded', shown ? 'false' : 'true');
      }
      return;
    }
    var btn = e.target.closest('[data-review-action]');
    if (!btn) return;
    var action = btn.getAttribute('data-review-action');
    var id = btn.getAttribute('data-id');
    if (action === 'approve') approveSubmission(id);
    else if (action === 'reject') openRejectModal(id);
  });

  // 移动端卡片视图：通过/驳回/查看
  $('reviewCardList').addEventListener('click', function (e) {
    var btn = e.target.closest('[data-review-action]');
    if (!btn) return;
    var action = btn.getAttribute('data-review-action');
    var id = btn.getAttribute('data-id');
    if (action === 'approve') approveSubmission(id);
    else if (action === 'reject') openRejectModal(id);
  });

  async function approveSubmission(id) {
    if (!confirm('确定通过该申请吗？举报通过将加入不可信邮箱列表，申诉通过将从列表中移除。')) return;
    try {
      await adminApi('/api/v1/admin/submissions/' + id + '/approve', { method: 'POST' });
      App.toast('success', '已通过');
      loadReview();
      loadReviewBadge();
    } catch (e) {
      App.toast('error', '操作失败：' + e.message);
    }
  }

  function openRejectModal(id) {
    currentReviewId = id;
    $('rejectSubReason').value = '';
    $('rejectReasonCount').textContent = '0';
    $('rejectSubConfirmBtn').disabled = true;
    openModal('rejectSubModal');
  }

  $('rejectSubReason').addEventListener('input', function () {
    $('rejectReasonCount').textContent = App.countChars(this.value);
    $('rejectSubConfirmBtn').disabled = !this.value.trim();
  });

  $('rejectSubConfirmBtn').addEventListener('click', async function () {
    if (!currentReviewId) return;
    var reason = $('rejectSubReason').value.trim();
    if (!reason) { App.toast('error', '请填写驳回原因'); return; }
    App.setLoading($('rejectSubConfirmBtn'), true, '提交中…');
    try {
      await adminApi('/api/v1/admin/submissions/' + currentReviewId + '/reject', {
        method: 'POST', body: { reason: reason }
      });
      closeModal('rejectSubModal');
      App.toast('success', '已驳回');
      loadReview();
      loadReviewBadge();
    } catch (e) {
      App.toast('error', '操作失败：' + e.message);
    } finally {
      App.setLoading($('rejectSubConfirmBtn'), false);
    }
  });

  // 待审核数量徽章
  async function loadReviewBadge() {
    try {
      const data = await adminApi('/api/v1/admin/submissions?status=pending&page_size=1', { method: 'GET' });
      var badge = $('reviewBadge');
      var count = data.total || 0;
      if (count > 0) {
        badge.textContent = count > 99 ? '99+' : count;
        badge.style.display = '';
      } else {
        badge.style.display = 'none';
      }
    } catch (e) { /* 静默忽略 */ }
  }

  // ===== 添加 / 编辑 提交 =====
  $('addNextBtn').addEventListener('click', function () {
    const email = $('addEmail').value.trim().toLowerCase();
    const reason = $('addReason').value.trim();
    const link = $('addLink').value.trim();
    const people = $('addPeople').value.trim();
    const bannedAt = $('addBannedAt').value.trim();

    if (!EMAIL_RE.test(email)) { App.toast('warning', '请输入有效的邮箱地址'); return; }
    if (!link.trim()) { App.toast('warning', '事件链接为必填项，请提供有效的证据链接'); return; }
    var linkErr = App.validateLink(link);
    if (linkErr) { App.toast('warning', linkErr); return; }
    if (App.countChars(reason) > 500) { App.toast('warning', '标记原因过长，最多 500 字'); return; }

    const isEdit = !!state.editing;
    $('confirmTitle').textContent = isEdit ? '确认修改' : '确认提交';
    $('confirmSubmitBtn').textContent = isEdit ? '确认修改' : '确认提交';
    const summary = '<div><strong>邮箱：</strong>' + esc(email) + '</div>' +
      '<div><strong>标记原因：</strong>' + (reason ? esc(reason) : '（未填写）') + '</div>' +
      '<div><strong>事件链接：</strong>' + (link ? esc(link) : '（未填写）') + '</div>' +
      '<div><strong>相关人：</strong>' + (people ? esc(people) : '（未填写）') + '</div>' +
      '<div><strong>标记时间：</strong>' + (bannedAt ? esc(bannedAt) : '当前时间') + '</div>';
    $('confirmSummary').innerHTML = summary;
    $('confirmSubmitModal')._payload = { email: email, reason: reason, link: link, people: people, bannedAt: bannedAt };
    openModal('confirmSubmitModal');
  });

  $('confirmSubmitBtn').addEventListener('click', async function () {
    const p = $('confirmSubmitModal')._payload;
    if (!p) return;
    const isEdit = !!state.editing;
    const url = isEdit ? '/api/v1/admin/update/' + state.editing.id : '/api/v1/admin/add';
    App.setLoading($('confirmSubmitBtn'), true, '提交中…');
    try {
      await adminApi(url, {
        method: isEdit ? 'PUT' : 'POST',
        body: { email: p.email, ban_reason: p.reason, event_link: p.link, event_related_people: p.people, banned_at: p.bannedAt }
      });
      App.toast('success', isEdit ? '修改已保存' : '已标记为不可信');
      closeModal('confirmSubmitModal');
      closeModal('addModal');
      state.editing = null;
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
    $('annContent').value = '欢迎使用社区不可信邮箱查询工具。\n\n- 请遵守社区规范，友好合格交流。\n- 被标记为不可信的用户将无法参与社区活动。\n\n如有疑问，请联系管理员。\n';
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

  function actionLabel(action) {
    return ACTION_LABELS[action] || action;
  }
  function tagClass(action) {
    return ACTION_TAGS[action] || 'tag-logout';
  }

  function renderAudit(list, total) {
    const head = '<thead><tr><th>时间</th><th>操作人</th><th>操作类型</th><th>操作目标</th><th>IP</th></tr></thead>';
    const rows = list.map(function (item) {
      return '<tr>' +
        '<td>' + esc(item.created_at) + '</td>' +
        '<td>' + esc(item.user) + '</td>' +
        '<td><span class="tag ' + tagClass(item.action) + '">' + esc(actionLabel(item.action)) + '</span></td>' +
        '<td>' + esc(item.target || '—') + '</td>' +
        '<td>' + esc(item.ip) + '</td>' +
        '</tr>';
    }).join('');
    $('auditTableWrap').innerHTML = '<table class="data-table">' + head + '<tbody>' + rows + '</tbody></table>';

    $('auditCardList').innerHTML = list.map(function (item) {
      return '<div class="list-item">' +
        '<div class="li-field"><span class="li-label">时间</span><span class="li-value">' + esc(item.created_at) + '</span></div>' +
        '<div class="li-field"><span class="li-label">操作人</span><span class="li-value">' + esc(item.user) + '</span></div>' +
        '<div class="li-field"><span class="li-label">类型</span><span class="li-value"><span class="tag ' + tagClass(item.action) + '">' + esc(actionLabel(item.action)) + '</span></span></div>' +
        '<div class="li-field"><span class="li-label">目标</span><span class="li-value">' + esc(item.target || '—') + '</span></div>' +
        '<div class="li-field"><span class="li-label">IP</span><span class="li-value">' + esc(item.ip) + '</span></div>' +
        '</div>';
    }).join('');

    $('auditEmpty').style.display = list.length === 0 ? '' : 'none';
    $('auditPagination').innerHTML = paginationHTML(state.audit.page, total, state.audit.pageSize);
  }

  // ===== 系统设置：收件箱 =====
  async function loadSettings() {
    try {
      const data = await adminApi('/api/v1/admin/settings');
      $('inboxEmail').value = data.inbox_email || '';
    } catch (e) { /* adminApi 已处理 */ }
  }

  $('inboxSaveBtn').addEventListener('click', async function () {
    const email = $('inboxEmail').value.trim();
    if (email && !EMAIL_RE.test(email)) { App.toast('warning', '请输入有效的收件箱邮箱地址'); return; }
    App.setLoading($('inboxSaveBtn'), true, '保存中…');
    try {
      await adminApi('/api/v1/admin/settings', { method: 'PUT', body: { inbox_email: email } });
      await loadSettings();
      App.toast('success', email ? '收件箱已保存' : '收件箱已清空，首页提报按钮将隐藏');
    } catch (err) {
      App.toast('error', err.message);
    } finally {
      App.setLoading($('inboxSaveBtn'), false);
    }
  });

  // ===== 系统设置：修改密码 =====
  $('pwdBtn').addEventListener('click', async function () {
    const oldPwd = $('pwdOld').value;
    const newPwd = $('pwdNew').value;
    const newPwd2 = $('pwdNew2').value;
    if (!oldPwd) { App.toast('warning', '请输入当前密码'); return; }
    if (newPwd.length < 8 || !/[a-z]/.test(newPwd) || !/[A-Z]/.test(newPwd) || !/\d/.test(newPwd)) {
      App.toast('warning', '新密码需至少 8 位，且同时包含大写字母、小写字母和数字');
      return;
    }
    if (newPwd !== newPwd2) { App.toast('warning', '两次输入的新密码不一致'); return; }

    App.setLoading($('pwdBtn'), true, '提交中…');
    try {
      await adminApi('/api/v1/admin/password', { method: 'POST', body: { old_password: oldPwd, new_password: newPwd } });
      App.toast('success', '密码已修改');
      $('pwdOld').value = '';
      $('pwdNew').value = '';
      $('pwdNew2').value = '';
    } catch (err) {
      App.toast('error', err.message);
    } finally {
      App.setLoading($('pwdBtn'), false);
    }
  });

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
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') {
      document.querySelectorAll('.modal-overlay.show').forEach(function (m) { m.classList.remove('show'); });
    }
  });

  // ===== 用户管理 =====
  async function loadUsers() {
    try {
      const data = await adminApi('/api/v1/admin/users', { method: 'GET' });
      state.users = data.list || [];
      renderUsers();
    } catch (e) {
      App.toast('error', '加载用户列表失败：' + e.message);
    }
  }

  function userRowHTML(u) {
    const roleTag = u.is_super
      ? '<span class="user-role-tag user-role-super">超级管理员</span>'
      : '<span class="user-role-tag user-role-normal">管理员</span>';
    const isSelf = u.username === state.currentUser;
    const actions =
      '<button class="btn btn-secondary btn-sm" data-action="view-totp" data-id="' + u.id + '" title="查看TOTP密钥">查看TOTP</button>' +
      '<button class="btn btn-secondary btn-sm" data-action="reset-pwd" data-id="' + u.id + '" data-username="' + esc(u.username) + '">重置密码</button>' +
      '<button class="btn btn-secondary btn-sm" data-action="reset-totp" data-id="' + u.id + '" data-username="' + esc(u.username) + '">重置TOTP</button>' +
      '<button class="btn btn-danger btn-sm" data-action="delete" data-id="' + u.id + '" data-username="' + esc(u.username) + '"' +
      (isSelf ? ' disabled' : '') + '>' + (isSelf ? '无法删除' : '删除') + '</button>';
    return {
      table: '<tr>' +
        '<td>' + esc(u.id) + '</td>' +
        '<td class="cell-email">' + esc(u.username) + (isSelf ? ' <span class="user-self-hint">(当前)</span>' : '') + '</td>' +
        '<td>' + roleTag + '</td>' +
        '<td>' + esc(u.created_by || '—') + '</td>' +
        '<td>' + esc(u.created_at || '—') + '</td>' +
        '<td class="users-actions">' + actions + '</td>' +
        '</tr>',
      card: '<div class="list-item">' +
        '<div class="li-field"><span class="li-label">用户名</span><span class="li-value">' + esc(u.username) + ' ' + roleTag + (isSelf ? ' <span class="user-self-hint">(当前)</span>' : '') + '</span></div>' +
        '<div class="li-field"><span class="li-label">角色</span><span class="li-value">' + (u.is_super ? '超级管理员' : '管理员') + '</span></div>' +
        '<div class="li-field"><span class="li-label">创建人</span><span class="li-value">' + esc(u.created_by || '—') + '</span></div>' +
        '<div class="li-field"><span class="li-label">创建时间</span><span class="li-value">' + esc(u.created_at || '—') + '</span></div>' +
        '<div class="li-actions">' + actions + '</div>' +
        '</div>'
    };
  }

  function renderUsers() {
    const wrap = $('usersTableWrap');
    const cardList = $('usersCardList');
    const empty = $('usersEmpty');
    const list = state.users;

    if (!list.length) {
      if (wrap) wrap.innerHTML = '';
      if (cardList) cardList.innerHTML = '';
      if (empty) empty.style.display = 'block';
      return;
    }
    if (empty) empty.style.display = 'none';

    if (wrap) {
      const head = '<thead><tr><th style="width:50px;">ID</th><th>用户名</th><th>角色</th><th>创建人</th><th>创建时间</th><th style="text-align:right;">操作</th></tr></thead>';
      const rows = list.map(function (u) { return userRowHTML(u).table; }).join('');
      wrap.innerHTML = '<table class="data-table">' + head + '<tbody>' + rows + '</tbody></table>';
    }
    if (cardList) {
      cardList.innerHTML = list.map(function (u) { return userRowHTML(u).card; }).join('');
    }
  }

  // 查看 TOTP 密钥（随时可点击查看）
  async function viewTOTP(userId) {
    try {
      const data = await adminApi('/api/v1/admin/users/' + userId + '/totp', { method: 'GET' });
      const secret = data.totp_secret || '';
      const setupUrl = data.totp_setup_url || '';
      // 用一个简单的 prompt 显示密钥
      showTOTPSecretModal(secret, setupUrl);
    } catch (e) {
      App.toast('error', '查看失败：' + e.message);
    }
  }

  // 使用一个通用的凭证显示弹窗，复用 resetTOTPModal
  function showTOTPSecretModal(secret, setupUrl) {
    $('resetTOTPText').textContent = secret;
    $('resetTOTPSetupLink').href = setupUrl;
    // 隐藏通知栏（因为这不是重置操作）
    var notice = document.querySelector('#resetTOTPModal .credential-notice');
    if (notice) notice.style.display = 'none';
    var title = document.querySelector('#resetTOTPModal .modal-title');
    if (title) title.textContent = 'TOTP 密钥';
    openModal('resetTOTPModal');
  }

  // 创建管理员
  $('addUserBtn').addEventListener('click', function () {
    $('newUsername').value = '';
    $('newUserIsSuper').checked = false;
    openModal('addUserModal');
    setTimeout(function () { $('newUsername').focus(); }, 50);
  });

  $('addUserConfirmBtn').addEventListener('click', async function () {
    const username = $('newUsername').value.trim();
    const isSuper = $('newUserIsSuper').checked;
    if (!username) { App.toast('error', '请输入用户名'); return; }
    if (!/^[a-zA-Z0-9_-]{3,32}$/.test(username)) {
      App.toast('error', '用户名 3-32 位，允许字母数字下划线减号'); return;
    }
    App.setLoading($('addUserConfirmBtn'), true, '创建中…');
    try {
      const data = await adminApi('/api/v1/admin/users', {
        method: 'POST',
        body: { username: username, is_super: isSuper }
      });
      closeModal('addUserModal');
      // 显示创建成功凭证
      $('createdUsername').textContent = data.username;
      $('createdPasswordText').textContent = data.password;
      $('createdTOTPText').textContent = data.totp_secret;
      $('createdSetupLink').href = data.totp_setup_url;
      openModal('userCreatedModal');
      loadUsers();
    } catch (e) {
      App.toast('error', '创建失败：' + e.message);
    } finally {
      App.setLoading($('addUserConfirmBtn'), false);
    }
  });

  // 重置密码
  async function resetPassword(userId, username) {
    if (!confirm('确定要重置管理员「' + username + '」的密码吗？\n新密码仅显示一次。')) return;
    try {
      const data = await adminApi('/api/v1/admin/users/' + userId + '/password', { method: 'POST' });
      $('resetPasswordText').textContent = data.password;
      openModal('resetPwdModal');
      loadUsers();
    } catch (e) {
      App.toast('error', '重置失败：' + e.message);
    }
  }

  // 重置 TOTP
  async function resetTOTP(userId, username) {
    if (!confirm('确定要重置管理员「' + username + '」的TOTP密钥吗？\n旧密钥立即失效，密钥可多次查看。')) return;
    try {
      const data = await adminApi('/api/v1/admin/users/' + userId + '/totp', { method: 'POST' });
      $('resetTOTPText').textContent = data.totp_secret;
      $('resetTOTPSetupLink').href = data.totp_setup_url;
      var notice = document.querySelector('#resetTOTPModal .credential-notice');
      if (notice) notice.style.display = '';
      var title = document.querySelector('#resetTOTPModal .modal-title');
      if (title) title.textContent = 'TOTP 密钥已重置';
      openModal('resetTOTPModal');
      loadUsers();
    } catch (e) {
      App.toast('error', '重置失败：' + e.message);
    }
  }

  // 删除管理员
  var deleteUserState = { id: null, username: '' };

  function openDeleteUser(userId, username) {
    deleteUserState.id = userId;
    deleteUserState.username = username;
    $('deleteUserText').textContent = username;
    $('deleteUserConfirmInput').value = '';
    $('deleteUserConfirmBtn').disabled = true;
    openModal('deleteUserModal');
  }

  $('deleteUserConfirmInput').addEventListener('input', function () {
    $('deleteUserConfirmBtn').disabled = this.value !== deleteUserState.username;
  });

  $('deleteUserConfirmBtn').addEventListener('click', async function () {
    if (!deleteUserState.id) return;
    App.setLoading($('deleteUserConfirmBtn'), true, '删除中…');
    try {
      await adminApi('/api/v1/admin/users/' + deleteUserState.id, { method: 'DELETE' });
      closeModal('deleteUserModal');
      App.toast('success', '管理员「' + deleteUserState.username + '」已删除');
      loadUsers();
    } catch (e) {
      App.toast('error', '删除失败：' + e.message);
    } finally {
      App.setLoading($('deleteUserConfirmBtn'), false);
    }
  });

  // 用户表格行内按钮事件委托
  $('usersTableWrap').addEventListener('click', function (e) {
    var btn = e.target.closest('button[data-action]');
    if (!btn) return;
    var id = btn.getAttribute('data-id');
    var username = btn.getAttribute('data-username') || '';
    var action = btn.getAttribute('data-action');
    if (action === 'view-totp') viewTOTP(id);
    else if (action === 'reset-pwd') resetPassword(id, username);
    else if (action === 'reset-totp') resetTOTP(id, username);
    else if (action === 'delete') openDeleteUser(id, username);
  });

  // 复制按钮的事件委托已统一放到 common.js 中处理。

  // ===== 初始化 =====
  (async function init() {
    try {
      const data = await App.api('/api/v1/admin/status');
      if (data.logged_in) {
        state.currentUser = data.username || 'admin';
        state.isSuper = !!data.is_super;
        updateAdminUI();
        showDashboard();
      } else {
        showLogin();
      }
    } catch (e) {
      showLogin();
    }
  })();
})();
