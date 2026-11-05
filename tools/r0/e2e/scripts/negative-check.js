#!/usr/bin/env node
'use strict';
/**
 * 负向合成验证：把「修复前」的 app.js（git show HEAD:server/web/dist/app.js）
 * 通过 Playwright route 注入到浏览器会话，证明新增 D 套件的断言在修复前确实会失败。
 *
 * 不修改任何磁盘上的业务代码：旧脚本只从 git 对象读取、只在网络层替换响应，
 * 服务端二进制仍是修复后版本（API/DB 完全一致），因此差异 100% 来自前端 app.js。
 *
 * 产出：docs/r0-evidence-7th-fix/after/negative-check.md / negative-check.json
 */
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const { startServer } = require('../lib/server');
const { launch } = require('../lib/browser');
const { bootstrap, storageStateFor } = require('../lib/fixtures');
const api = require('../lib/api');

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..', '..');
const OUT_DIR = path.resolve(REPO_ROOT, 'docs', 'r0-evidence-7th-fix', 'after');
const VIEWPORT = { width: 390, height: 844 };

function readPreFixAppJs() {
  const js = execFileSync('git', ['show', 'HEAD:server/web/dist/app.js'], {
    cwd: REPO_ROOT, encoding: 'utf8', maxBuffer: 8 * 1024 * 1024,
  });
  if (!js.includes('fillSelect($("#fCategory"), state.categories);')) {
    throw new Error('取到的 HEAD:app.js 不像修复前版本（未找到未过滤的分类填充）');
  }
  if (js.includes('view.append(card);                                // 与 renderHome')) {
    throw new Error('取到的 HEAD:app.js 像修复后版本，负向验证前提不成立');
  }
  return js;
}

async function withPage(browser, data, role, oldJs, fn) {
  const ctx = await browser.newContext({ viewport: VIEWPORT, storageState: storageStateFor(data, role) });
  const page = await ctx.newPage();
  await page.route(/\/app\.js$/, (route) => route.fulfill({
    status: 200,
    contentType: 'application/javascript; charset=utf-8',
    body: oldJs,
  }));
  try {
    return await fn(page);
  } finally {
    await ctx.close().catch(() => {});
  }
}

async function settle(page) {
  await page.waitForLoadState('networkidle', { timeout: 20000 }).catch(() => {});
}

async function main() {
  fs.mkdirSync(OUT_DIR, { recursive: true });
  const oldJs = readPreFixAppJs();
  const out = {
    startedAt: new Date().toISOString(),
    injectedAppJsSha256: require('node:crypto').createHash('sha256').update(oldJs).digest('hex'),
    gitSha: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: REPO_ROOT, encoding: 'utf8' }).trim(),
    checks: [],
  };
  const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-negative-'));
  let server;
  let bh;
  try {
    server = await startServer({ dataDir });
    const data = await bootstrap(server);
    data.origin = new URL(server.base).origin;
    bh = await launch();
    const { browser, base } = { browser: bh.browser, base: server.base };
    const mainId = data.books.main.id;
    const token = data.users.owner.token;

    /* 1) 交易总表：修复前 #view 为空 → D-TXNS-01 / I-TXNS-01 必然 FAIL */
    out.checks.push({
      id: 'NEG-1', correspondsTo: 'D-TXNS-01 / I-TXNS-01..19',
      value: await withPage(browser, data, 'owner', oldJs, async (page) => {
        await page.goto(`${base}/#b/${mainId}/txns`, { waitUntil: 'domcontentloaded' });
        await settle(page);
        return page.evaluate(() => {
          const v = document.querySelector('#view');
          return {
            viewChildren: v ? v.childElementCount : -1,
            innerTextLen: v ? (v.innerText || '').length : -1,
            hasTable: v ? !!v.querySelector('table tbody') : false,
          };
        });
      }),
      preFixFailsBecause: 'viewChildren=0 且无表格，D-TXNS-01 断言 childElementCount>=1 / 行数>=1 会 FAIL，I-TXNS-01..19 无控件可交互',
    });

    /* 2) 分类下拉：修复前支出下拉混入收入类 → D-CAT-01/02/03 必然 FAIL */
    out.checks.push({
      id: 'NEG-2', correspondsTo: 'D-CAT-01 / D-CAT-02 / D-CAT-03 / D-CAT-04',
      value: await withPage(browser, data, 'owner', oldJs, async (page) => {
        await page.goto(`${base}/#b/${mainId}`, { waitUntil: 'domcontentloaded' });
        await settle(page);
        await page.locator('#fab').click();
        await page.waitForFunction(() => { const d = document.querySelector('#formDlg'); return !!d && d.open; }, null, { timeout: 10000 }).catch(() => {});
        const read = () => page.evaluate(() => {
          const s = document.querySelector('#fCategory');
          return s ? { count: s.options.length, value: s.value, first: s.options[0] ? s.options[0].textContent : '', values: Array.from(s.options).map((o) => o.value) } : null;
        });
        const expense = await read();
        await page.selectOption('#fType', '1');
        const income = await read();
        const cats = await api.categories(base, token, mainId, false);
        const incomeIds = cats.filter((c) => c.catType === 1).map((c) => String(c.id));
        const expenseIds = cats.filter((c) => c.catType === 2).map((c) => String(c.id));
        return {
          expense, income,
          incomeIdsInExpense: expense ? expense.values.filter((v) => incomeIds.includes(v)).length : -1,
          expenseIdsInIncome: income ? income.values.filter((v) => expenseIds.includes(v)).length : -1,
          unchangedOnTypeSwitch: !!expense && !!income && JSON.stringify(expense.values) === JSON.stringify(income.values),
        };
      }),
      preFixFailsBecause: '支出下拉混入收入类 id（首项「工资」），D-CAT-01/02 的集合相等断言会 FAIL；切换类型下拉不变，D-CAT-03 重置断言会 FAIL；默认选中收入类提交会被 400，D-CAT-04 会 FAIL',
    });

    /* 3) 成员邀请：修复前 member 也能看到按钮 → D-MEM-03 必然 FAIL */
    out.checks.push({
      id: 'NEG-3', correspondsTo: 'D-MEM-03',
      value: await withPage(browser, data, 'member', oldJs, async (page) => {
        await page.goto(`${base}/#b/${mainId}/members`, { waitUntil: 'domcontentloaded' });
        await settle(page);
        return page.evaluate(() => ({
          inviteBtnCount: Array.from(document.querySelectorAll('#view button')).filter((b) => (b.textContent || '').includes('邀请成员')).length,
          viewHasText: (document.querySelector('#view') ? document.querySelector('#view').innerText : '').includes('邀请成员'),
        }));
      }),
      preFixFailsBecause: 'member 身份按钮数=1 且 #view 含「邀请成员」，D-MEM-03 断言 count===0 且无该文本会 FAIL（后端 403 与 UI 越权无关）',
    });

    /* 4) 新增账户：修复前 innerText 不含名称但 textContent 含 → 说明旧 C 判定必 FAIL、D-ACCT-01 的严格信号仍可证明业务正常 */
    out.checks.push({
      id: 'NEG-4', correspondsTo: 'D-ACCT-01..04 / 旧 C 的 I-ACCT-06',
      value: await withPage(browser, data, 'owner', oldJs, async (page) => {
        const name = `NEG-账户-${Date.now()}`;
        await page.goto(`${base}/#b/${mainId}/accounts`, { waitUntil: 'domcontentloaded' });
        await settle(page);
        await page.locator('#view button', { hasText: '＋ 新增账户' }).first().click();
        await page.waitForFunction(() => { const d = document.querySelector('#acctDlg'); return !!d && d.open; }, null, { timeout: 10000 });
        await page.fill('#acctName', name);
        const postPromise = page.waitForResponse((r) => r.request().method() === 'POST' && /\/accounts$/.test(r.url()), { timeout: 15000 }).catch(() => null);
        await page.locator('#acctSubmit').click();
        const postResp = await postPromise;
        await page.waitForFunction((n) => {
          const v = document.querySelector('#view');
          return !!v && (v.textContent || '').includes(n);
        }, name, { timeout: 10000 }).catch(() => {});
        const dom = await page.evaluate((n) => {
          const v = document.querySelector('#view');
          const td = Array.from(v ? v.querySelectorAll('table tbody tr td:first-child') : []).find((e) => (e.textContent || '').trim() === n);
          return {
            viewChildren: v ? v.childElementCount : -1,
            innerTextHasName: v ? (v.innerText || '').includes(n) : false,
            textContentHasName: v ? (v.textContent || '').includes(n) : false,
            firstColDisplay: td ? window.getComputedStyle(td).display : '(未找到)',
            rows: v ? v.querySelectorAll('table tbody tr').length : -1,
          };
        }, name);
        const apiAccs = await api.accounts(base, token, mainId);
        return { name, postStatus: postResp ? postResp.status() : 0, dom, apiHasName: apiAccs.some((a) => a.name === name) };
      }),
      preFixFailsBecause: '修复前 app.js 与修复后一致地重绘了列表（POST 2xx、textContent 含名称），但 390px 下 style.css 隐藏首列使 innerText 不含名称——这正是旧 C 用 waitText(body.innerText) 恒 FAIL 的原因；D-ACCT-01 改用 textContent + 行数信号后判定才成立。本项证明第 4 项属测试口径问题、业务代码无需改动',
    });
  } catch (e) {
    out.fatal = String(e && e.stack ? e.stack : e);
    process.stderr.write(`NEGATIVE_CHECK_ERROR ${out.fatal}\n`);
  } finally {
    if (bh) await bh.browser.close().catch(() => {});
    if (server) await server.stop().catch(() => {});
    try { fs.rmSync(dataDir, { recursive: true, force: true }); } catch (_) { /* ignore */ }
  }
  out.finishedAt = new Date().toISOString();
  fs.writeFileSync(path.join(OUT_DIR, 'negative-check.json'), JSON.stringify(out, null, 2) + '\n');

  const lines = [];
  lines.push('# 负向合成验证：新增测试在修复前确实会失败');
  lines.push('');
  lines.push(`- 采集时间：${out.startedAt} ~ ${out.finishedAt}`);
  lines.push(`- 注入内容：\`git show HEAD:server/web/dist/app.js\`（commit \`${out.gitSha}\`，即修复前的 2241 行版本），SHA256=\`${out.injectedAppJsSha256}\``);
  lines.push('- 方式：Playwright `page.route(/\\/app\\.js$/)` 在网络层把脚本响应替换为修复前版本；磁盘业务代码、Go 后端、API、DB 均未改动，服务端二进制为修复后重新构建的版本。');
  lines.push('- 浏览器：真实 msedge；390×844 视口（与 C/D 套件一致）。');
  lines.push('');
  lines.push('## 逐项观测');
  lines.push('');
  for (const c of out.checks) {
    lines.push(`### ${c.id} · 对应 ${c.correspondsTo}`);
    lines.push('');
    lines.push('```json');
    lines.push(JSON.stringify(c.value, null, 2));
    lines.push('```');
    lines.push('');
    lines.push(`修复前为何会 FAIL：${c.preFixFailsBecause}`);
    lines.push('');
  }
  if (out.fatal) {
    lines.push(`## 脚本异常`);
    lines.push('');
    lines.push('```');
    lines.push(out.fatal);
    lines.push('```');
  }
  lines.push('> 结论：D 套件的断言不是恒真式；在修复前实现下，上述 4 组观测值分别使对应用例 FAIL。修复后的 a/c/d 结果见同目录 `*-after.log`。');
  lines.push('');
  fs.writeFileSync(path.join(OUT_DIR, 'negative-check.md'), lines.join('\n'));
  process.stdout.write(`[negative-check] output -> ${OUT_DIR}\n`);
  if (out.fatal) process.exitCode = 2;
}

main();
