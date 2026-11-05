'use strict';
/**
 * I-TXNS-01..19 逐控件真实执行配方。
 *
 * 由 C 套件 `groupTxns` 与 D 套件 `runTxnsControls` 共用，避免重复实现。
 * 每条控件：真实点击/输入/提交 + 真实观测值（DOM 文本、网络响应码、API 回读），
 * 独立 PASS / FAIL / BLOCKED，独立截图；不整页 PASS、不硬编码 PASS。
 *
 * 视口设为 1000×844：E2E 默认 390px 会触发 style.css L331 的窄屏规则，
 * 把 td:nth-child(1)（交易表复选框列 / 账户表名称列）隐藏，无法真实点击行勾选框；
 * 本配方改用宽视口让控件可见并可交互（账户页的窄屏可见性另由 D 套件专门回归）。
 */
const crypto = require('node:crypto');
const api = require('../lib/api');

const VIEWPORT = { width: 1000, height: 844 };
const ok = (detail) => ({ status: 'PASS', detail, shots: [] });
const bad = (detail) => ({ status: 'FAIL', detail, shots: [] });
const blocked = (detail) => ({ status: 'BLOCKED', detail, shots: [] });

const goBtn = (page) => page.locator('#view .filters button').nth(0);
const resetBtn = (page) => page.locator('#view .filters button').nth(1);

/** 等待当前请求结束后的确定性 DOM 信号：骨架屏被替换 */
async function settled(page) {
  await page.waitForFunction(
    () => !document.querySelector('#view [aria-busy="true"]'),
    null,
    { timeout: 12000 },
  ).catch(() => {});
}

/** 读取表格全部行的单元格文本（用 textContent，避开窄屏 CSS 隐藏列的干扰） */
async function tableRows(page) {
  return page.evaluate(() => {
    const v = document.querySelector('#view');
    const tbody = v ? v.querySelector('table tbody') : null;
    if (!tbody) return [];
    return Array.from(tbody.querySelectorAll('tr')).map((tr) => {
      const chk = tr.querySelector('input[type=checkbox]');
      return {
        cells: Array.from(tr.querySelectorAll('td')).map((td) => (td.textContent || '').trim()),
        hasCheckbox: !!chk,
        checked: chk ? chk.checked : null,
        id: chk ? chk.dataset.id : null,
      };
    });
  });
}

/** 读取筛选行当前值 */
async function filtersState(page) {
  return page.evaluate(() => {
    const f = document.querySelector('#view .filters');
    if (!f) return null;
    const kw = f.querySelector('input[placeholder="关键词搜索"]');
    return {
      kw: kw ? kw.value : null,
      selects: Array.from(f.querySelectorAll('select')).map((s) => s.value),
      dates: Array.from(f.querySelectorAll('input[type=date]')).map((d) => d.value),
    };
  });
}

/** 执行一次动作并捕获它触发的 GET /transactions 真实响应 */
async function query(page, action) {
  const [resp] = await Promise.all([
    page.waitForResponse(
      (r) => r.request().method() === 'GET' && /\/api\/v1\/books\/\d+\/transactions\?/.test(r.url()),
      { timeout: 15000 },
    ),
    action(),
  ]);
  await settled(page);
  let data = null;
  try {
    const j = await resp.json();
    data = j && j.data ? j.data : null;
  } catch (_) { /* 非 JSON */ }
  return { status: resp.status(), url: resp.url(), data };
}

/** 复位筛选条件（不依赖固定 sleep：等待骨架屏消失） */
async function reset(page) {
  await resetBtn(page).click({ timeout: 5000 }).catch(() => {});
  await settled(page);
}

/** 关闭任何可能残留的对话框，避免遮挡后续真实点击 */
async function closeDialogs(page) {
  for (const sel of ['#fCancel', '#eCancel', '#cCancel', '#acctCancel', '#iCancel']) {
    const b = page.locator(sel);
    // eslint-disable-next-line no-await-in-loop
    if (await b.isVisible().catch(() => false)) {
      // eslint-disable-next-line no-await-in-loop
      await b.click({ timeout: 2000 }).catch(() => {});
    }
  }
  await page.keyboard.press('Escape').catch(() => {});
}

/** 单条控件执行器：独立判定 + 独立截图；异常一律记为 FAIL 并写明原因 */
async function step(R, id, ctx, fn) {
  let r;
  try {
    r = await fn();
  } catch (e) {
    r = bad(`执行异常：${e && e.message ? e.message : e}`);
  }
  const shots = [];
  let shot = '';
  try { shot = await ctx.out.screenshot(ctx.page, ctx.shotDir, id); } catch (_) { /* 截图失败不影响判定 */ }
  if (shot) shots.push(shot);
  for (const p of (r.extraShots || [])) shots.push(p);
  delete r.extraShots;
  r.shots = shots;
  R[id] = r;
}

/**
 * 执行 I-TXNS-01..19。
 * @param {{page:object, base:string, data:object, out:object, shotDir:string}} ctx
 * @returns {Promise<{R:Object}>} R[id] = {status, detail, shots}
 */
async function runTxnsControls(ctx) {
  const { page, base, data, out, shotDir } = ctx;
  const R = {};
  const bookId = data.books.main.id;
  const token = data.users.owner.token;
  const url = `${base}/#b/${bookId}/txns`;
  const inner = { page, out, shotDir };

  await page.setViewportSize(VIEWPORT);
  await page.goto(url, { waitUntil: 'domcontentloaded' });
  // 强制重新执行 SPA：若上一用例留在同一 hash 且未卸载（例如注入 500 后的失败态），
  // 同 URL 的 goto 会被浏览器优化为不重新加载文档，导致读到旧 DOM。
  await page.reload({ waitUntil: 'domcontentloaded' });
  await page.waitForLoadState('networkidle', { timeout: 20000 }).catch(() => {});
  await settled(page);

  const initialRows = await tableRows(page);
  const dataRows = initialRows.filter((r) => r.hasCheckbox);
  const firstNote = (dataRows.find((r) => r.cells[5]) || {}).cells
    ? (dataRows.find((r) => r.cells[5]) || {}).cells[5].trim()
    : '';
  const firstType = dataRows.length ? dataRows[0].cells[2].trim() : '';
  const typeValue = firstType === '收入' ? '1' : firstType === '支出' ? '2' : firstType === '转账' ? '3' : null;
  const accounts = await api.accounts(base, token, bookId).catch(() => []);
  const categories = await api.categories(base, token, bookId, false).catch(() => []);
  const firstCatName = (dataRows.find((r) => r.cells[4] && r.cells[4] !== '—') || {}).cells
    ? (dataRows.find((r) => r.cells[4] && r.cells[4] !== '—') || {}).cells[4].trim()
    : '';
  const firstCat = categories.find((c) => c.name === firstCatName) || null;
  const firstAcc = accounts[0] || null;

  /* 01 关键词搜索 */
  await step(R, 'I-TXNS-01', inner, async () => {
    await reset(page);
    const needle = firstNote || 'e2e';
    await page.fill('#view input[placeholder="关键词搜索"]', needle);
    const q = await query(page, () => goBtn(page).click());
    const rows = await tableRows(page);
    const notes = rows.filter((r) => r.hasCheckbox).map((r) => r.cells[5] || '');
    const pass = q.status === 200 && /keyword=/.test(q.url) && rows.length >= 1 && notes.length >= 1
      && notes.every((n) => n.includes(needle));
    return pass
      ? ok(`关键词「${needle}」→ HTTP ${q.status}，命中 ${notes.length} 行，备注=${notes.map((n) => `「${n}」`).join('、')}；全部含关键词`)
      : bad(`HTTP ${q.status} url=${q.url}；命中行=${rows.length} 备注=${JSON.stringify(notes)} 关键词=${needle}`);
  });

  /* 02 类型筛选 */
  await step(R, 'I-TXNS-02', inner, async () => {
    await reset(page);
    if (!typeValue) return blocked(`初始数据无可识别类型（首行类型=「${firstType}」），无法执行类型筛选`);
    await page.locator('#view .filters select').nth(0).selectOption(typeValue);
    const q = await query(page, () => goBtn(page).click());
    const rows = await tableRows(page);
    const types = rows.filter((r) => r.hasCheckbox).map((r) => r.cells[2] || '');
    const pass = q.status === 200 && new RegExp(`type=${typeValue}`).test(q.url)
      && types.length >= 1 && types.every((t) => t === firstType);
    return pass
      ? ok(`选「${firstType}」→ HTTP ${q.status} url 含 type=${typeValue}；${types.length} 行类型全部为「${firstType}」`)
      : bad(`HTTP ${q.status} url=${q.url}；类型列=${JSON.stringify(types)} 期望全为「${firstType}」`);
  });

  /* 03 账户筛选 */
  await step(R, 'I-TXNS-03', inner, async () => {
    await reset(page);
    if (!firstAcc) return blocked('API 未返回账户，无法执行账户筛选');
    await page.locator('#view .filters select').nth(1).selectOption(String(firstAcc.id));
    const q = await query(page, () => goBtn(page).click());
    const rows = await tableRows(page);
    const accs = rows.filter((r) => r.hasCheckbox).map((r) => r.cells[3] || '');
    const pass = q.status === 200 && new RegExp(`accountId=${firstAcc.id}`).test(q.url)
      && accs.length >= 1 && accs.every((a) => a.includes(firstAcc.name));
    return pass
      ? ok(`选账户「${firstAcc.name}」(id=${firstAcc.id}) → HTTP ${q.status}；${accs.length} 行账户列=${JSON.stringify(accs)} 全部命中`)
      : bad(`HTTP ${q.status} url=${q.url}；账户列=${JSON.stringify(accs)} 期望含「${firstAcc.name}」`);
  });

  /* 04 分类筛选 */
  await step(R, 'I-TXNS-04', inner, async () => {
    await reset(page);
    if (!firstCat) return blocked(`未在 API 分类中找到「${firstCatName}」，无法执行分类筛选`);
    await page.locator('#view .filters select').nth(2).selectOption(String(firstCat.id));
    const q = await query(page, () => goBtn(page).click());
    const rows = await tableRows(page);
    const cats = rows.filter((r) => r.hasCheckbox).map((r) => r.cells[4] || '');
    const pass = q.status === 200 && new RegExp(`categoryId=${firstCat.id}`).test(q.url)
      && cats.length >= 1 && cats.every((c) => c === firstCatName);
    return pass
      ? ok(`选分类「${firstCatName}」(id=${firstCat.id}) → HTTP ${q.status}；${cats.length} 行分类列=${JSON.stringify(cats)}`)
      : bad(`HTTP ${q.status} url=${q.url}；分类列=${JSON.stringify(cats)} 期望全为「${firstCatName}」`);
  });

  /* 05 开始日期 */
  await step(R, 'I-TXNS-05', inner, async () => {
    await reset(page);
    await page.locator('#view .filters input[type=date]').nth(0).fill('2000-01-01');
    const q = await query(page, () => goBtn(page).click());
    const rows = await tableRows(page);
    const pass = q.status === 200 && /minTime=/.test(q.url) && rows.filter((r) => r.hasCheckbox).length >= 1;
    return pass
      ? ok(`开始日期 2000-01-01 → HTTP ${q.status}，url 含 minTime=；返回 ${rows.filter((r) => r.hasCheckbox).length} 行`)
      : bad(`HTTP ${q.status} url=${q.url} 行数=${rows.length}`);
  });

  /* 06 结束日期 */
  await step(R, 'I-TXNS-06', inner, async () => {
    await reset(page);
    await page.locator('#view .filters input[type=date]').nth(1).fill('2099-12-31');
    const q = await query(page, () => goBtn(page).click());
    const rows = await tableRows(page);
    const pass = q.status === 200 && /maxTime=/.test(q.url) && rows.filter((r) => r.hasCheckbox).length >= 1;
    return pass
      ? ok(`结束日期 2099-12-31 → HTTP ${q.status}，url 含 maxTime=；返回 ${rows.filter((r) => r.hasCheckbox).length} 行`)
      : bad(`HTTP ${q.status} url=${q.url} 行数=${rows.length}`);
  });

  /* 07 查询按钮 */
  await step(R, 'I-TXNS-07', inner, async () => {
    await reset(page);
    const q = await query(page, () => goBtn(page).click());
    const rows = await tableRows(page);
    const pass = q.status === 200 && /page=1&count=50/.test(q.url) && rows.length >= 1;
    return pass
      ? ok(`点击「查询」→ GET ${q.url.split('/api/')[1] || q.url} → HTTP ${q.status}；重绘后 tbody ${rows.length} 行`)
      : bad(`HTTP ${q.status} url=${q.url} 行数=${rows.length}`);
  });

  /* 08 重置按钮 */
  await step(R, 'I-TXNS-08', inner, async () => {
    await page.fill('#view input[placeholder="关键词搜索"]', '重置前探测');
    await page.locator('#view .filters select').nth(0).selectOption('2');
    await page.locator('#view .filters input[type=date]').nth(0).fill('2020-01-01');
    const q = await query(page, () => resetBtn(page).click());
    const st = await filtersState(page);
    const clean = st && st.kw === '' && st.selects.every((v) => v === '0') && st.dates.every((v) => v === '');
    const pass = q.status === 200 && !/keyword=/.test(q.url) && clean;
    return pass
      ? ok(`点击「重置」→ HTTP ${q.status}，url 不含 keyword；筛选值回默认 kw=「${st.kw}」selects=${JSON.stringify(st.selects)} dates=${JSON.stringify(st.dates)}`)
      : bad(`HTTP ${q.status} url=${q.url} 筛选值=${JSON.stringify(st)}`);
  });

  /* 09 回车提交 */
  await step(R, 'I-TXNS-09', inner, async () => {
    await reset(page);
    const needle = firstNote || 'e2e';
    await page.fill('#view input[placeholder="关键词搜索"]', needle);
    const q = await query(page, () => page.locator('#view input[placeholder="关键词搜索"]').press('Enter'));
    const rows = await tableRows(page);
    const pass = q.status === 200 && /keyword=/.test(q.url) && rows.filter((r) => r.hasCheckbox).length >= 1;
    return pass
      ? ok(`关键词框按 Enter → HTTP ${q.status}，url 含 keyword=；返回 ${rows.filter((r) => r.hasCheckbox).length} 行`)
      : bad(`HTTP ${q.status} url=${q.url} 行数=${rows.length}`);
  });

  /* 10 全选本页 */
  await step(R, 'I-TXNS-10', inner, async () => {
    await reset(page);
    const all = page.locator('#view .bulkbar input[type=checkbox]').first();
    if (await all.isChecked()) await all.uncheck();
    await all.check();
    const rows = await tableRows(page);
    const n = rows.filter((r) => r.hasCheckbox).length;
    const checked = rows.filter((r) => r.hasCheckbox && r.checked === true).length;
    const info = await page.locator('#view .bulkbar span.muted').innerText().catch(() => '');
    const delDisabled = await page.locator('#view .bulkbar button', { hasText: '删除选中' }).isDisabled().catch(() => null);
    const pass = n >= 1 && checked === n && new RegExp(`已选 ${n} 项`).test(info) && delDisabled === false;
    return pass
      ? ok(`勾选「全选本页」→ ${n} 行全部 checked；提示=「${info}」；删除选中 disabled=${delDisabled}`)
      : bad(`行数=${n} checked=${checked} info=「${info}」delDisabled=${delDisabled}`);
  });

  /* 11 行勾选 */
  await step(R, 'I-TXNS-11', inner, async () => {
    await reset(page);
    const all = page.locator('#view .bulkbar input[type=checkbox]').first();
    // reset 不清空「全选本页」复选框；若上一条用例留下了 checked=true，
    // 直接 check() 不会触发 change 事件，必须先显式取消再重新勾选。
    if (await all.isChecked()) await all.uncheck();
    await all.check();
    const before = await tableRows(page);
    const n = before.filter((r) => r.hasCheckbox).length;
    const firstChk = page.locator('#view table tbody tr input[type=checkbox]').first();
    await firstChk.uncheck();
    const after = await tableRows(page);
    const checked = after.filter((r) => r.hasCheckbox && r.checked === true).length;
    const info = await page.locator('#view .bulkbar span.muted').innerText().catch(() => '');
    const pass = n >= 2 && checked === n - 1 && new RegExp(`已选 ${n - 1} 项`).test(info);
    return pass
      ? ok(`全选 ${n} 项后取消首行 → checked=${checked}，提示=「${info}」（真实 change 事件驱动既有 handler）`)
      : bad(`行数=${n} 取消后 checked=${checked} info=「${info}」`);
  });

  /* 12 删除选中（真实删除 + API 回读） */
  await step(R, 'I-TXNS-12', inner, async () => {
    await reset(page);
    const note = `D-删除用例-${Date.now()}`;
    const created = await api.createTx(base, token, bookId, {
      clientUuid: crypto.randomUUID(), txType: 2,
      accountId: data.ids.cashId, amount: 1234,
      categoryId: data.ids.expenseCatId, txTime: Math.floor(Date.now() / 1000),
      timezoneOffset: 480, comment: note, source: 0,
    });
    const txId = created.transactionIds[0];
    await query(page, () => goBtn(page).click());
    const target = page.locator('#view table tbody tr', { hasText: note }).first();
    if (!(await target.count())) return bad(`专用交易 ${txId} 创建成功但未出现在表格，无法执行删除`);
    await target.locator('input[type=checkbox]').check();
    const delBtn = page.locator('#view .bulkbar button', { hasText: '删除选中' });
    await delBtn.click();
    await page.locator('#confirmDlg[open]').waitFor({ state: 'visible', timeout: 6000 }).catch(() => {});
    const delReq = page.waitForRequest(
      (r) => r.method() === 'DELETE' && new RegExp(`/transactions/${txId}`).test(r.url()),
      { timeout: 10000 },
    ).catch(() => null);
    await page.locator('#cOk').click({ timeout: 5000 }).catch(() => {});
    const req = await delReq;
    await settled(page);
    const after = await api.transactions(base, token, bookId);
    const items = Array.isArray(after) ? after : after.items || [];
    const rec = await api.recycle(base, token, bookId);
    const rlist = Array.isArray(rec) ? rec : rec.items || [];
    const gone = items.every((t) => t.id !== txId);
    const inRecycle = rlist.some((x) => x.id === txId);
    const pass = !!req && gone && inRecycle;
    return pass
      ? ok(`勾选专用交易 ${txId} → 真实 DELETE 请求已发出；API 列表已无该笔=${gone}，回收站含该笔=${inRecycle}`)
      : bad(`DELETE 请求=${!!req} 列表已无=${gone} 回收站含=${inRecycle} txId=${txId}`);
  });

  /* 13 删除选中取消（断言无 DELETE、数据不变） */
  await step(R, 'I-TXNS-13', inner, async () => {
    await reset(page);
    const before = await api.transactions(base, token, bookId);
    const nBefore = Array.isArray(before) ? before.length : (before.items || []).length;
    const firstChk = page.locator('#view table tbody tr input[type=checkbox]').first();
    if (!(await firstChk.count())) return blocked('表格无可勾选行，无法执行取消删除');
    await firstChk.check();
    let deleteCount = 0;
    const onReq = (r) => { if (r.method() === 'DELETE' && /\/transactions\/\d+/.test(r.url())) deleteCount += 1; };
    page.on('request', onReq);
    await page.locator('#view .bulkbar button', { hasText: '删除选中' }).click();
    await page.locator('#confirmDlg[open]').waitFor({ state: 'visible', timeout: 6000 }).catch(() => {});
    await page.locator('#cCancel').click({ timeout: 5000 }).catch(() => {});
    await page.waitForLoadState('networkidle', { timeout: 8000 }).catch(() => {});
    page.off('request', onReq);
    const after = await api.transactions(base, token, bookId);
    const nAfter = Array.isArray(after) ? after.length : (after.items || []).length;
    const pass = deleteCount === 0 && nAfter === nBefore;
    return pass
      ? ok(`确认框点「取消」→ DELETE 请求数=${deleteCount}，交易数 ${nBefore}→${nAfter}（未删除）`)
      : bad(`DELETE 请求数=${deleteCount}，交易数 ${nBefore}→${nAfter}`);
  });

  /* 14 ＋ 记一笔 */
  await step(R, 'I-TXNS-14', inner, async () => {
    await reset(page);
    await closeDialogs(page);
    await page.locator('#view .bulkbar button', { hasText: '＋ 记一笔' }).click();
    const opened = await page.locator('#formDlg').evaluate((d) => d.open).catch(() => false);
    const catCount = await page.locator('#fCategory option').count();
    await page.locator('#fCancel').click({ timeout: 3000 }).catch(() => {});
    const closed = await page.locator('#formDlg').evaluate((d) => !d.open).catch(() => false);
    const pass = opened === true && closed === true && catCount > 0;
    return pass
      ? ok(`点击「＋ 记一笔」→ #formDlg.open=${opened}，分类下拉 ${catCount} 项；取消后关闭=${closed}`)
      : bad(`opened=${opened} closed=${closed} 分类项=${catCount}`);
  });

  /* 15 行编辑 */
  await step(R, 'I-TXNS-15', inner, async () => {
    await reset(page);
    await closeDialogs(page);
    const rows = await tableRows(page);
    const first = rows.find((r) => r.hasCheckbox);
    if (!first) return blocked('表格无数据行，无法执行行编辑');
    await page.locator('#view table tbody tr button', { hasText: '编辑' }).first().click();
    const opened = await page.locator('#editDlg').evaluate((d) => d.open).catch(() => false);
    const eType = await page.locator('#eType').innerText().catch(() => '');
    const eAmount = await page.locator('#eAmount').inputValue().catch(() => '');
    await page.locator('#eCancel').click({ timeout: 3000 }).catch(() => {});
    const closed = await page.locator('#editDlg').evaluate((d) => !d.open).catch(() => false);
    const pass = opened === true && closed === true && eType.trim() === first.cells[2].trim();
    return pass
      ? ok(`行「编辑」→ #editDlg.open=${opened}，eType=「${eType}」与行类型「${first.cells[2]}」一致，eAmount=「${eAmount}」；取消后关闭=${closed}`)
      : bad(`opened=${opened} eType=「${eType}」行类型=「${first.cells[2]}」closed=${closed}`);
  });

  /* 16 上一页 / 下一页 */
  await step(R, 'I-TXNS-16', inner, async () => {
    await reset(page);
    const pager = page.locator('#view .pager');
    const pagerText = await pager.innerText().catch(() => '');
    const prevDisabled = await pager.locator('button').nth(0).isDisabled().catch(() => null);
    const nextDisabled = await pager.locator('button').nth(1).isDisabled().catch(() => null);
    const all = await api.transactions(base, token, bookId);
    const total = Array.isArray(all) ? all.length : (all.total !== undefined ? all.total : (all.items || []).length);
    const count = 50;
    const expectDisabled = total <= count;
    let clicked = '未点击（数据不足一页，按设计禁用）';
    if (nextDisabled === false) {
      const q = await query(page, () => pager.locator('button').nth(1).click());
      const text2 = await pager.innerText().catch(() => '');
      clicked = `点击下一页 → HTTP ${q.status} url=${q.url.split('?')[1]} 页码=「${text2.replace(/\n/g, ' ')}」`;
      await query(page, () => pager.locator('button').nth(0).click());
      if (!/page=2/.test(q.url)) return bad(`点击下一页未发出 page=2 请求：${q.url}`);
    }
    const pass = /共 \d+ 条 · 第 1 页/.test(pagerText)
      && prevDisabled === true && nextDisabled === expectDisabled
      && new RegExp(`共 ${total} 条`).test(pagerText);
    return pass
      ? ok(`分页条=「${pagerText.replace(/\n/g, ' ')}」API total=${total} count=${count}；上一页 disabled=${prevDisabled}，下一页 disabled=${nextDisabled}（期望 ${expectDisabled}）；${clicked}`)
      : bad(`pager=「${pagerText.replace(/\n/g, ' ')}」total=${total} prevDisabled=${prevDisabled} nextDisabled=${nextDisabled} 期望=${expectDisabled}`);
  });

  /* 17 骨架屏：受控慢网络采集加载态，之后断言已被真实表格替换 */
  await step(R, 'I-TXNS-17', inner, async () => {
    const slowRoute = /\/api\/v1\/books\/\d+\/transactions\?/;
    await page.route(slowRoute, async (route) => {
      await new Promise((r) => setTimeout(r, 700));   // 仅用于制造可观测的加载中窗口，不参与任何 PASS 判定
      await route.continue();
    });
    let skeletonSeen = false;
    let bars = 0;
    let extraShots = [];
    try {
      const nav = page.reload({ waitUntil: 'domcontentloaded' });
      await page.waitForSelector('#view [aria-busy="true"]', { timeout: 8000 });
      skeletonSeen = true;
      bars = await page.locator('#view .skel').count();
      const p = await out.screenshot(page, shotDir, 'I-TXNS-17-skeleton');
      extraShots = [p];
      await nav;
      await settled(page);
    } catch (_) { /* 采集失败按观测值判定 */ }
    await page.unroute(slowRoute).catch(() => {});
    const busyAfter = await page.locator('#view [aria-busy="true"]').count();
    const rows = await tableRows(page);
    const pass = skeletonSeen && bars === 5 && busyAfter === 0 && rows.length >= 1;
    const r = pass
      ? ok(`慢网络下观测到骨架屏 aria-busy=true、.skel ${bars} 条；响应后骨架屏残留=${busyAfter}，tbody ${rows.length} 行（截图 I-TXNS-17-skeleton 为加载态）`)
      : bad(`观测到骨架屏=${skeletonSeen} .skel=${bars} 残留 busy=${busyAfter} 行数=${rows.length}`);
    r.extraShots = extraShots;
    return r;
  });

  /* 18 空态 */
  await step(R, 'I-TXNS-18', inner, async () => {
    await reset(page);
    await page.fill('#view input[placeholder="关键词搜索"]', '绝不存在的关键词XYZ-9999');
    const q = await query(page, () => goBtn(page).click());
    const emptyText = await page.locator('#view .empty').innerText().catch(() => '');
    const rows = await tableRows(page);
    const pass = q.status === 200 && /没有符合条件的交易/.test(emptyText) && rows.length === 1;
    const result = pass
      ? ok(`无匹配结果 → HTTP ${q.status}；空态=「${emptyText.replace(/\n/g, ' ')}」；tbody ${rows.length} 行`)
      : bad(`HTTP ${q.status} 空态=「${emptyText.replace(/\n/g, ' ')}」行数=${rows.length}`);
    await reset(page);
    return result;
  });

  /* 19 加载失败：拦截 500，断言有可见错误反馈且不是永久骨架屏（放在最后） */
  await step(R, 'I-TXNS-19', inner, async () => {
    const failRoute = /\/api\/v1\/books\/\d+\/transactions\?/;
    await page.route(failRoute, (route) => route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ code: 500, msg: 'E2E 注入的服务器错误', data: null }),
    }));
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForFunction(() => {
      const v = document.querySelector('#view');
      return !!v && (v.innerText || '').includes('交易加载失败');
    }, null, { timeout: 12000 }).catch(() => {});
    const viewText = await page.locator('#view').innerText().catch(() => '');
    const busy = await page.locator('#view [aria-busy="true"]').count();
    const toast = await page.locator('#toast').innerText().catch(() => '');
    await page.unroute(failRoute).catch(() => {});
    const errLine = (viewText.match(/交易加载失败[^\n]*/) || [''])[0];
    const pass = /交易加载失败/.test(viewText) && /E2E 注入的服务器错误/.test(viewText)
      && busy === 0 && /E2E 注入的服务器错误/.test(toast);
    return pass
      ? ok(`注入 HTTP 500 → 页面可见反馈=「${errLine}」；骨架屏残留 aria-busy=${busy}（非永久）；toast=「${toast}」`)
      : bad(`errLine=「${errLine}」view 片段=「${viewText.replace(/\n/g, ' ').slice(-160)}」busy=${busy} toast=「${toast}」`);
  });

  return { R };
}

module.exports = { runTxnsControls, VIEWPORT };
