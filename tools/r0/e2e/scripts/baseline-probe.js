#!/usr/bin/env node
'use strict';
/**
 * 第七次缺陷修复 · 修复前基线探针（只读，不改任何业务代码）。
 *
 * 产出（写入 docs/r0-evidence-7th-fix/baseline/）：
 *   - account-repro.md        第 4 项「新增账户」独立稳定复现 3 次（真实浏览器）
 *   - pre-fix-observations.md 交易总表 / 分类下拉 / member 邀请按钮 的修复前观测
 *   - baseline-probe.json     上述全部原始观测数据（可机器复核）
 *
 * 判定口径：区分「API 有、DOM 无」与「DOM 有但不可见」——
 *   viewTextContentHasName=true 且 viewInnerTextHasName=false  → DOM 有但被隐藏；
 *   两者皆 false                                              → API 有、DOM 无；
 *   两者皆 true                                               → 正常渲染。
 *
 * 不通过 sleep/延迟刷新制造 PASS：等待采用确定性信号（网络空闲、POST 响应、
 * #view 的 MutationObserver 重绘记录）；时间线快照仅作诊断记录，不作通过依据。
 */
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const { startServer } = require('../lib/server');
const { launch } = require('../lib/browser');
const { bootstrap, storageStateFor } = require('../lib/fixtures');
const api = require('../lib/api');

const OUT_DIR = path.resolve(__dirname, '..', '..', '..', '..', 'docs', 'r0-evidence-7th-fix', 'baseline');
const REPO_ROOT = path.resolve(__dirname, '..', '..', '..', '..');
const VIEWPORT = { width: 390, height: 844 };
const ITERATIONS = 3;

const nowIso = () => new Date().toISOString();

function gitSha() {
  try {
    return execFileSync('git', ['rev-parse', 'HEAD'], { cwd: REPO_ROOT, encoding: 'utf8' }).trim();
  } catch (_) {
    return 'unknown';
  }
}

/** 等待 SPA 完成当前路由的首轮请求（确定性网络信号，不是固定 sleep） */
async function settle(page, base) {
  await page.waitForLoadState('networkidle', { timeout: 20000 }).catch(() => {});
  await page.waitForFunction(() => !!document.querySelector('#view'), null, { timeout: 10000 }).catch(() => {});
  return page.url().replace(base, '');
}

/** 采集 #view 的完整可见性快照，用于区分「DOM 无」与「DOM 有但隐藏」 */
async function snapshot(page, needle) {
  return page.evaluate((name) => {
    const view = document.querySelector('#view');
    const bodyInnerText = document.body ? document.body.innerText || '' : '';
    const viewInnerText = view ? view.innerText || '' : '';
    const viewTextContent = view ? view.textContent || '' : '';
    const tds = view
      ? Array.from(view.querySelectorAll('td')).map((td) => {
          const cs = window.getComputedStyle(td);
          return {
            text: td.innerText,
            visible: !!(td.offsetWidth || td.offsetHeight || td.getClientRects().length),
            display: cs.display,
            visibility: cs.visibility,
          };
        })
      : [];
    const leaves = view
      ? Array.from(view.querySelectorAll('*')).filter(
          (el) => el.children.length === 0 && (el.textContent || '').includes(name),
        )
      : [];
    const hiddenLeaves = leaves.filter((el) => {
      const cs = window.getComputedStyle(el);
      return (
        cs.display === 'none' ||
        cs.visibility === 'hidden' ||
        !(el.offsetWidth || el.offsetHeight || el.getClientRects().length)
      );
    });
    const dlg = document.querySelector('#acctDlg');
    const err = document.querySelector('#acctErr');
    const toast = document.querySelector('#toast');
    const table = view ? view.querySelector('table tbody') : null;
    return {
      viewChildren: view ? view.childElementCount : -1,
      rowCount: table ? table.querySelectorAll('tr').length : 0,
      viewInnerText,
      viewTextContent,
      bodyInnerText,
      bodyHasName: bodyInnerText.includes(name),
      viewInnerTextHasName: viewInnerText.includes(name),
      viewTextContentHasName: viewTextContent.includes(name),
      tdCount: tds.length,
      tds,
      leafMatchCount: leaves.length,
      hiddenLeafMatchCount: hiddenLeaves.length,
      acctDlgOpen: dlg ? dlg.open : null,
      acctErr: err ? (err.textContent || '').trim() : '',
      toast: toast ? (toast.textContent || '').trim() : '',
      mutations: Array.isArray(window.__viewMutations) ? window.__viewMutations.slice() : [],
    };
  }, needle);
}

/** 安装 #view 重绘记录器：记录每一次 DOM 变化时的重绘内容与是否包含目标名 */
async function installObserver(page, needle) {
  await page.evaluate((name) => {
    window.__probeName = name;
    window.__viewMutations = [];
    const v = document.querySelector('#view');
    if (!v) return;
    const mo = new MutationObserver(() => {
      const text = v.textContent || '';
      const inner = v.innerText || '';
      window.__viewMutations.push({
        at: new Date().toISOString(),
        children: v.childElementCount,
        rows: v.querySelectorAll('table tbody tr').length,
        textHasName: text.includes(window.__probeName),
        innerTextHasName: inner.includes(window.__probeName),
        snippet: inner.slice(0, 240),
      });
    });
    mo.observe(v, { childList: true, subtree: true, characterData: true });
    window.__viewObserver = mo;
  }, needle);
}

/** 读取 #fCategory 的全部 option（文本 + value） */
async function categoryOptions(page) {
  return page.evaluate(() => {
    const sel = document.querySelector('#fCategory');
    if (!sel) return null;
    return {
      count: sel.options.length,
      value: sel.value,
      options: Array.from(sel.options).map((o) => ({ text: o.textContent, value: o.value })),
    };
  });
}

function attachDiagnostics(page, base, sink) {
  page.on('console', (msg) => {
    if (msg.type() === 'error' || msg.type() === 'warning') {
      sink.console.push({ at: nowIso(), type: msg.type(), text: msg.text().slice(0, 400) });
    }
  });
  page.on('pageerror', (e) => sink.console.push({ at: nowIso(), type: 'pageerror', text: String(e && e.message ? e.message : e).slice(0, 400) }));
  page.on('requestfailed', (r) => sink.network.push({ at: nowIso(), kind: 'requestfailed', url: r.url().replace(base, ''), reason: r.failure() ? r.failure().errorText : 'unknown' }));
  page.on('response', (r) => {
    const u = r.url();
    if (u.includes('/api/v1/')) {
      sink.network.push({ at: nowIso(), kind: 'response', method: r.request().method(), url: u.replace(base, ''), status: r.status() });
    }
  });
}

async function runAccountIteration({ browser, base, data, index }) {
  const name = `复现账户-${index}-${Date.now()}`;
  const sink = { console: [], network: [] };
  const context = await browser.newContext({ viewport: VIEWPORT, storageState: storageStateFor(data, 'owner') });
  const page = await context.newPage();
  attachDiagnostics(page, base, sink);
  const iter = { index, name, startedAt: nowIso() };
  try {
    const url = `${base}/#b/${data.books.main.id}/accounts`;
    await page.goto(url, { waitUntil: 'domcontentloaded' });
    iter.routeAfterGoto = await settle(page, base);
    iter.before = await snapshot(page, name);

    const addBtn = page.locator('button', { hasText: '＋ 新增账户' }).first();
    iter.addBtnCount = await page.locator('button', { hasText: '＋ 新增账户' }).count();
    await addBtn.click({ timeout: 10000 });
    iter.dlgOpenAfterClick = await page.locator('#acctDlg').evaluate((d) => d.open).catch(() => null);
    await page.fill('#acctName', name);
    await installObserver(page, name);

    const postPromise = page.waitForResponse(
      (r) => r.request().method() === 'POST' && /\/api\/v1\/books\/\d+\/accounts$/.test(r.url()),
      { timeout: 20000 },
    );
    await page.locator('#acctSubmit').click();
    const postResp = await postPromise;
    let postBody = '';
    try { postBody = (await postResp.text()).slice(0, 1000); } catch (_) { postBody = '(响应体不可读)'; }
    iter.post = { status: postResp.status(), url: postResp.url().replace(base, ''), body: postBody };

    // API 回读（Node 侧，独立于浏览器缓存）
    const apiAccs = await api.accounts(base, data.users.owner.token, data.books.main.id);
    iter.api = {
      count: Array.isArray(apiAccs) ? apiAccs.length : -1,
      hasName: Array.isArray(apiAccs) && apiAccs.some((a) => a.name === name),
      names: (Array.isArray(apiAccs) ? apiAccs : []).map((a) => a.name),
    };

    // 确定性 DOM 信号：POST 响应后等待 #view 出现重绘（不依赖固定 sleep）
    const redrawSeen = await page
      .waitForFunction(
        () => Array.isArray(window.__viewMutations) && window.__viewMutations.length > 0,
        null,
        { timeout: 8000 },
      )
      .then(() => true)
      .catch(() => false);
    iter.redrawSeen = redrawSeen;

    const nameAppeared = await page
      .waitForFunction(
        (n) => {
          const v = document.querySelector('#view');
          return !!v && (v.innerText || '').includes(n);
        },
        name,
        { timeout: 6000 },
      )
      .then(() => true)
      .catch(() => false);
    iter.nameAppearedWithin6s = nameAppeared;
    iter.afterSubmit = await snapshot(page, name);

    // 刷新页面（重新走 route → renderAccounts → loadBookMeta），验证数据是否本就存在
    await page.goto(url, { waitUntil: 'domcontentloaded' });
    iter.routeAfterReload = await settle(page, base);
    iter.afterReload = await snapshot(page, name);

    // 归类判定
    const s = iter.afterSubmit;
    if (s.viewTextContentHasName && s.viewInnerTextHasName) iter.classification = 'DOM 有且可见（正常）';
    else if (s.viewTextContentHasName && !s.viewInnerTextHasName) iter.classification = 'DOM 有但不可见/文本被隐藏';
    else iter.classification = 'API 有、DOM 无';
    if (iter.api.hasName && iter.afterReload.viewInnerTextHasName) iter.refreshNote = '刷新后可见（提交后当次渲染缺失）';
    else if (iter.api.hasName && !iter.afterReload.viewInnerTextHasName) iter.refreshNote = '刷新后仍不可见';
    else iter.refreshNote = 'API 无该账户';
  } catch (e) {
    iter.error = String(e && e.stack ? e.stack : e);
    iter.classification = '复现脚本异常';
  } finally {
    iter.console = sink.console;
    iter.network = sink.network;
    iter.finishedAt = nowIso();
    await context.close().catch(() => {});
  }
  return iter;
}

async function runObservations({ browser, base, data }) {
  const out = { startedAt: nowIso() };

  // 1) 交易总表修复前状态
  {
    const ctx = await browser.newContext({ viewport: VIEWPORT, storageState: storageStateFor(data, 'owner') });
    const page = await ctx.newPage();
    const sink = { console: [], network: [] };
    attachDiagnostics(page, base, sink);
    await page.goto(`${base}/#b/${data.books.main.id}/txns`, { waitUntil: 'domcontentloaded' });
    await settle(page, base);
    out.txns = await snapshot(page, '__NEVER_MATCH__');
    out.txns.console = sink.console;
    out.txns.network = sink.network.slice(-30);
    await ctx.close();
  }

  // 2) 记一笔弹窗：支出 / 收入 / 再切回支出 时 #fCategory 的全部 option
  {
    const ctx = await browser.newContext({ viewport: VIEWPORT, storageState: storageStateFor(data, 'owner') });
    const page = await ctx.newPage();
    await page.goto(`${base}/#b/${data.books.main.id}`, { waitUntil: 'domcontentloaded' });
    await settle(page, base);
    await page.locator('#fab').click();
    await page.waitForFunction(() => { const d = document.querySelector('#formDlg'); return !!d && d.open; }, null, { timeout: 10000 }).catch(() => {});
    out.addForm = { dlgOpen: await page.locator('#formDlg').evaluate((d) => d.open).catch(() => null) };
    await page.selectOption('#fType', '2');
    out.addForm.expense = await categoryOptions(page);
    out.addForm.expenseCatRowHidden = await page.locator('#fCatRow').evaluate((e) => e.classList.contains('hidden')).catch(() => null);
    await page.selectOption('#fType', '1');
    out.addForm.income = await categoryOptions(page);
    await page.selectOption('#fType', '3');
    out.addForm.transfer = await categoryOptions(page);
    out.addForm.transferCatRowHidden = await page.locator('#fCatRow').evaluate((e) => e.classList.contains('hidden')).catch(() => null);
    out.addForm.transferToRowHidden = await page.locator('#fToRow').evaluate((e) => e.classList.contains('hidden')).catch(() => null);
    await page.selectOption('#fType', '2');
    out.addForm.expenseAgain = await categoryOptions(page);
    // API 侧分类真实 catType 分布，用于判定下拉是否混入收入类
    const cats = await api.categories(base, data.users.owner.token, data.books.main.id, false);
    out.addForm.apiCategories = (Array.isArray(cats) ? cats : []).map((c) => ({ id: c.id, name: c.name, catType: c.catType, parentId: c.parentId }));
    await ctx.close();
  }

  // 3) member 身份进成员页，「邀请成员」按钮数量与 DOM 文本
  {
    const ctx = await browser.newContext({ viewport: VIEWPORT, storageState: storageStateFor(data, 'member') });
    const page = await ctx.newPage();
    const sink = { console: [], network: [] };
    attachDiagnostics(page, base, sink);
    await page.goto(`${base}/#b/${data.books.main.id}/members`, { waitUntil: 'domcontentloaded' });
    await settle(page, base);
    out.members = await page.evaluate(() => {
      const view = document.querySelector('#view');
      const btns = Array.from(document.querySelectorAll('button')).filter((b) => (b.textContent || '').includes('邀请成员'));
      return {
        inviteBtnCount: btns.length,
        viewInnerText: view ? view.innerText : '',
        viewHasInviteText: view ? (view.innerText || '').includes('邀请成员') : false,
        memberRowCount: view ? view.querySelectorAll('table tbody tr').length : 0,
        roles: Array.from(view ? view.querySelectorAll('table tbody tr td:nth-child(2)') : []).map((td) => td.innerText),
      };
    });
    out.members.console = sink.console;
    out.members.network = sink.network.slice(-30);
    // 后端权限校验仍须保留：member 直接 POST invite 应为 403
    const denied = await api.raw(base, 'POST', `/api/v1/books/${data.books.main.id}/members/invite`, {
      token: data.users.member.token, body: { role: 3 },
    });
    out.members.directInviteStatus = denied.status;
    await ctx.close();
  }

  // 4) owner / admin 身份进成员页，作为邀请按钮的对照基线
  {
    out.membersByRole = {};
    for (const role of ['owner', 'admin']) {
      const ctx = await browser.newContext({ viewport: VIEWPORT, storageState: storageStateFor(data, role) });
      const page = await ctx.newPage();
      await page.goto(`${base}/#b/${data.books.main.id}/members`, { waitUntil: 'domcontentloaded' });
      await settle(page, base);
      out.membersByRole[role] = await page.evaluate(() => {
        const view = document.querySelector('#view');
        return {
          inviteBtnCount: Array.from(document.querySelectorAll('button')).filter((b) => (b.textContent || '').includes('邀请成员')).length,
          memberRowCount: view ? view.querySelectorAll('table tbody tr').length : 0,
        };
      });
      await ctx.close();
    }
  }

  out.finishedAt = nowIso();
  return out;
}

function renderAccountRepro(raw) {
  const lines = [];
  lines.push('# 第 4 项缺陷 · 修复前独立稳定复现（新增账户 POST 后页面不出现）');
  lines.push('');
  lines.push('## 运行信息');
  lines.push('');
  lines.push(`- 采集时间：${raw.startedAt} ~ ${raw.finishedAt}`);
  lines.push(`- commit：\`${raw.gitSha}\`；分支：\`${raw.branch}\``);
  lines.push(`- 服务二进制：\`${raw.binary}\``);
  lines.push(`- 浏览器：${raw.browser ? `${raw.browser.from} ${raw.browser.version} channel=${raw.browser.channel}` : '-'}`);
  lines.push(`- 实例：${raw.serverBase}（临时 SQLite，随机端口，与主仓库数据隔离）`);
  lines.push('- 复现方式：真实浏览器；每次迭代新建独立 context/page，共用同一临时实例；POST 用 `waitForResponse` 捕获真实响应，`GET /accounts` 用 Node 侧 API 回读，DOM 重绘用 `#view` 的 MutationObserver 记录（确定性信号，不使用固定 sleep 制造结论）。');
  lines.push('- 判定口径：`viewTextContentHasName` = DOM 中存在该文本（含被 CSS 隐藏）；`viewInnerTextHasName` = 渲染文本中存在该文本。前者真/后者假 → 「DOM 有但不可见」；两者皆假 → 「API 有、DOM 无」。');
  lines.push('');
  lines.push('## 三次复现');
  lines.push('');
  for (const it of raw.iterations) {
    lines.push(`### 迭代 ${it.index} · \`${it.name}\``);
    lines.push('');
    if (it.error) {
      lines.push(`- 脚本异常：${it.error}`);
      lines.push('');
      continue;
    }
    lines.push(`- POST：\`${it.post.method || 'POST'} ${it.post.url}\` → HTTP **${it.post.status}**；响应体：\`${it.post.body.replace(/\n/g, ' ').slice(0, 400)}\``);
    lines.push(`- GET /accounts 回读：共 ${it.api.count} 个账户，含目标名 = **${it.api.hasName}**；名称列表：${it.api.names.map((n) => `\`${n}\``).join(', ')}`);
    lines.push(`- 提交前：\`#view.childElementCount=${it.before.viewChildren}\`，账户行数=${it.before.rowCount}，\`#acctDlg.open=${it.before.acctDlgOpen}\``);
    lines.push(`- 点击「＋ 新增账户」：匹配按钮数=${it.addBtnCount}，点击后 \`#acctDlg.open=${it.dlgOpenAfterClick}\``);
    lines.push(`- 提交后：\`#view.childElementCount=${it.afterSubmit.viewChildren}\`，账户行数=${it.afterSubmit.rowCount}，\`#acctDlg.open=${it.afterSubmit.acctDlgOpen}\`，\`#acctErr=「${it.afterSubmit.acctErr}」\`，toast=「${it.afterSubmit.toast}」`);
    lines.push(`- 提交后文本观测：\`viewTextContentHasName=${it.afterSubmit.viewTextContentHasName}\`，\`viewInnerTextHasName=${it.afterSubmit.viewInnerTextHasName}\`，\`bodyHasName=${it.afterSubmit.bodyHasName}\`，叶子节点命中=${it.afterSubmit.leafMatchCount}（其中隐藏=${it.afterSubmit.hiddenLeafMatchCount}）`);
    lines.push(`- 提交后全部 \`<td>\` 文本（共 ${it.afterSubmit.tdCount} 个）：${it.afterSubmit.tds.map((t) => `\`${t.text.replace(/\n/g, ' ')}\`${t.visible ? '' : '[不可见]'}`).join('；') || '(无)'}`);
    lines.push(`- \`#view\` 重绘记录（MutationObserver，共 ${it.afterSubmit.mutations.length} 次）：${it.afterSubmit.mutations.map((m) => `{rows=${m.rows},textHasName=${m.textHasName},innerTextHasName=${m.innerTextHasName}}`).join(' → ') || '(无重绘)'}`);
    lines.push(`- POST 响应后观测到重绘 = ${it.redrawSeen}；6s 内目标名出现在可见文本 = ${it.nameAppearedWithin6s}`);
    lines.push(`- 刷新页面后：\`#view.childElementCount=${it.afterReload.viewChildren}\`，账户行数=${it.afterReload.rowCount}，\`viewTextContentHasName=${it.afterReload.viewTextContentHasName}\`，\`viewInnerTextHasName=${it.afterReload.viewInnerTextHasName}\` → ${it.refreshNote}`);
    lines.push(`- **归类：${it.classification}**`);
    lines.push(`- console error/pageerror：${it.console.length ? it.console.map((c) => `[${c.type}] ${c.text}`).join('；') : '(无)'}`);
    lines.push(`- 网络日志（API，共 ${it.network.length} 条）：${it.network.map((n) => n.kind === 'response' ? `${n.method} ${n.url} → ${n.status}` : `${n.kind} ${n.url} ${n.reason || ''}`).join('；') || '(无)'}`);
    lines.push('');
  }
  lines.push('## 汇总与结论');
  lines.push('');
  const classifications = raw.iterations.map((it) => `${it.index}:${it.classification}`);
  lines.push(`- 三次归类：${classifications.join('；')}`);
  lines.push(`- 三次 API 回读均含目标名：${raw.iterations.every((it) => it.api && it.api.hasName)}`);
  lines.push(`- 三次提交后 DOM 可见文本均含目标名：${raw.iterations.every((it) => it.afterSubmit && it.afterSubmit.viewInnerTextHasName)}`);
  lines.push(`- 三次刷新后可见文本均含目标名：${raw.iterations.every((it) => it.afterReload && it.afterReload.viewInnerTextHasName)}`);
  lines.push('');
  lines.push('> 本文件仅记录修复前观测事实；根因判定见 `../root-cause-and-plan.md`。');
  lines.push('');
  return lines.join('\n');
}

function renderObservations(raw) {
  const o = raw.observations;
  const lines = [];
  lines.push('# 修复前基线观测（交易总表 / 分类下拉 / 邀请按钮）');
  lines.push('');
  lines.push(`- 采集时间：${o.startedAt} ~ ${o.finishedAt}；commit \`${raw.gitSha}\``);
  lines.push('');
  lines.push('## 1. 交易总表 `renderTxns`');
  lines.push('');
  lines.push(`- \`#view.childElementCount = ${o.txns.viewChildren}\``);
  lines.push(`- \`#view.innerText = 「${o.txns.viewInnerText.replace(/\n/g, ' / ')}」\`（长度 ${o.txns.viewInnerText.length}）`);
  lines.push(`- \`#view.textContent = 「${o.txns.viewTextContent.replace(/\n/g, ' / ')}」\`（长度 ${o.txns.viewTextContent.length}）`);
  lines.push(`- \`<td>\` 数 = ${o.txns.tdCount}；账户行数（tbody tr）= ${o.txns.rowCount}`);
  lines.push(`- console error/pageerror：${o.txns.console.length ? o.txns.console.map((c) => `[${c.type}] ${c.text}`).join('；') : '(无)'}`);
  lines.push(`- 网络日志尾部：${o.txns.network.map((n) => `${n.method || ''} ${n.url} ${n.status || n.reason || ''}`).join('；') || '(无)'}`);
  lines.push('');
  lines.push('## 2. 记一笔弹窗 `#fCategory`（修复前）');
  lines.push('');
  const fmt = (k, v) => {
    if (!v) return `- ${k}：(读取失败)`;
    return `- ${k}：option 数=${v.count}，当前 value=\`${v.value}\`；全部 option：${v.options.map((x) => `\`${x.text}\`(${x.value})`).join('、') || '(空)'}`;
  };
  lines.push(`- \`#formDlg.open = ${o.addForm.dlgOpen}\``);
  lines.push(fmt('支出（txType=2）', o.addForm.expense));
  lines.push(fmt('收入（txType=1）', o.addForm.income));
  lines.push(fmt('转账（txType=3）', o.addForm.transfer));
  lines.push(fmt('切回支出（txType=2）', o.addForm.expenseAgain));
  lines.push(`- 转账时 \`#fCatRow.hidden=${o.addForm.transferCatRowHidden}\`，\`#fToRow.hidden=${o.addForm.transferToRowHidden}\``);
  lines.push(`- API 真实分类（catType 1=收入 2=支出，共 ${o.addForm.apiCategories.length} 项）：${o.addForm.apiCategories.map((c) => `\`${c.name}\`(id=${c.id},catType=${c.catType},parent=${c.parentId})`).join('；')}`);
  lines.push('');
  lines.push('## 3. 成员页「邀请成员」按钮（修复前）');
  lines.push('');
  lines.push(`- member 身份：按钮数=**${o.members.inviteBtnCount}**，\`#view\` 内含「邀请成员」文本=${o.members.viewHasInviteText}，成员行数=${o.members.memberRowCount}，角色列=${o.members.roles.map((r) => `\`${r}\``).join('、')}`);
  lines.push(`- member 直接 POST \`/members/invite\`：HTTP **${o.members.directInviteStatus}**（后端 403 校验须原样保留）`);
  lines.push(`- owner 身份：按钮数=${o.membersByRole.owner.inviteBtnCount}，成员行数=${o.membersByRole.owner.memberRowCount}`);
  lines.push(`- admin 身份：按钮数=${o.membersByRole.admin.inviteBtnCount}，成员行数=${o.membersByRole.admin.memberRowCount}`);
  lines.push('');
  return lines.join('\n');
}

async function main() {
  fs.mkdirSync(OUT_DIR, { recursive: true });
  const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-baseline-probe-'));
  const raw = {
    startedAt: nowIso(),
    gitSha: gitSha(),
    branch: (() => {
      try { return execFileSync('git', ['branch', '--show-current'], { cwd: REPO_ROOT, encoding: 'utf8' }).trim(); }
      catch (_) { return 'unknown'; }
    })(),
    iterations: [],
  };
  let server;
  let browserHandle;
  try {
    server = await startServer({ dataDir });
    raw.serverBase = server.base;
    raw.binary = server.bin;
    const data = await bootstrap(server);
    data.origin = new URL(server.base).origin;
    browserHandle = await launch();
    raw.browser = { from: browserHandle.from, version: browserHandle.version, channel: browserHandle.channel };

    for (let i = 1; i <= ITERATIONS; i += 1) {
      process.stdout.write(`[probe] account repro iteration ${i}/${ITERATIONS}\n`);
      // eslint-disable-next-line no-await-in-loop
      const it = await runAccountIteration({ browser: browserHandle.browser, base: server.base, data, index: i });
      raw.iterations.push(it);
    }
    process.stdout.write('[probe] collecting pre-fix observations\n');
    raw.observations = await runObservations({ browser: browserHandle.browser, base: server.base, data });
  } catch (e) {
    raw.fatal = String(e && e.stack ? e.stack : e);
    process.stderr.write(`BASELINE_PROBE_ERROR ${raw.fatal}\n`);
  } finally {
    if (browserHandle) await browserHandle.browser.close().catch(() => {});
    if (server) await server.stop().catch(() => {});
    try { fs.rmSync(dataDir, { recursive: true, force: true }); } catch (_) { /* ignore */ }
  }
  raw.finishedAt = nowIso();
  fs.writeFileSync(path.join(OUT_DIR, 'baseline-probe.json'), JSON.stringify(raw, null, 2) + '\n');
  fs.writeFileSync(path.join(OUT_DIR, 'account-repro.md'), renderAccountRepro(raw));
  if (raw.observations) fs.writeFileSync(path.join(OUT_DIR, 'pre-fix-observations.md'), renderObservations(raw));
  process.stdout.write(`[probe] output -> ${OUT_DIR}\n`);
  if (raw.fatal) process.exitCode = 2;
}

main();
