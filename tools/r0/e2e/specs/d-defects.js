'use strict';
/**
 * 包 D · 第七次 4 项授权缺陷的自动化回归（真实浏览器 + 真实网络 + API 回读）。
 *
 * 覆盖：
 *   - 交易总表：有数据/空态/请求失败/行编辑 + I-TXNS-01..19 逐条真实重跑
 *   - 记一笔分类：收入/支出下拉隔离、类型切换重置、支出创建 2xx 与 API 一致、刷新仍在、转账 categoryId:0 语义
 *   - 成员邀请：owner/admin 可见可用、member 不可见、member 直接 POST 仍 403、成员列表不受影响、/users/me 失败关闭
 *   - 新增账户：POST 响应 + 重绘确定性 DOM 信号、刷新后仍在、只 +1 无重复、API↔DOM 全量一致
 *                （仅「DOM 渲染一致性，不含视觉可见性」；视觉可见性验收项未闭环，待 Owner 裁决 CSS）
 *
 * 每个断言都有真实观测值（DOM 文本 / 网络响应码 / 请求体 / API 回读），
 * 不使用「代码里有该函数」「截图存在」代替，不硬编码 PASS。
 */
const crypto = require('node:crypto');
const api = require('../lib/api');
const { storageStateFor } = require('../lib/fixtures');
const { attachDiagnostics } = require('../lib/evidence');
const { runTxnsControls } = require('./txns-controls');

const VIEWPORT = { width: 390, height: 844 };
const ok = (detail) => ({ status: 'PASS', detail });
const bad = (detail) => ({ status: 'FAIL', detail });
const blocked = (detail) => ({ status: 'BLOCKED', detail });

async function newSession(browser, data, role) {
  const ctx = await browser.newContext({ viewport: VIEWPORT, storageState: storageStateFor(data, role) });
  const page = await ctx.newPage();
  const diag = [];
  attachDiagnostics(page, diag);
  return { ctx, page, diag };
}

async function settle(page) {
  await page.waitForLoadState('networkidle', { timeout: 20000 }).catch(() => {});
  await page.waitForFunction(
    () => !document.querySelector('#view [aria-busy="true"]'),
    null,
    { timeout: 12000 },
  ).catch(() => {});
}

/**
 * 视觉可见性原始观测（只记录，不作为任何 PASS 依据）：
 *   visibleInnerText —— #view.innerText 是否包含账户名（被 CSS 隐藏时为 false）
 *   firstTdDisplay   —— 命中账户名那一行的首列 td 的 computed display（390px 下为 none）
 * 收紧裁决要求每条新增账户用例都保留这两个字段。
 */
async function visibilityProbe(page, name) {
  return page.evaluate((n) => {
    const v = document.querySelector('#view');
    const tds = Array.from(document.querySelectorAll('#view table tbody tr td:first-child'));
    const hit = tds.find((td) => (td.textContent || '').trim() === n);
    return {
      visibleInnerText: v ? (v.innerText || '').includes(n) : false,
      firstTdDisplay: hit ? window.getComputedStyle(hit).display : '(未找到对应 td)',
    };
  }, name);
}

async function optionsOf(page, sel) {
  return page.evaluate((s) => {
    const el = document.querySelector(s);
    if (!el) return null;
    return Array.from(el.options).map((o) => ({ text: o.textContent, value: o.value }));
  }, sel);
}

async function catOptions(page) {
  return optionsOf(page, '#fCategory');
}

/** 捕获一次 POST 请求与其响应（真实网络观测） */
async function capturePost(page, urlRe, action) {
  const [req, resp] = await Promise.all([
    page.waitForRequest((r) => r.method() === 'POST' && urlRe.test(r.url()), { timeout: 15000 }),
    page.waitForResponse((r) => r.request().method() === 'POST' && urlRe.test(r.url()), { timeout: 15000 }),
    action(),
  ]);
  let body = {};
  try { body = JSON.parse(req.postData() || '{}'); } catch (_) { /* 非 JSON */ }
  let respBody = null;
  try { respBody = await resp.json(); } catch (_) { /* 非 JSON */ }
  return { status: resp.status(), url: req.url(), body, respBody };
}

async function runDefects({ browser, base, data, runDir }) {
  const results = [];
  const mainId = data.books.main.id;
  const token = data.users.owner.token;
  const txnsUrl = `${base}/#b/${mainId}/txns`;

  /** 单条用例：真实执行 + 独立判定 + 独立截图 */
  const step = async (id, name, page, subdir, fn) => {
    let r;
    try { r = await fn(); }
    catch (e) { r = bad(`执行异常：${e && e.message ? e.message : e}`); }
    const evidence = [];
    try {
      const p = await runDir.screenshot(page, subdir, id);
      evidence.push(runDir.rel(p));
    } catch (_) { /* 截图失败不影响判定 */ }
    results.push({ suite: 'D', id, name, status: r.status, detail: r.detail, evidence });
  };

  /* ---------------- A 组：交易总表 ---------------- */
  {
    const session = await newSession(browser, data, 'owner');
    const { ctx, page, diag } = session;
    try {
      await step('D-TXNS-01', '交易总表：有数据列表实际显示（行数 + 备注文本）', page, 'D/txns', async () => {
        await page.goto(txnsUrl, { waitUntil: 'domcontentloaded' });
        await settle(page);
        const snap = await page.evaluate(() => {
          const v = document.querySelector('#view');
          const trs = v ? Array.from(v.querySelectorAll('table tbody tr')) : [];
          return {
            children: v ? v.childElementCount : -1,
            heading: v && v.querySelector('h2') ? v.querySelector('h2').textContent : '',
            rows: trs.length,
            notes: trs.map((tr) => {
              const tds = tr.querySelectorAll('td');
              return tds.length >= 6 ? (tds[5].textContent || '').trim() : '';
            }),
            filters: v ? v.querySelectorAll('.filters select').length : 0,
          };
        });
        const list = await api.transactions(base, token, mainId);
        const items = Array.isArray(list) ? list : list.items || [];
        const pass = snap.children >= 1 && snap.rows >= 1 && /交易/.test(snap.heading)
          && snap.filters === 3 && snap.notes.some((n) => n.length > 0) && items.length >= 1;
        return pass
          ? ok(`#view.childElementCount=${snap.children}，标题=「${snap.heading}」，筛选下拉=${snap.filters} 个，tbody ${snap.rows} 行，备注列=${JSON.stringify(snap.notes)}；API 交易数=${items.length}`)
          : bad(`children=${snap.children} heading=「${snap.heading}」filters=${snap.filters} rows=${snap.rows} notes=${JSON.stringify(snap.notes)} api=${items.length}`);
      });

      await step('D-TXNS-02', '交易总表：空数据显示空态', page, 'D/txns', async () => {
        await page.goto(`${base}/#b/${data.books.empty.id}/txns`, { waitUntil: 'domcontentloaded' });
        await settle(page);
        await page.waitForSelector('#view table tbody tr', { timeout: 15000 }).catch(() => {});
        const snap = await page.evaluate(() => {
          const v = document.querySelector('#view');
          const e = v ? v.querySelector('.empty') : null;
          return {
            children: v ? v.childElementCount : -1,
            empty: e ? e.innerText : '',
            rows: v ? v.querySelectorAll('table tbody tr').length : -1,
          };
        });
        const list = await api.transactions(base, token, data.books.empty.id);
        const items = Array.isArray(list) ? list : list.items || [];
        const pass = snap.children >= 1 && /没有符合条件的交易/.test(snap.empty) && items.length === 0;
        return pass
          ? ok(`空账本：#view.childElementCount=${snap.children}，空态=「${snap.empty.replace(/\n/g, ' ')}」，tbody ${snap.rows} 行；API 交易数=${items.length}`)
          : bad(`children=${snap.children} empty=「${snap.empty.replace(/\n/g, ' ')}」rows=${snap.rows} api=${items.length}`);
      });

      await step('D-TXNS-03', '交易总表：请求失败（route 500）有可见错误反馈且不是永久骨架屏', page, 'D/txns', async () => {
        const failRoute = /\/api\/v1\/books\/\d+\/transactions\?/;
        await page.route(failRoute, (route) => route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ code: 500, msg: 'D套件注入500', data: null }),
        }));
        await page.goto(txnsUrl, { waitUntil: 'domcontentloaded' });
        await page.waitForFunction(() => {
          const v = document.querySelector('#view');
          return !!v && (v.innerText || '').includes('交易加载失败');
        }, null, { timeout: 12000 }).catch(() => {});
        const viewText = await page.locator('#view').innerText().catch(() => '');
        const busy = await page.locator('#view [aria-busy="true"]').count();
        const toast = await page.locator('#toast').innerText().catch(() => '');
        const skeletonBars = await page.locator('#view .skel').count();
        await page.unroute(failRoute).catch(() => {});
        // 恢复 SPA 到正常态：同 hash 的 goto 不会重新加载文档，必须显式 reload
        await page.reload({ waitUntil: 'domcontentloaded' });
        await settle(page);
        const pass = /交易加载失败/.test(viewText) && /D套件注入500/.test(viewText)
          && busy === 0 && skeletonBars === 0 && /D套件注入500/.test(toast);
        return pass
          ? ok(`注入 500 → #view 可见文本=「${viewText.replace(/\n/g, ' ').slice(0, 180)}」；aria-busy 残留=${busy}，.skel 残留=${skeletonBars}（非永久骨架屏）；toast=「${toast}」`)
          : bad(`view=「${viewText.replace(/\n/g, ' ').slice(0, 180)}」busy=${busy} skel=${skeletonBars} toast=「${toast}」`);
      });

      await step('D-TXNS-04', '交易总表：行「编辑」可用', page, 'D/txns', async () => {
        await page.reload({ waitUntil: 'domcontentloaded' });
        await settle(page);
        const firstType = await page.locator('#view table tbody tr td:nth-child(3)').first().innerText().catch(() => '');
        const editCount = await page.locator('#view table tbody tr button', { hasText: '编辑' }).count();
        if (!editCount) return blocked('表格无「编辑」按钮，无法执行行编辑');
        await page.locator('#view table tbody tr button', { hasText: '编辑' }).first().click();
        const opened = await page.locator('#editDlg').evaluate((d) => d.open).catch(() => false);
        const eType = await page.locator('#eType').innerText().catch(() => '');
        const eAmount = await page.locator('#eAmount').inputValue().catch(() => '');
        await page.locator('#eCancel').click({ timeout: 3000 }).catch(() => {});
        const pass = opened === true && eType.trim() === firstType.trim() && eAmount.length > 0;
        return pass
          ? ok(`行「编辑」按钮数=${editCount} → #editDlg.open=${opened}，eType=「${eType}」与行类型「${firstType}」一致，eAmount=「${eAmount}」`)
          : bad(`按钮数=${editCount} opened=${opened} eType=「${eType}」行类型=「${firstType}」eAmount=「${eAmount}」`);
      });

      // I-TXNS-01..19 逐条真实重跑（与 C 套件共用同一配方，不整页 PASS、不硬编码 PASS）
      const txnsResult = await runTxnsControls({ page, base, data, out: runDir, shotDir: 'D/I-TXNS' });
      for (let i = 1; i <= 19; i += 1) {
        const id = `I-TXNS-${String(i).padStart(2, '0')}`;
        const r = txnsResult.R[id] || blocked('配方未返回该控件的独立判定');
        results.push({
          suite: 'D',
          id,
          name: `[D 重跑] ${id} · 交易总表控件`,
          status: r.status,
          detail: r.detail,
          evidence: (r.shots || []).map((p) => runDir.rel(p)),
        });
      }
      if (diag.length) runDir.writeJson('D/txns/diagnostics.json', diag.slice(0, 50));
    } catch (e) {
      results.push({ suite: 'D', id: 'D-TXNS-GROUP', name: '交易总表分组', status: 'FAIL', detail: `分组执行异常：${e && e.message ? e.message : e}`, evidence: [] });
    } finally {
      await ctx.close().catch(() => {});
    }
  }

  /* ---------------- B 组：记一笔分类下拉 ---------------- */
  {
    const { ctx, page } = await newSession(browser, data, 'owner');
    let categories = [];
    try {
      categories = await api.categories(base, token, mainId, false);
      await page.goto(`${base}/#b/${mainId}`, { waitUntil: 'domcontentloaded' });
      await settle(page);
      await page.locator('#fab').click();
      await page.waitForFunction(() => {
        const d = document.querySelector('#formDlg');
        return !!d && d.open;
      }, null, { timeout: 10000 });
      const incomeIds = categories.filter((c) => c.catType === 1).map((c) => String(c.id)).sort();
      const expenseIds = categories.filter((c) => c.catType === 2).map((c) => String(c.id)).sort();

      await step('D-CAT-01', '支出类型下拉只含支出分类', page, 'D/category', async () => {
        await page.selectOption('#fType', '2');
        const opts = await catOptions(page);
        if (!opts) return blocked('#fCategory 不存在');
        const values = opts.map((o) => o.value).sort();
        const leaked = values.filter((v) => incomeIds.includes(v));
        const pass = JSON.stringify(values) === JSON.stringify(expenseIds) && leaked.length === 0;
        return pass
          ? ok(`支出下拉 ${values.length} 项，与 API catType=2 的 ${expenseIds.length} 项完全一致；混入收入类=${leaked.length}；首项=「${opts[0] ? opts[0].text : ''}」(${opts[0] ? opts[0].value : ''})`)
          : bad(`支出下拉=${JSON.stringify(values)} 期望=${JSON.stringify(expenseIds)} 混入收入类=${JSON.stringify(leaked)}`);
      });

      await step('D-CAT-02', '收入类型下拉只含收入分类', page, 'D/category', async () => {
        await page.selectOption('#fType', '1');
        const opts = await catOptions(page);
        if (!opts) return blocked('#fCategory 不存在');
        const values = opts.map((o) => o.value).sort();
        const leaked = values.filter((v) => expenseIds.includes(v));
        const pass = JSON.stringify(values) === JSON.stringify(incomeIds) && leaked.length === 0;
        return pass
          ? ok(`收入下拉 ${values.length} 项，与 API catType=1 的 ${incomeIds.length} 项完全一致；混入支出类=${leaked.length}`)
          : bad(`收入下拉=${JSON.stringify(values)} 期望=${JSON.stringify(incomeIds)} 混入支出类=${JSON.stringify(leaked)}`);
      });

      await step('D-CAT-03', '类型切换后已选分类被正确重置、转账隐藏分类行', page, 'D/category', async () => {
        await page.selectOption('#fType', '2');
        const exp = await catOptions(page) || [];
        const lastExpense = exp.length ? exp[exp.length - 1].value : '';
        if (lastExpense) await page.locator('#fCategory').selectOption(lastExpense);
        const chosenExpense = await page.locator('#fCategory').inputValue();
        await page.selectOption('#fType', '1');
        const afterIncome = await page.locator('#fCategory').inputValue();
        await page.selectOption('#fType', '2');
        const afterExpense = await page.locator('#fCategory').inputValue();
        await page.selectOption('#fType', '3');
        const catHidden = await page.locator('#fCatRow').evaluate((e) => e.classList.contains('hidden')).catch(() => null);
        const toHidden = await page.locator('#fToRow').evaluate((e) => e.classList.contains('hidden')).catch(() => null);
        const pass = chosenExpense === lastExpense && afterIncome !== chosenExpense
          && incomeIds.includes(afterIncome) && expenseIds.includes(afterExpense)
          && catHidden === true && toHidden === false;
        return pass
          ? ok(`支出选中 ${chosenExpense} → 切收入重置为 ${afterIncome}(收入类) → 切回支出重置为 ${afterExpense}(支出类)；转账 #fCatRow.hidden=${catHidden} #fToRow.hidden=${toHidden}`)
          : bad(`选中支出=${chosenExpense} 切收入=${afterIncome} 切回支出=${afterExpense} catHidden=${catHidden} toHidden=${toHidden}`);
      });

      let createdNote = '';
      await step('D-CAT-04', '支出类型直接创建交易成功（2xx）且 API 回读 txType/categoryId 一致', page, 'D/category', async () => {
        await page.selectOption('#fType', '2');
        createdNote = `D-支出分类回归-${Date.now()}`;
        await page.fill('#fAmount', '12.34');
        await page.fill('#fNote', createdNote);
        const post = await capturePost(page, /\/api\/v1\/books\/\d+\/transactions$/, () => page.locator('#fSubmit').click());
        const list = await api.transactions(base, token, mainId);
        const items = Array.isArray(list) ? list : list.items || [];
        const created = items.find((t) => (t.note || t.comment || '').includes(createdNote));
        const pass = post.status >= 200 && post.status < 300
          && post.body.txType === 2 && expenseIds.includes(String(post.body.categoryId))
          && !!created && created.txType === 2 && expenseIds.includes(String(created.categoryId));
        return pass
          ? ok(`POST=${post.status} 请求体 txType=${post.body.txType} categoryId=${post.body.categoryId}(支出类)；API 回读命中备注「${createdNote}」，txType=${created && created.txType} categoryId=${created && created.categoryId}`)
          : bad(`POST=${post.status} body=${JSON.stringify(post.body)} API 命中=${!!created} txType=${created && created.txType} categoryId=${created && created.categoryId}；resp=${JSON.stringify(post.respBody).slice(0, 200)}`);
      });

      await step('D-CAT-05', '支出交易刷新页面后仍存在', page, 'D/category', async () => {
        await page.goto(txnsUrl, { waitUntil: 'domcontentloaded' });
        await settle(page);
        // hash-change 导航下 networkidle 可能在 SPA 异步渲染之前就返回，
        // 必须等到表格数据行（含第 6 个 td）出现的确定性 DOM 信号再读取。
        await page.waitForSelector('#view table tbody tr td:nth-child(6)', { timeout: 15000 }).catch(() => {});
        const notes = await page.locator('#view table tbody tr td:nth-child(6)').allTextContents();
        const list = await api.transactions(base, token, mainId);
        const items = Array.isArray(list) ? list : list.items || [];
        const still = items.some((t) => (t.note || t.comment || '').includes(createdNote));
        const inDom = notes.some((n) => n.includes(createdNote));
        const pass = still && inDom;
        return pass
          ? ok(`刷新后 API 回读仍含「${createdNote}」=${still}，交易总表备注列含该文本=${inDom}；备注列=${JSON.stringify(notes)}`)
          : bad(`API 仍在=${still} DOM 含=${inDom} 备注列=${JSON.stringify(notes)}`);
      });

      await step('D-CAT-06', '转账时分类行隐藏且提交 categoryId:0 语义不变', page, 'D/category', async () => {
        await page.goto(`${base}/#b/${mainId}`, { waitUntil: 'domcontentloaded' });
        await settle(page);
        await page.locator('#fab').click();
        await page.waitForFunction(() => { const d = document.querySelector('#formDlg'); return !!d && d.open; }, null, { timeout: 10000 });
        await page.selectOption('#fType', '3');
        const accs = await api.accounts(base, token, mainId);
        const from = accs[0];
        const to = accs.find((a) => a.id !== from.id);
        if (!to) return blocked('可用账户不足两个，无法执行转账');
        await page.locator('#fAccount').selectOption(String(from.id));
        await page.locator('#fTo').selectOption(String(to.id));
        const note = `D-转账语义-${Date.now()}`;
        await page.fill('#fAmount', '5.00');
        await page.fill('#fNote', note);
        const catHidden = await page.locator('#fCatRow').evaluate((e) => e.classList.contains('hidden')).catch(() => null);
        const post = await capturePost(page, /\/api\/v1\/books\/\d+\/transactions$/, () => page.locator('#fSubmit').click());
        const list = await api.transactions(base, token, mainId);
        const items = Array.isArray(list) ? list : list.items || [];
        const created = items.find((t) => (t.note || t.comment || '').includes(note));
        const pass = catHidden === true && post.status >= 200 && post.status < 300
          && post.body.txType === 3 && post.body.categoryId === 0
          && Number(post.body.toAccountId) === to.id && !!created;
        return pass
          ? ok(`#fCatRow.hidden=${catHidden}；POST=${post.status} 请求体 txType=${post.body.txType} categoryId=${post.body.categoryId} toAccountId=${post.body.toAccountId}；API 回读命中转账=${!!created}`)
          : bad(`catHidden=${catHidden} POST=${post.status} body=${JSON.stringify(post.body)} API 命中=${!!created}`);
      });
    } catch (e) {
      results.push({ suite: 'D', id: 'D-CAT-GROUP', name: '分类筛选分组', status: 'FAIL', detail: `分组执行异常：${e && e.message ? e.message : e}`, evidence: [] });
    } finally {
      await ctx.close().catch(() => {});
    }
  }

  /* ---------------- C 组：成员邀请权限 ---------------- */
  {
    const roleChecks = [
      { role: 'owner', id: 'D-MEM-01', name: 'owner 能看到并使用「邀请成员」入口' },
      { role: 'admin', id: 'D-MEM-02', name: 'admin 能看到并使用「邀请成员」入口' },
    ];
    for (const item of roleChecks) {
      // eslint-disable-next-line no-await-in-loop
      const { ctx, page } = await newSession(browser, data, item.role);
      try {
        // eslint-disable-next-line no-await-in-loop
        await step(item.id, item.name, page, 'D/members', async () => {
          await page.goto(`${base}/#b/${mainId}/members`, { waitUntil: 'domcontentloaded' });
          await settle(page);
          const count = await page.locator('#view button', { hasText: '邀请成员' }).count();
          let dlgOpen = false;
          if (count > 0) {
            await page.locator('#view button', { hasText: '邀请成员' }).first().click();
            dlgOpen = await page.locator('#investDlg').isVisible().catch(() => false);
          }
          const rows = await page.locator('#view table tbody tr').count();
          const pass = count === 1 && dlgOpen === true && rows >= 3;
          return pass
            ? ok(`role=${item.role}：按钮数=${count}，点击后 #investDlg 可见=${dlgOpen}，成员列表 ${rows} 行`)
            : bad(`role=${item.role}：按钮数=${count} 弹窗可见=${dlgOpen} 成员行数=${rows}`);
        });
      } catch (e) {
        results.push({ suite: 'D', id: item.id, name: item.name, status: 'FAIL', detail: `执行异常：${e && e.message ? e.message : e}`, evidence: [] });
      } finally {
        // eslint-disable-next-line no-await-in-loop
        await ctx.close().catch(() => {});
      }
    }

    {
      const { ctx, page } = await newSession(browser, data, 'member');
      try {
        await step('D-MEM-03', 'member 看不到「邀请成员」入口', page, 'D/members', async () => {
          await page.goto(`${base}/#b/${mainId}/members`, { waitUntil: 'domcontentloaded' });
          await settle(page);
          const count = await page.locator('#view button', { hasText: '邀请成员' }).count();
          const viewText = await page.locator('#view').innerText().catch(() => '');
          const bodyText = await page.locator('body').innerText().catch(() => '');
          const pass = count === 0 && !viewText.includes('邀请成员') && !bodyText.includes('邀请成员');
          return pass
            ? ok(`member：按钮数=${count}；#view 文本含「邀请成员」=${viewText.includes('邀请成员')}；body 文本含=${bodyText.includes('邀请成员')}`)
            : bad(`按钮数=${count}；#view 含=${viewText.includes('邀请成员')}；body 含=${bodyText.includes('邀请成员')}`);
        });

        await step('D-MEM-04', 'member 直接 POST invite 仍被服务端 403', page, 'D/members', async () => {
          const denied = await api.raw(base, 'POST', `/api/v1/books/${mainId}/members/invite`, {
            token: data.users.member.token, body: { role: 3 },
          });
          const pass = denied.status === 403;
          return pass
            ? ok(`member 直接 POST /books/${mainId}/members/invite → HTTP ${denied.status}（后端 403 校验保留）`)
            : bad(`HTTP ${denied.status}（期望 403）；响应=${String(denied.text).slice(0, 200)}`);
        });

        await step('D-MEM-05', '成员列表/角色展示不受影响', page, 'D/members', async () => {
          await page.goto(`${base}/#b/${mainId}/members`, { waitUntil: 'domcontentloaded' });
          await settle(page);
          const roles = await page.locator('#view table tbody tr td:nth-child(2)').allTextContents();
          const rows = await page.locator('#view table tbody tr').count();
          const removeCount = await page.locator('#view button', { hasText: '移除成员' }).count();
          const members = await api.members(base, token, mainId);
          const pass = rows === 3 && members.length === 3
            && roles.some((r) => r.includes('所有者')) && roles.some((r) => r.includes('管理员'))
            && roles.some((r) => r.includes('普通成员'));
          return pass
            ? ok(`成员列表 ${rows} 行（API ${members.length} 条），角色列=${JSON.stringify(roles)}；移除按钮=${removeCount} 个`)
            : bad(`rows=${rows} api=${members.length} roles=${JSON.stringify(roles)} remove=${removeCount}`);
        });

        await step('D-MEM-06', '/users/me 失败时失败关闭（owner 也不显示邀请按钮）', page, 'D/members', async () => {
          // 复用 owner 会话，故意让 /users/me 返回 500，验证不渲染入口
          const ownerCtx = await browser.newContext({ viewport: VIEWPORT, storageState: storageStateFor(data, 'owner') });
          const ownerPage = await ownerCtx.newPage();
          try {
            const meRoute = /\/api\/v1\/users\/me$/;
            await ownerPage.route(meRoute, (route) => route.fulfill({
              status: 500, contentType: 'application/json',
              body: JSON.stringify({ code: 500, msg: '注入 me 失败', data: null }),
            }));
            await ownerPage.goto(`${base}/#b/${mainId}/members`, { waitUntil: 'domcontentloaded' });
            await settle(ownerPage);
            const count = await ownerPage.locator('#view button', { hasText: '邀请成员' }).count();
            const rows = await ownerPage.locator('#view table tbody tr').count();
            const viewText = await ownerPage.locator('#view').innerText().catch(() => '');
            await ownerPage.unroute(meRoute).catch(() => {});
            const pass = count === 0 && !viewText.includes('邀请成员') && rows >= 3;
            return pass
              ? ok(`/users/me=500 → owner 页面邀请按钮数=${count}，#view 无该文本；成员列表仍渲染 ${rows} 行（失败关闭）`)
              : bad(`按钮数=${count} view 含邀请文本=${viewText.includes('邀请成员')} 成员行数=${rows}`);
          } finally {
            await ownerCtx.close().catch(() => {});
          }
        });
      } catch (e) {
        results.push({ suite: 'D', id: 'D-MEM-GROUP', name: '成员邀请分组', status: 'FAIL', detail: `分组执行异常：${e && e.message ? e.message : e}`, evidence: [] });
      } finally {
        await ctx.close().catch(() => {});
      }
    }
  }

  /* ---------------- D 组：新增账户（DOM 渲染一致性，不含视觉可见性） ---------------- */
  {
    const { ctx, page } = await newSession(browser, data, 'owner');
    const acctUrl = `${base}/#b/${mainId}/accounts`;
    let name = '';
    let beforeNames = [];
    let postStatus = 0;
    const visibilityLog = [];

    /**
     * 收紧裁决：每条用例名与 detail 都标注「DOM 渲染一致性，不含视觉可见性」，
     * 且每条都保留 visibleInnerText / firstTdDisplay 两个原始观测字段。
     * PASS 只代表 DOM 与 API 的数据一致性，绝不引用 I-ACCT-06、也不代表视觉可见性通过。
     */
    const acctStep = async (id, label, fn) => step(
      id,
      `${label}（DOM 渲染一致性，不含视觉可见性）`,
      page,
      'D/account',
      async () => {
        const r = await fn();
        const probe = await visibilityProbe(page, name);
        visibilityLog.push({ id, name, ...probe });
        const suffix = `；visibleInnerText=${probe.visibleInnerText}；firstTdDisplay=${probe.firstTdDisplay}；`
          + '【本组只断言 DOM 渲染一致性，不含视觉可见性；390px 视口下账户名称列被 style.css L331 隐藏，'
          + '视觉可见性验收项未闭环，待 Owner 裁决，本轮不改 CSS】';
        return { status: r.status, detail: r.detail + suffix };
      },
    );

    try {
      await page.goto(acctUrl, { waitUntil: 'domcontentloaded' });
      await settle(page);
      const beforeAccs = await api.accounts(base, token, mainId);
      beforeNames = beforeAccs.map((a) => a.name);
      const rowsBefore = await page.locator('#view table tbody tr').count();

      await acctStep('D-ACCT-01', '新增账户：POST 响应 + 重绘后确定性 DOM 信号，提交后数据进入列表', async () => {
        name = `D-账户回归-${Date.now()}`;
        await page.locator('#view button', { hasText: '＋ 新增账户' }).first().click();
        await page.waitForFunction(() => { const d = document.querySelector('#acctDlg'); return !!d && d.open; }, null, { timeout: 10000 });
        await page.fill('#acctName', name);
        const postPromise = page.waitForResponse(
          (r) => r.request().method() === 'POST' && /\/api\/v1\/books\/\d+\/accounts$/.test(r.url()),
          { timeout: 15000 },
        ).catch(() => null);
        await page.locator('#acctSubmit').click();
        const postResp = await postPromise;
        postStatus = postResp ? postResp.status() : 0;
        const redrawn = await page.waitForFunction(
          (n) => {
            const v = document.querySelector('#view');
            return !!v && v.querySelectorAll('table tbody tr').length === n;
          },
          rowsBefore + 1,
          { timeout: 10000 },
        ).then(() => true).catch(() => false);
        const domNames = await page.locator('#view table tbody tr td:first-child').allTextContents();
        const inDom = domNames.some((t) => t.trim() === name);
        const apiAccs = await api.accounts(base, token, mainId);
        const inApi = apiAccs.some((a) => a.name === name);
        const pass = postStatus >= 200 && postStatus < 300 && redrawn && inDom && inApi;
        return pass
          ? ok(`POST=${postStatus}，tbody ${rowsBefore}→${rowsBefore + 1}（重绘=${redrawn}），DOM 首列 textContent 命中=${inDom}，API 回读命中=${inApi}；DOM 首列=${JSON.stringify(domNames)}`)
          : bad(`POST=${postStatus} 重绘+1=${redrawn} DOM 命中=${inDom} API 命中=${inApi}；DOM 首列=${JSON.stringify(domNames)}`);
      });

      await acctStep('D-ACCT-02', '新增账户：刷新页面后数据仍存在', async () => {
        await page.reload({ waitUntil: 'domcontentloaded' });
        await settle(page);
        const domNames = await page.locator('#view table tbody tr td:first-child').allTextContents();
        const apiAccs = await api.accounts(base, token, mainId);
        const pass = domNames.some((t) => t.trim() === name) && apiAccs.some((a) => a.name === name);
        return pass
          ? ok(`刷新后 DOM 首列 ${domNames.length} 项含「${name}」，API ${apiAccs.length} 项含「${name}」`)
          : bad(`DOM 首列=${JSON.stringify(domNames)} API 名称=${JSON.stringify(apiAccs.map((a) => a.name))}`);
      });

      await acctStep('D-ACCT-03', '新增账户：账户总数只 +1（无重复）', async () => {
        const apiAccs = await api.accounts(base, token, mainId);
        const domNames = await page.locator('#view table tbody tr td:first-child').allTextContents();
        const apiHits = apiAccs.filter((a) => a.name === name).length;
        const domHits = domNames.filter((t) => t.trim() === name).length;
        const pass = apiAccs.length === beforeNames.length + 1 && apiHits === 1 && domHits === 1
          && domNames.length === beforeNames.length + 1;
        return pass
          ? ok(`账户总数 ${beforeNames.length}→${apiAccs.length}（+1）；「${name}」在 API 出现 ${apiHits} 次、DOM 首列出现 ${domHits} 次；DOM 行数=${domNames.length}`)
          : bad(`总数 ${beforeNames.length}→${apiAccs.length} apiHits=${apiHits} domHits=${domHits} domRows=${domNames.length}`);
      });

      await acctStep('D-ACCT-04', '新增账户：API 回读与 DOM 首列文本全量一致', async () => {
        const apiAccs = await api.accounts(base, token, mainId);
        const domNames = (await page.locator('#view table tbody tr td:first-child').allTextContents()).map((t) => t.trim());
        const apiNames = apiAccs.map((a) => a.name);
        const pass = JSON.stringify([...domNames].sort()) === JSON.stringify([...apiNames].sort());
        return pass
          ? ok(`DOM 首列=${JSON.stringify(domNames)} 与 API 名称集合=${JSON.stringify(apiNames)} 完全一致（390px 视口下 style.css L331 隐藏 td:nth-child(1)，故用 textContent 而非 innerText；该 CSS 缺陷不在本次授权范围，未修改）`)
          : bad(`DOM=${JSON.stringify(domNames)} API=${JSON.stringify(apiNames)}`);
      });
    } catch (e) {
      results.push({ suite: 'D', id: 'D-ACCT-GROUP', name: '新增账户分组（DOM 渲染一致性，不含视觉可见性）', status: 'FAIL', detail: `分组执行异常：${e && e.message ? e.message : e}`, evidence: [] });
    } finally {
      try { runDir.writeJson('D/account/visibility-observations.json', visibilityLog); } catch (_) { /* ignore */ }
      await ctx.close().catch(() => {});
    }
  }

  runDir.writeJson('D/defects-summary.json', {
    total: results.length,
    tally: results.reduce((a, r) => { a[r.status] = (a[r.status] || 0) + 1; return a; }, {}),
    note: '第 4 项业务代码零改动；新增账户组仅断言 DOM 渲染一致性（不含视觉可见性），视觉可见性验收项未闭环、待 Owner 裁决 style.css L331；C 套件 I-ACCT-06 保持 FAIL，未被本组改判。',
  });
  return results;
}

module.exports = { runDefects };
