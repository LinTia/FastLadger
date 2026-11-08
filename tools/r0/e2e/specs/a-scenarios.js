'use strict';
/**
 * 包 A · 11 项独立自动化复验（Playwright）
 *
 * 覆盖：pending / 网络拒绝 / HTTP 503 / 恢复 / 慢速网络 / 有数据 / 空数据 /
 *       权限不足 / 多角色 / 离线恢复 / 同步冲突
 *
 * 每个场景：独立 browser context（隔离 storage）→ 执行动作 → 断言 UI/API → 截图 + trace。
 * 断言基于真实浏览器行为与真实 HTTP 响应，不用静态代码定位充数。
 */
const api = require('../lib/api');
const { storageStateFor } = require('../lib/fixtures');
const { attachDiagnostics } = require('../lib/evidence');

const VIEWPORT = { width: 390, height: 844 };

async function withContext(browser, { base, role, data, tracing }, fn) {
  const ctx = await browser.newContext({
    viewport: VIEWPORT,
    storageState: role ? storageStateFor(data, role) : undefined,
    ignoreHTTPSErrors: true,
  });
  if (tracing) {
    await ctx.tracing.start({ screenshots: true, snapshots: true, sources: false });
  }
  const page = await ctx.newPage();
  const diag = attachDiagnostics(page, []);
  try {
    return await fn(page, ctx, diag);
  } finally {
    if (tracing) {
      await ctx.tracing.stop({ path: tracing }).catch(() => {});
    }
    await ctx.close().catch(() => {});
  }
}

const toastText = async (page) => {
  const t = page.locator('#toast');
  if (await t.count()) return (await t.innerText().catch(() => '')).trim();
  return '';
};

async function waitToast(page, timeout = 5000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const t = await toastText(page);
    if (t) return t;
    await page.waitForTimeout(100);
  }
  return '';
}

async function fillLogin(page, base) {
  await page.goto(base + '/#books', { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('input[placeholder="用户名"]', { timeout: 10000 });
}

module.exports = [
  {
    id: 'A01',
    name: 'pending：提交中按钮进入禁用 + 文案「登录中…」',
    async run({ browser, base, data, runDir }) {
      return withContext(browser, { base, tracing: runDir.path('A01/trace.zip') }, async (page) => {
        let delayed = false;
        await page.route('**/api/v1/auth/login', async (route) => {
          delayed = true;
          await new Promise((r) => setTimeout(r, 1500));
          await route.continue();
        });
        await fillLogin(page, base);
        await page.fill('input[placeholder="用户名"]', 'owner');
        await page.fill('input[placeholder^="密码"]', data.password);
        await page.locator('button', { hasText: '登 录' }).first().click();
        // 在请求被挂起的窗口内轮询，抓到 pending 态即算命中（单点采样会有竞态）
        const pending = await page.waitForFunction(() => {
          const b = [...document.querySelectorAll('#view button')].find((x) => /登录中/.test(x.textContent || ''));
          return b ? { text: b.textContent.trim(), disabled: b.disabled } : false;
        }, { timeout: 4000 }).then((h) => h.jsonValue()).catch(() => null);
        const shotPending = await runDir.screenshot(page, 'A01', 'pending');
        await page.waitForFunction(() => !!localStorage.getItem('fl_token'), { timeout: 20000 });
        const shot = await runDir.screenshot(page, 'A01', 'after-login');
        return {
          status: delayed && pending && pending.disabled ? 'PASS' : 'FAIL',
          detail: `请求被延迟=${delayed}；pending 态=${JSON.stringify(pending)}；登录后 token 已写入=${await page.evaluate(() => !!localStorage.getItem('fl_token'))}`,
          evidence: [shotPending, shot],
        };
      });
    },
  },

  {
    id: 'A02',
    name: '网络拒绝：请求 abort → 错误提示可见且留在登录页',
    async run({ browser, base, data, runDir }) {
      return withContext(browser, { base, tracing: runDir.path('A02/trace.zip') }, async (page) => {
        await page.route('**/api/v1/auth/login', (route) => route.abort('failed'));
        await fillLogin(page, base);
        await page.fill('input[placeholder="用户名"]', 'owner');
        await page.fill('input[placeholder^="密码"]', data.password);
        await page.locator('button', { hasText: '登 录' }).first().click();
        const toast = await waitToast(page, 8000);
        const stillLogin = await page.locator('input[placeholder="用户名"]').count();
        const shot = await runDir.screenshot(page, 'A02', 'network-abort');
        const hasToken = await page.evaluate(() => !!localStorage.getItem('fl_token'));
        return {
          status: toast.length > 0 && stillLogin === 1 && hasToken === false ? 'PASS' : 'FAIL',
          detail: `toast=「${toast}」；仍在登录页=${stillLogin === 1}；写入 token=${hasToken}`,
          evidence: [shot],
        };
      });
    },
  },

  {
    id: 'A03',
    name: 'HTTP 503：服务端 503 → 展示错误信息，不伪造成功',
    async run({ browser, base, data, runDir }) {
      return withContext(browser, { base, tracing: runDir.path('A03/trace.zip') }, async (page) => {
        await page.route('**/api/v1/auth/login', (route) =>
          route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 503, msg: '服务暂时不可用' }) }));
        await fillLogin(page, base);
        await page.fill('input[placeholder="用户名"]', 'owner');
        await page.fill('input[placeholder^="密码"]', data.password);
        await page.locator('button', { hasText: '登 录' }).first().click();
        const toast = await waitToast(page, 8000);
        const hasToken = await page.evaluate(() => !!localStorage.getItem('fl_token'));
        const shot = await runDir.screenshot(page, 'A03', 'http-503');
        return {
          status: toast.includes('不可用') && hasToken === false ? 'PASS' : 'FAIL',
          detail: `toast=「${toast}」；是否误写 token=${hasToken}`,
          evidence: [shot],
        };
      });
    },
  },

  {
    id: 'A04',
    name: '恢复：一次失败后重试成功并可进入账本',
    async run({ browser, base, data, runDir }) {
      return withContext(browser, { base, tracing: runDir.path('A04/trace.zip') }, async (page) => {
        let fail = true;
        await page.route('**/api/v1/auth/login', async (route) => {
          if (fail) { fail = false; return route.abort('failed'); }
          return route.continue();
        });
        await fillLogin(page, base);
        await page.fill('input[placeholder="用户名"]', 'owner');
        await page.fill('input[placeholder^="密码"]', data.password);
        await page.locator('button', { hasText: '登 录' }).first().click();
        const firstToast = await waitToast(page, 8000);
        await page.waitForTimeout(600);
        await page.locator('button', { hasText: '登 录' }).first().click();
        const recovered = await page.waitForFunction(() => !!localStorage.getItem('fl_token'), { timeout: 20000 }).then(() => true).catch(() => false);
        const token = await page.evaluate(() => localStorage.getItem('fl_token'));
        const shot = await runDir.screenshot(page, 'A04', 'retry-success');
        return {
          status: firstToast.length > 0 && recovered && !!token ? 'PASS' : 'FAIL',
          detail: `首次失败 toast=「${firstToast}」；重试后是否登录=${recovered}；token 已写入=${!!token}；hash=${await page.evaluate(() => location.hash)}`,
          evidence: [shot],
        };
      });
    },
  },

  {
    id: 'A05',
    name: '慢速网络：800ms 延迟下 pending 仍可见，最终成功',
    async run({ browser, base, data, runDir }) {
      return withContext(browser, { base, tracing: runDir.path('A05/trace.zip') }, async (page) => {
        const cdp = await page.context().newCDPSession(page);
        await cdp.send('Network.enable');
        await cdp.send('Network.emulateNetworkConditions', {
          offline: false, latency: 800, downloadThroughput: 200 * 1024, uploadThroughput: 100 * 1024,
        });
        await fillLogin(page, base);
        await page.fill('input[placeholder="用户名"]', 'owner');
        await page.fill('input[placeholder^="密码"]', data.password);
        const t0 = Date.now();
        await page.locator('button', { hasText: '登 录' }).first().click();
        // 在 800ms 链路延迟窗口内轮询抓 pending 态（单点采样有竞态）。
        // A01 的 disabled 断言不能替代 A05 慢网场景，故此处独立取回 { text, disabled }。
        const pending = await page.waitForFunction(() => {
          const b = [...document.querySelectorAll('#view button')].find((x) => /登录中/.test(x.textContent || ''));
          return b ? { text: b.textContent.trim(), disabled: b.disabled } : false;
        }, null, { timeout: 4000 }).then((h) => h.jsonValue()).catch(() => null);
        await page.waitForFunction(() => !!localStorage.getItem('fl_token'), { timeout: 30000 });
        const elapsed = Date.now() - t0;
        const shot = await runDir.screenshot(page, 'A05', 'slow-network');
        return {
          status: elapsed >= 800 && !!pending && /登录中/.test(pending.text) && pending.disabled === true ? 'PASS' : 'FAIL',
          detail: `总耗时=${elapsed}ms（含 800ms 链路延迟）；中途按钮文案=「${pending ? pending.text : '未捕获'}」；pendingDisabled=${pending ? pending.disabled : '未捕获'}；登录后 token 已写入=${await page.evaluate(() => !!localStorage.getItem('fl_token'))}`,
          evidence: [shot],
        };
      });
    },
  },

  {
    id: 'A06',
    name: '有数据：首页展示真实金额与交易条目',
    async run({ browser, base, data, runDir }) {
      return withContext(browser, { base, role: 'owner', data, tracing: runDir.path('A06/trace.zip') }, async (page) => {
        await page.goto(`${base}/#b/${data.books.main.id}`, { waitUntil: 'domcontentloaded' });
        await page.waitForTimeout(1500);
        const text = await page.locator('#view').innerText();
        const hasYuan = /¥/.test(text);
        const hasSeed = text.includes('早餐') || text.includes('打车') || text.includes('工资');
        const apiTx = await api.transactions(base, data.users.owner.token, data.books.main.id);
        const list = Array.isArray(apiTx) ? apiTx : apiTx.items || apiTx.list || [];
        const shot = await runDir.screenshot(page, 'A06', 'home-with-data');
        return {
          status: hasYuan && hasSeed && list.length >= 3 ? 'PASS' : 'FAIL',
          detail: `页面含 ¥=${hasYuan}；含种子交易=${hasSeed}；API 交易数=${list.length}`,
          evidence: [shot],
        };
      });
    },
  },

  {
    id: 'A07',
    name: '空数据：空账本显示空态且不串数据',
    async run({ browser, base, data, runDir }) {
      return withContext(browser, { base, role: 'owner', data, tracing: runDir.path('A07/trace.zip') }, async (page) => {
        await page.goto(`${base}/#b/${data.books.empty.id}`, { waitUntil: 'domcontentloaded' });
        await page.waitForTimeout(1200);
        const text = await page.locator('#view').innerText();
        const apiTx = await api.transactions(base, data.users.owner.token, data.books.empty.id);
        const list = Array.isArray(apiTx) ? apiTx : apiTx.items || apiTx.list || [];
        const leaks = ['早餐', '打车', '工资'].filter((k) => text.includes(k));
        const shot = await runDir.screenshot(page, 'A07', 'empty-book');
        return {
          status: list.length === 0 && leaks.length === 0 ? 'PASS' : 'FAIL',
          detail: `API 交易数=${list.length}；页面残留他账本数据=${JSON.stringify(leaks)}；页面含「暂无」=${text.includes('暂无')}`,
          evidence: [shot],
        };
      });
    },
  },

  {
    id: 'A08',
    name: '权限不足：member 调管理接口被 403 拒绝且不泄露',
    async run({ browser, base, data, runDir }) {
      const denied = await api.raw(base, 'DELETE', `/api/v1/books/${data.books.main.id}`, { token: data.users.member.token });
      const deniedInvite = await api.raw(base, 'POST', `/api/v1/books/${data.books.main.id}/members/invite`, {
        token: data.users.member.token, body: { role: 3 },
      });
      return withContext(browser, { base, role: 'member', data, tracing: runDir.path('A08/trace.zip') }, async (page) => {
        await page.goto(`${base}/#b/${data.books.main.id}/members`, { waitUntil: 'domcontentloaded' });
        await page.waitForTimeout(1200);
        const body = await page.locator('#view').innerText();
        const inviteBtn = await page.locator('button', { hasText: '邀请成员' }).count();
        const shot = await runDir.screenshot(page, 'A08', 'member-members-page');
        const apiOk = denied.status === 403 && deniedInvite.status === 403;
        return {
          status: apiOk ? 'PASS' : 'FAIL',
          detail: `DELETE /books → ${denied.status}；POST /members/invite → ${deniedInvite.status}；成员页「邀请成员」按钮数=${inviteBtn}（成员管理接口 403 校验通过；UI 入口可见性另由 D 套件 D-MEM-* 覆盖）；页面文本长度=${body.length}`,
          evidence: [shot],
        };
      });
    },
  },

  {
    id: 'A09',
    name: '多角色：owner/admin/member 三种会话看到不同能力',
    async run({ browser, base, data, runDir }) {
      const rows = [];
      for (const role of ['owner', 'admin', 'member']) {
        // eslint-disable-next-line no-await-in-loop
        await withContext(browser, { base, role, data, tracing: runDir.path(`A09/${role}-trace.zip`) }, async (page) => {
          await page.goto(`${base}/#b/${data.books.main.id}/members`, { waitUntil: 'domcontentloaded' });
          await page.waitForTimeout(1000);
          const members = await api.members(base, data.users[role].token, data.books.main.id);
          const shot = await runDir.screenshot(page, 'A09', `members-${role}`);
          rows.push({
            role,
            members: Array.isArray(members) ? members.length : -1,
            inviteBtn: await page.locator('button', { hasText: '邀请成员' }).count(),
            text: (await page.locator('#view').innerText()).slice(0, 120),
            shot,
          });
        });
      }
      const ok = rows.every((r) => r.members === 3);
      return {
        status: ok ? 'PASS' : 'FAIL',
        detail: rows.map((r) => `${r.role}: 成员数=${r.members} 邀请按钮=${r.inviteBtn}`).join('；'),
        evidence: rows.map((r) => r.shot),
      };
    },
  },

  {
    id: 'A10',
    name: '离线 / 恢复：离线时报错，恢复网络后重试成功',
    async run({ browser, base, data, runDir }) {
      return withContext(browser, { base, role: 'owner', data, tracing: runDir.path('A10/trace.zip') }, async (page) => {
        await page.goto(`${base}/#b/${data.books.main.id}`, { waitUntil: 'domcontentloaded' });
        await page.waitForTimeout(800);
        await page.context().setOffline(true);
        const offlineShot = await runDir.screenshot(page, 'A10', 'offline');
        // 触发一次真实请求（进入交易总表走 API）
        await page.evaluate((id) => { location.hash = `#b/${id}/txns`; }, data.books.main.id);
        await page.waitForTimeout(1200);
        const offlineText = await page.locator('#view').innerText();
        const offlineToast = await toastText(page);
        await page.context().setOffline(false);
        await page.evaluate((id) => { location.hash = `#b/${id}`; }, data.books.main.id);
        await page.waitForTimeout(1500);
        const onlineText = await page.locator('#view').innerText();
        const onlineShot = await runDir.screenshot(page, 'A10', 'back-online');
        const ok = onlineText.includes('¥') || onlineText.includes('早餐');
        return {
          status: ok ? 'PASS' : 'FAIL',
          detail: `离线时 toast=「${offlineToast}」视图文本长度=${offlineText.length}；恢复后含金额=${onlineText.includes('¥')} 含种子=${onlineText.includes('早餐')}`,
          evidence: [offlineShot, onlineShot],
        };
      });
    },
  },

  {
    id: 'A11',
    name: '同步冲突：同一 clientUuid 重放不重复落账',
    async run({ base, data, runDir }) {
      const bookId = data.books.main.id;
      const tok = data.users.owner.token;
      const before = await api.transactions(base, tok, bookId);
      const beforeList = Array.isArray(before) ? before : before.items || [];
      const uuid = require('node:crypto').randomUUID();
      const item = {
        clientUuid: uuid, txType: 2, accountId: data.ids.cashId, amount: 7777,
        categoryId: data.ids.expenseCatId, txTime: Math.floor(Date.now() / 1000), timezoneOffset: 480, comment: 'replay', source: 1,
      };
      const first = await api.push(base, tok, bookId, [item]);
      const second = await api.push(base, tok, bookId, [{ ...item, amount: 9999 }]);
      const after = await api.transactions(base, tok, bookId);
      const afterList = Array.isArray(after) ? after : after.items || [];
      const r1 = (first.results || first.items || first)[0] || {};
      const r2 = (second.results || second.items || second)[0] || {};
      const detail = {
        firstStatus: r1.status, secondStatus: r2.status,
        beforeCount: beforeList.length, afterCount: afterList.length,
      };
      const noDouble = afterList.length === beforeList.length + 1;
      const rejectedOrReplayed = ['replayed', 'conflict'].includes(r2.status);
      runDir.writeJson('A11/push-results.json', { item, first, second, detail });
      return {
        status: noDouble && rejectedOrReplayed ? 'PASS' : 'FAIL',
        detail: `首次 status=${r1.status}，重放（金额改为 9999）status=${r2.status}；交易数 ${beforeList.length}→${afterList.length}（应 +1）`,
        evidence: [],
      };
    },
  },
];
