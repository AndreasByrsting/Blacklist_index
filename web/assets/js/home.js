(function () {
  'use strict';
  const App = window.App;
  const $ = function (id) { return document.getElementById(id); };
  const esc = App.escapeHTML;

  const EMAIL_RE = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/;

  // 国内常见邮箱 + 谷歌邮箱
  const EMAIL_SUFFIXES = [
    'gmail.com', 'qq.com', '163.com', '126.com', 'foxmail.com',
    'sina.com', 'sohu.com', 'aliyun.com', 'yeah.net', '139.com', '189.cn'
  ];

  // ===== Canvas 图片压缩 =====
  // 将图片缩放到 maxDim 以内，输出 WebP (0.8 quality) 保证文字清晰且体积小。
  async function compressImage(file, maxDim) {
    maxDim = maxDim || 1600;
    var bitmap = await createImageBitmap(file);
    var scale = Math.min(1, maxDim / Math.max(bitmap.width, bitmap.height));
    var w = Math.round(bitmap.width * scale);
    var h = Math.round(bitmap.height * scale);
    var canvas = document.createElement('canvas');
    canvas.width = w;
    canvas.height = h;
    var ctx = canvas.getContext('2d');
    ctx.drawImage(bitmap, 0, 0, w, h);
    bitmap.close && bitmap.close();
    var blob = await new Promise(function (resolve) {
      canvas.toBlob(resolve, 'image/webp', 0.8);
    });
    if (!blob) throw new Error('图片压缩失败');
    return blob;
  }

  // ===== 图片选择器 =====
  function initImagePicker(zoneId, inputId, gridId, hintId, imagesArr, config) {
    var zone = $(zoneId);
    var input = $(inputId);
    var grid = $(gridId);
    var hint = $(hintId);

    function updateHint() {
      var max = config.maxCount || 0;
      var label = config.required ? '证据图片 <span class="required">*</span>' : '证据图片';
      var labelText = config.required ? '证据图片 *' : '证据图片';
      var labelEl = zone.parentElement.querySelector('.form-label');
      if (labelEl) labelEl.innerHTML = label;
      if (hint) {
        var parts = [];
        parts.push(config.required ? '必填' : '可选');
        parts.push(max > 0 ? '最多 ' + max + ' 张' : '数量不限');
        hint.textContent = parts.join('，');
      }
    }
    updateHint();

    zone.addEventListener('click', function () { input.click(); });

    zone.addEventListener('dragover', function (e) {
      e.preventDefault();
      zone.classList.add('dragover');
    });
    zone.addEventListener('dragleave', function () { zone.classList.remove('dragover'); });
    zone.addEventListener('drop', function (e) {
      e.preventDefault();
      zone.classList.remove('dragover');
      handleFiles(e.dataTransfer.files);
    });

    input.addEventListener('change', function () {
      handleFiles(input.files);
      input.value = '';
    });

    async function handleFiles(fileList) {
      var files = Array.prototype.slice.call(fileList).filter(function (f) {
        return /^image\/(jpeg|png|webp)$/.test(f.type);
      });
      var max = config.maxCount || 0;
      var remaining = max > 0 ? max - imagesArr.length : files.length;
      if (remaining <= 0) { App.toast('warning', '已达到上传上限'); return; }
      if (files.length > remaining) {
        App.toast('warning', '最多上传 ' + max + ' 张，已添加前 ' + remaining + ' 张');
        files = files.slice(0, remaining);
      }

      for (var i = 0; i < files.length; i++) {
        try {
          var blob = await compressImage(files[i]);
          var previewUrl = URL.createObjectURL(blob);
          imagesArr.push({ blob: blob, previewUrl: previewUrl, name: files[i].name });
        } catch (e) {
          App.toast('error', files[i].name + ' 压缩失败');
        }
      }
      renderGrid();
    }

    function renderGrid() {
      grid.innerHTML = imagesArr.map(function (item, idx) {
        return '<div class="img-thumb">' +
          '<img src="' + item.previewUrl + '" alt="预览">' +
          '<button class="img-thumb-remove" type="button" data-idx="' + idx + '" title="删除">✕</button>' +
          '</div>';
      }).join('');

      grid.querySelectorAll('.img-thumb-remove').forEach(function (btn) {
        btn.addEventListener('click', function () {
          var idx = parseInt(btn.getAttribute('data-idx'), 10);
          URL.revokeObjectURL(imagesArr[idx].previewUrl);
          imagesArr.splice(idx, 1);
          renderGrid();
        });
      });
    }

    function clearImages() {
      imagesArr.forEach(function (item) { URL.revokeObjectURL(item.previewUrl); });
      imagesArr.length = 0;
      renderGrid();
    }

    return { clear: clearImages, updateConfig: function (c) { config = c; updateHint(); } };
  }

  var reportPicker, appealPicker;

  const emailInput = $('emailInput');
  const searchBtn = $('searchBtn');
  const resultArea = $('resultArea');
  const announcementCard = $('announcementCard');
  const announcementContent = $('announcementContent');

  let inboxEmail = '';
  let hasAnnouncement = false;
  let isAdminLoggedIn = false;
  let reportLinkConfig = { evidenceRequired: true, domains: [] };
  let appealLinkConfig = { evidenceRequired: true, domains: [] };
  let reportImageConfig = { required: false, maxCount: 3 };
  let appealImageConfig = { required: false, maxCount: 3 };
  let reportImages = [];
  let appealImages = [];
  let statsEnabled = false;
  let statsCurrent = 0;
  let statsTarget = 0;

  const ICON_LOGIN = '<svg class="entry-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2"></rect><path d="M7 11V7a5 5 0 0 1 10 0v4"></path></svg>';
  const ICON_BACK = '<svg class="entry-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="19" y1="12" x2="5" y2="12"></line><polyline points="12 19 5 12 12 5"></polyline></svg>';

  // ===== 登录 / 首页视图切换 =====
  function showLoginView(show) {
    $('searchCard').style.display = show ? 'none' : '';
    $('loginCard').style.display = show ? '' : 'none';
    $('announcementCard').style.display = (show || !hasAnnouncement) ? 'none' : '';
    updateStatsVisibility();
    $('adminEntryBtn').innerHTML = show
      ? ICON_BACK + '<span id="adminEntryLabel">返回首页</span>'
      : ICON_LOGIN + '<span id="adminEntryLabel">管理员登录</span>';
    if (show) setTimeout(function () { $('homeUsername').focus(); }, 80);
    else setTimeout(function () { emailInput.focus(); }, 80);
  }

  $('adminEntryBtn').addEventListener('click', function () {
    if (isAdminLoggedIn) {
      window.location.href = document.body.getAttribute('data-dashboard') || '/';
      return;
    }
    showLoginView($('loginCard').style.display === 'none');
  });
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape' && $('loginCard').style.display !== 'none') showLoginView(false);
  });

  function setHomeError(msg) {
    const el = $('homeLoginError');
    el.textContent = msg;
    el.classList.toggle('show', !!msg);
  }

  // ===== 登录 =====
  $('homeLoginForm').addEventListener('submit', async function (e) {
    e.preventDefault();
    const username = $('homeUsername').value.trim();
    const password = $('homePassword').value;
    const totp = $('homeTotp').value.trim();
    if (!username) { setHomeError('请输入用户名'); return; }
    if (!password) { setHomeError('请输入密码'); return; }
    if (!/^\d{6}$/.test(totp)) { setHomeError('请输入 6 位动态口令'); return; }

    setHomeError('');
    App.setLoading($('homeLoginBtn'), true, '登录中…');
    try {
      await App.api('/api/v1/admin/login', { method: 'POST', body: { username: username, password: password, totp: totp } });
      window.location.href = document.body.getAttribute('data-dashboard') || '/';
    } catch (err) {
      setHomeError(err.message);
    } finally {
      App.setLoading($('homeLoginBtn'), false);
    }
  });

  // ===== 邮箱后缀快速补全 =====
  const suffixBtn = $('suffixBtn');
  const suffixMenu = $('suffixMenu');

  EMAIL_SUFFIXES.forEach(function (s) {
    const item = document.createElement('button');
    item.type = 'button';
    item.className = 'suffix-item';
    item.textContent = '@' + s;
    item.addEventListener('click', function () { applySuffix(s); });
    suffixMenu.appendChild(item);
  });

  function setMenuOpen(open) {
    suffixMenu.classList.toggle('open', open);
    suffixBtn.classList.toggle('open', open);
    suffixBtn.setAttribute('aria-expanded', open ? 'true' : 'false');
  }

  function applySuffix(suffix) {
    let v = emailInput.value.trim();
    if (v.indexOf('@') !== -1) v = v.slice(0, v.indexOf('@'));
    emailInput.value = v ? v + '@' + suffix : '@' + suffix;
    setMenuOpen(false);
    emailInput.focus();
    if (!v) emailInput.setSelectionRange(0, 0);
  }

  suffixBtn.addEventListener('click', function (e) {
    e.stopPropagation();
    setMenuOpen(!suffixMenu.classList.contains('open'));
  });
  suffixMenu.addEventListener('click', function (e) { e.stopPropagation(); });
  document.addEventListener('click', function () { setMenuOpen(false); });
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') setMenuOpen(false);
  });

  // ===== 查询结果 =====
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
      '<div><div class="headline-text">该邮箱未被标记为不可信</div>' +
      '<div class="desc">该邮箱在社区中是基本可信的</div></div></div>' +
      '</div>'
    );
  }

  function similarState() {
    setResult(
      '<div class="result-state result--similar">' +
      '<div class="headline"><span class="status-icon">!</span>' +
      '<div><div class="headline-text">该邮箱未被标记，但存在同名账户风险</div>' +
      '<div class="desc">检测到与当前邮箱账户名相同的其他域名邮箱已被标记为不可信</div></div></div>' +
      '</div>'
    );
  }

  function peopleChipsHTML(d) {
    let list = Array.isArray(d.related_people_list) && d.related_people_list.length
      ? d.related_people_list
      : String(d.related_people || '').split(/[,，、]/).map(function (x) { return x.trim(); }).filter(Boolean);
    if (!list.length) return '';
    return '<div class="people-chips">' + list.map(function (name, i) {
      return '<span class="person-chip c' + (i % 3) + '" style="animation-delay:' + (i * 45) + 'ms">' + esc(name) + '</span>';
    }).join('') + '</div>';
  }

  function evidenceImagesHTML(images) {
    if (!Array.isArray(images) || !images.length) return '';
    var thumbs = images.map(function (img) {
      return '<button class="evidence-img" type="button" data-img-url="' + esc(img.url) + '">' +
        '<img src="' + esc(img.url) + '" alt="证据图片" loading="lazy">' +
        '</button>';
    }).join('');
    return '<div class="detail-row"><span class="detail-label">证据图片</span>' +
      '<div class="detail-value"><div class="evidence-img-grid">' + thumbs + '</div></div></div>';
  }

  function blockedState(d) {
    let detail = '';
    if (d.reason_html) {
      detail += '<div class="detail-row"><span class="detail-label">标记原因</span>' +
        '<div class="detail-value markdown-body">' + App.sanitizeHTML(d.reason_html) + '</div></div>';
    }
    if (d.event_link) {
      const url = String(d.event_link);
      const isSafe = /^https?:\/\//i.test(url);
      detail += '<div class="detail-row"><span class="detail-label">事件链接</span><div class="detail-value">' +
        (isSafe
          ? '<a href="' + esc(url) + '" target="_blank" rel="noopener noreferrer">' + esc(url) + '</a>'
          : esc(url)) +
        '</div></div>';
    }
    if (Array.isArray(d.images) && d.images.length) {
      detail += evidenceImagesHTML(d.images);
    }
    const chips = peopleChipsHTML(d);
    if (chips) {
      detail += '<div class="detail-row"><span class="detail-label">相关人</span>' +
        '<div class="detail-value">' + chips + '</div></div>';
    }
    if (d.banned_at) {
      detail += '<div class="detail-row"><span class="detail-label">标记时间</span>' +
        '<div class="detail-value">' + esc(d.banned_at) + '</div></div>';
    }
    setResult(
      '<div class="result-state result--blocked">' +
      '<div class="headline"><span class="status-icon">✕</span>' +
      '<div><div class="headline-text">该邮箱已被标记为不可信</div></div></div>' +
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
      else if (data.similar) similarState();
      else safeState();
      bumpStats();
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

  // ===== 首页统计（查询次数，超管可开关）=====
  function formatCount(n) {
    return n.toLocaleString('en-US');
  }

  function updateStatsVisibility() {
    var card = $('statsCard');
    if (!card) return;
    var isLogin = $('loginCard').style.display !== 'none';
    card.style.display = (statsEnabled && !isLogin) ? '' : 'none';
  }

  function animateCount(to) {
    var el = $('statsNumber');
    if (!el) return;
    var from = statsCurrent;
    var dur = 1100;
    var start = performance.now();
    function easeOutCubic(t) { return 1 - Math.pow(1 - t, 3); }
    function frame(now) {
      var t = (now - start) / dur;
      if (t < 0) t = 0;
      if (t > 1) t = 1;
      statsCurrent = Math.round(from + (to - from) * easeOutCubic(t));
      el.textContent = formatCount(statsCurrent);
      if (t < 1) {
        requestAnimationFrame(frame);
      } else {
        statsCurrent = to;
        el.textContent = formatCount(to);
        el.classList.remove('pop');
        void el.offsetWidth;
        el.classList.add('pop');
      }
    }
    requestAnimationFrame(frame);
  }

  function bumpStats() {
    if (!statsEnabled) return;
    statsTarget += 1;
    animateCount(statsTarget);
  }

  (async function loadStats() {
    try {
      const data = await App.api('/api/v1/stats');
      statsEnabled = !!data.enabled;
      if (statsEnabled) {
        statsCurrent = 0;
        statsTarget = parseInt(data.count, 10) || 0;
        $('statsCard').setAttribute('aria-hidden', 'false');
        updateStatsVisibility();
        animateCount(statsTarget);
      } else {
        updateStatsVisibility();
      }
    } catch (e) { /* 静默忽略统计加载失败 */ }
  })();

  // ===== 提交 & 申诉 & 查询进度 =====
  $('submitBlWrap').style.display = ''; // 始终显示提交入口

  // 初始化图片选择器
  reportPicker = initImagePicker('reportUploadZone', 'reportImageInput', 'reportImgGrid', 'reportImageHint', reportImages, reportImageConfig);
  appealPicker = initImagePicker('appealUploadZone', 'appealImageInput', 'appealImgGrid', 'appealImageHint', appealImages, appealImageConfig);

  // —— 弹窗通用 ——
  function openModal(id) { $(id).classList.add('show'); }
  function closeModal(id) { $(id).classList.remove('show'); }

  document.querySelectorAll('[data-close]').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var mid = btn.getAttribute('data-close');
      if (mid) closeModal(mid);
    });
  });

  function bindModalOverlay(id) {
    var m = $(id);
    if (!m) return;
    m.addEventListener('click', function (e) {
      if (e.target === m) closeModal(id);
    });
  }
  ['submitModal', 'querySubModal', 'submitSuccessModal', 'userDelayConfirmModal'].forEach(bindModalOverlay);

  // —— 上报弹窗选项卡切换 ——
  function switchSubTab(name) {
    $('tabReport').classList.toggle('active', name === 'report');
    $('tabAppeal').classList.toggle('active', name === 'appeal');
    $('panelReport').classList.toggle('active', name === 'report');
    $('panelAppeal').classList.toggle('active', name === 'appeal');
  }
  $('tabReport').addEventListener('click', function () { switchSubTab('report'); });
  $('tabAppeal').addEventListener('click', function () { switchSubTab('appeal'); });

  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') {
      document.querySelectorAll('.modal-overlay.show').forEach(function (m) {
        m.classList.remove('show');
      });
    }
  });

  // —— 邮箱上报（默认展示「提交不可信邮箱」）——
  $('submitBlBtn').addEventListener('click', function () {
    $('reportEmail').value = '';
    $('reportReason').value = '';
    $('reportLink').value = '';
    $('reportPeople').value = '';
    $('reportReasonCount').textContent = '0';
    reportPicker.clear();
    switchSubTab('report');
    openModal('submitModal');
  });

  $('reportReason').addEventListener('input', function () {
    $('reportReasonCount').textContent = App.countChars(this.value);
  });

  $('reportSubmitBtn').addEventListener('click', function () {
    var email = $('reportEmail').value.trim().toLowerCase();
    var reason = $('reportReason').value.trim();
    var link = $('reportLink').value.trim();
    var people = $('reportPeople').value.trim();

    if (!EMAIL_RE.test(email)) { App.toast('error', '请输入有效的邮箱地址'); return; }
    if (!reason) { App.toast('error', '请填写标记原因'); return; }
    if (App.countChars(reason) > 500) { App.toast('error', '标记原因过长，最多 500 字'); return; }
    if (reportLinkConfig.evidenceRequired && !link) { App.toast('error', '事件链接为必填项，请提供有效的证据链接'); return; }
    if (link) {
      var linkErr = App.validateLink(link, reportLinkConfig.domains);
      if (linkErr) { App.toast('error', linkErr); return; }
    }
    if (reportImageConfig.required && reportImages.length === 0) {
      App.toast('error', '请上传证据图片'); return;
    }

    // 校验通过后进入 3 秒延时确认
    startUserDelayConfirm({ email: email, reason: reason, link: link, people: people, images: reportImages.slice() });
  });

  // ===== 用户提交的 3 秒延时确认 =====
  var userDelayPayload = null;
  var userDelayTimer = null;

  function startUserDelayConfirm(payload) {
    userDelayPayload = payload;
    var seconds = 3;
    var btn = $('userDelayConfirmBtn');
    btn.disabled = true;
    btn.textContent = '确认提交';
    $('userDelayCountdown').textContent = seconds;
    openModal('userDelayConfirmModal');

    if (userDelayTimer) clearInterval(userDelayTimer);
    userDelayTimer = setInterval(function () {
      seconds--;
      $('userDelayCountdown').textContent = seconds;
      if (seconds <= 0) {
        clearInterval(userDelayTimer);
        userDelayTimer = null;
        btn.disabled = false;
      }
    }, 1000);
  }

  $('userDelayConfirmBtn').addEventListener('click', function () {
    if (this.disabled) return;
    if (!userDelayPayload) return;
    closeModal('userDelayConfirmModal');
    if (userDelayTimer) { clearInterval(userDelayTimer); userDelayTimer = null; }
    doSubmitReport(userDelayPayload);
  });

  // 关闭延时弹窗时清理计时器
  document.querySelector('#userDelayConfirmModal .modal-close').addEventListener('click', function () {
    if (userDelayTimer) { clearInterval(userDelayTimer); userDelayTimer = null; }
  });

  async function doSubmitReport(p) {
    App.setLoading($('reportSubmitBtn'), true, '提交中…');
    try {
      var fd = new FormData();
      fd.append('email', p.email);
      fd.append('ban_reason', p.reason);
      fd.append('event_link', p.link);
      fd.append('event_related_people', p.people);
      if (p.images) {
        p.images.forEach(function (item, i) {
          fd.append('images', item.blob, 'image_' + i + '.webp');
        });
      }
      var data = await App.api('/api/v1/submit/report', { method: 'POST', body: fd });
      closeModal('submitModal');
      reportPicker.clear();
      showSubmitSuccess(data.query_code);
    } catch (e) {
      App.toast('error', e.message);
    } finally {
      App.setLoading($('reportSubmitBtn'), false);
    }
  }

  // —— 申诉（切换到「邮箱申诉」选项卡）——
  function openAppeal(email) {
    $('appealEmail').value = email || '';
    $('appealReason').value = '';
    $('appealEvidence').value = '';
    $('appealReasonCount').textContent = '0';
    appealPicker.clear();
    switchSubTab('appeal');
    openModal('submitModal');
  }

  // 在 blocked 结果中添加申诉按钮
  var _blockedState = blockedState;
  blockedState = function (d) {
    _blockedState(d);
    // 在结果底部添加申诉按钮
    var actions = document.createElement('div');
    actions.className = 'result-actions';
    actions.style.marginTop = '14px';
    actions.innerHTML = '<button id="resultAppealBtn" class="btn btn-secondary btn-sm" type="button">' +
      '<svg class="btn-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 1 0 2.64-6.36"></path><polyline points="3 3 3 8 8 8"></polyline></svg>' +
      '<span>我要申诉</span></button>';
    resultArea.querySelector('.result-state').appendChild(actions);
    $('resultAppealBtn').addEventListener('click', function () {
      openAppeal(d.email || emailInput.value.trim());
    });
  };

  $('appealReason').addEventListener('input', function () {
    $('appealReasonCount').textContent = App.countChars(this.value);
  });

  $('appealSubmitBtn').addEventListener('click', async function () {
    var email = $('appealEmail').value.trim().toLowerCase();
    var reason = $('appealReason').value.trim();
    var evidence = $('appealEvidence').value.trim();

    if (!EMAIL_RE.test(email)) { App.toast('error', '请输入有效的邮箱地址'); return; }
    if (!reason) { App.toast('error', '请填写申诉理由'); return; }
    if (App.countChars(reason) > 500) { App.toast('error', '申诉理由过长，最多 500 字'); return; }
    if (appealLinkConfig.evidenceRequired && !evidence) { App.toast('error', '反驳证据链接为必填项，请提供有效链接'); return; }
    if (evidence) {
      var linkErr = App.validateLink(evidence, appealLinkConfig.domains);
      if (linkErr) { App.toast('error', linkErr); return; }
    }
    if (appealImageConfig.required && appealImages.length === 0) {
      App.toast('error', '请上传证据图片'); return;
    }

    App.setLoading($('appealSubmitBtn'), true, '提交中…');
    try {
      var fd = new FormData();
      fd.append('email', email);
      fd.append('appeal_reason', reason);
      fd.append('appeal_evidence', evidence);
      appealImages.forEach(function (item, i) {
        fd.append('images', item.blob, 'image_' + i + '.webp');
      });
      var data = await App.api('/api/v1/submit/appeal', { method: 'POST', body: fd });
      closeModal('submitModal');
      appealPicker.clear();
      showSubmitSuccess(data.query_code);
    } catch (e) {
      App.toast('error', e.message);
    } finally {
      App.setLoading($('appealSubmitBtn'), false);
    }
  });

  // —— 查询进度 ——
  $('querySubBtn').addEventListener('click', function () {
    $('querySubInput').value = localStorage.getItem('lastQueryCode') || '';
    $('querySubResult').style.display = 'none';
    openModal('querySubModal');
    setTimeout(function () { $('querySubInput').focus(); }, 50);
  });

  $('querySubGoBtn').addEventListener('click', querySubmission);
  $('querySubInput').addEventListener('keydown', function (e) {
    if (e.key === 'Enter') querySubmission();
  });

  async function querySubmission() {
    var code = $('querySubInput').value.trim();
    if (!code) { App.toast('error', '请输入查询码'); return; }

    App.setLoading($('querySubGoBtn'), true, '查询中…');
    try {
      var data = await App.api('/api/v1/submission?code=' + encodeURIComponent(code));
      renderQueryResult(data);
    } catch (e) {
      $('querySubResult').style.display = 'none';
      App.toast('error', e.message);
    } finally {
      App.setLoading($('querySubGoBtn'), false);
    }
  }

  var SUB_STATUS = {
    pending: { label: '待审核', tag: 'pending' },
    approved: { label: '已通过', tag: 'approved' },
    rejected: { label: '已驳回', tag: 'rejected' }
  };
  var SUB_TYPE = { report: '举报', appeal: '申诉' };

  function renderQueryResult(d) {
    $('qrType').textContent = SUB_TYPE[d.type] || d.type;
    $('qrEmail').textContent = d.email || '—';

    var st = SUB_STATUS[d.status] || { label: d.status, tag: '' };
    var statusEl = $('qrStatus');
    statusEl.textContent = st.label;
    statusEl.className = 'sub-result-value sub-status-tag sub-status-' + st.tag;

    $('qrCreated').textContent = d.created_at || '—';

    $('qrReviewedRow').style.display = d.reviewed_at ? '' : 'none';
    $('qrReviewed').textContent = d.reviewed_at || '—';

    $('qrRejectRow').style.display = (d.status === 'rejected' && d.reject_reason) ? '' : 'none';
    $('qrRejectReason').textContent = d.reject_reason || '—';

    $('querySubResult').style.display = '';
  }

  // —— 提交成功弹窗 ——
  function showSubmitSuccess(code) {
    $('successQueryCode').textContent = code;
    localStorage.setItem('lastQueryCode', code);
    openModal('submitSuccessModal');
  }

  // ===== 公告 =====
  (async function loadAnnouncement() {
    try {
      const data = await App.api('/api/v1/announcement');
      if (data.content_html) {
        announcementContent.innerHTML = App.sanitizeHTML(data.content_html);
        announcementCard.style.display = '';
        hasAnnouncement = true;
      }
    } catch (e) { /* 静默忽略公告加载失败 */ }
  })();

  // ===== 必填星号与须知文案跟随配置开关 =====
  function updateRequiredUI() {
    var rStar = $('reportLinkStar');
    if (rStar) rStar.style.display = reportLinkConfig.evidenceRequired ? '' : 'none';
    var rNotice = $('reportLinkNotice');
    if (rNotice) rNotice.textContent = reportLinkConfig.evidenceRequired ? '事件链接为必填项' : '事件链接为选填项';

    var aStar = $('appealLinkStar');
    if (aStar) aStar.style.display = appealLinkConfig.evidenceRequired ? '' : 'none';
    var aNotice = $('appealLinkNotice');
    if (aNotice) aNotice.textContent = appealLinkConfig.evidenceRequired ? '反驳证据链接（必填）' : '反驳证据链接（选填）';
  }

  // ===== 联系管理员（收件箱配置后拉起本机邮件应用）=====
  (async function loadSiteConfig() {
    try {
      const data = await App.api('/api/v1/site-config');
      inboxEmail = data.inbox_email || '';
      reportLinkConfig = {
        evidenceRequired: data.report_evidence_required !== false,
        domains: data.report_link_domains || []
      };
      appealLinkConfig = {
        evidenceRequired: data.appeal_evidence_required !== false,
        domains: data.appeal_link_domains || []
      };
      reportImageConfig = {
        required: data.report_image_required === true,
        maxCount: data.report_image_max || 3
      };
      appealImageConfig = {
        required: data.appeal_image_required === true,
        maxCount: data.appeal_image_max || 3
      };
      if (reportPicker) reportPicker.updateConfig(reportImageConfig);
      if (appealPicker) appealPicker.updateConfig(appealImageConfig);
      updateRequiredUI();
    } catch (e) { /* 静默忽略 */ }
    var btn = $('contactAdminBtn');
    if (btn && inboxEmail) {
      btn.style.display = '';
      btn.addEventListener('click', function () {
        window.location.href = 'mailto:' + inboxEmail;
      });
    }
  })();

  // ===== 登录态检测：已登录则点击「管理员登录」直接进入后台 =====
  (async function checkAdminStatus() {
    try {
      const data = await App.api('/api/v1/admin/status');
      isAdminLoggedIn = !!data.logged_in;
      if (isAdminLoggedIn) $('adminEntryLabel').textContent = '进入后台';
    } catch (e) { /* 静默忽略 */ }
  })();
})();