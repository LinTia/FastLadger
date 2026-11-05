#!/usr/bin/env node
'use strict';
/**
 * QA 交付补充证据：生成一张清晰可读的「交易加载失败」截图。
 *
 * 边界（team-lead 本轮授权）：
 *   - 独立干净实例（临时 SQLite + 随机端口 + 每次新生成 JWT 密钥）；
 *   - 不修改磁盘上的任何业务代码 / CSS / Go 后端 / 既有测试代码；
 *   - 不放宽、不新增、不修改任何 A/D/C 套件判定；脚本内的检查只是「截图是否合格」的自检；
 *   - 不重跑 A/D/C 全套；不 push、不 commit。
 *
 * 流程：在网络层 route 注入 GET /transactions? 的 HTTP 500（全程不 unroute）→
 *       进入交易总表 → 等 #view.innerText 出现「交易加载失败」→
 *       确认无 dialog[open] 遮挡、骨架屏已清除、.empty 已布局 →
 *       用 Playwright 官方 animations:'disabled' 跳过 CSS 过渡态后截图。
 *
 * 产出：
 *   docs/r0-evidence-7th-fix/qa/screenshots/05b-txns-load-failure-clear.png
 *   docs/r0-evidence-7th-fix/qa/05b-shot.log
 */
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { startServer } = require('../lib/server');
const { launch } = require('../lib/browser');
const { bootstrap, storageStateFor } = require('../lib/fixtures');

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..', '..');
const OUT_DIR = path.resolve(REPO_ROOT, 'docs', 'r0-evidence-7th-fix', 'qa');
const SHOT_PATH = path.join(OUT_DIR, 'screenshots', '05b-txns-load-failure-clear.png');
const LOG_PATH = path.join(OUT_DIR, '05b-shot.log');
const VIEWPORT = { width: 390, height: 844 };
const INJECT_MSG = 'QA 交付截图注入 500';
const FAIL_ROUTE = /\/api\/v1\/books\/\d+\/transactions\?/;

const lines = [];
function log(s) {
  lines.push(s);
  process.stdout.write(`${s}\n`);
}

async function main() {
  log('=== QA 补充证据：清晰「交易加载失败」截图（不重跑全套、不改任何判定）===');
  log(`开始(UTC): ${new Date().toISOString()}`);
  log(`视口: ${JSON.stringify(VIEWPORT)}；注入消息: ${INJECT_MSG}`);
  fs.mkdirSync(path.dirname(SHOT_PATH), { recursive: true });

  const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-qa-shot-'));
  let server;
  let bh;
  let rc = 0;
  try {
    server = await startServer({ dataDir });
    log(`实例: ${server.base}（临时 SQLite，随机端口，每次新生成 JWT 密钥；与既有 artifacts 隔离）`);

    const data = await bootstrap(server);
    // storageStateFor 依赖 ctx.origin 写入 localStorage 登录态。
    data.origin = new URL(server.base).origin;

    bh = await launch();
    log(`浏览器: ${bh.from} channel=${bh.channel} version=${bh.version}`);

    const ctx = await bh.browser.newContext({
      viewport: VIEWPORT,
      storageState: storageStateFor(data, 'owner'),
    });
    const page = await ctx.newPage();

    // 网络层注入 500：必须在首次导航前设置，且全程不 unroute，
    // 保证截图时页面停留在失败态（既有 D-TXNS-03 截图错配正源于 fn 末尾的 reload 复位）。
    await page.route(FAIL_ROUTE, (route) => route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ code: 500, msg: INJECT_MSG, data: null }),
    }));

    await page.goto(`${server.base}/#b/${data.books.main.id}/txns`, { waitUntil: 'domcontentloaded' });

    // 确定性等待：错误文本出现 + 无 dialog 遮挡 + .empty 已布局 + 骨架屏清除。
    // 不使用固定 sleep 制造任何结论。
    await page.waitForFunction(() => {
      const v = document.querySelector('#view');
      if (!v) return false;
      if (!(v.innerText || '').includes('交易加载失败')) return false;
      if (document.querySelectorAll('dialog[open]').length !== 0) return false;
      const empty = v.querySelector('.empty');
      if (!empty || empty.getBoundingClientRect().height === 0) return false;
      if (v.querySelectorAll('[aria-busy="true"]').length !== 0) return false;
      return true;
    }, null, { timeout: 15000 });

    const obs = await page.evaluate((injectMsg) => {
      const v = document.querySelector('#view');
      const text = v ? (v.innerText || '') : '';
      const empty = v ? v.querySelector('.empty') : null;
      const p = empty ? empty.querySelector('p') : null;
      return {
        viewport: { width: window.innerWidth, height: window.innerHeight },
        errLine: text.split('\n').map((t) => t.trim()).find((t) => t.includes('交易加载失败')) || '',
        injectMsgVisible: text.includes(injectMsg),
        ariaBusyCount: v ? v.querySelectorAll('[aria-busy="true"]').length : -1,
        skeletonBars: v ? v.querySelectorAll('.skel').length : -1,
        dialogsOpen: document.querySelectorAll('dialog[open]').length,
        emptyBoxCount: v ? v.querySelectorAll('.empty').length : -1,
        emptyText: p ? p.textContent : '',
        emptyHeight: empty ? empty.getBoundingClientRect().height : -1,
        viewOpacity: v ? window.getComputedStyle(v).opacity : null,
        paragraphOpacity: p ? window.getComputedStyle(p).opacity : null,
        toast: (document.querySelector('#toast') || {}).innerText || '',
      };
    }, INJECT_MSG);
    log(`观测: ${JSON.stringify(obs, null, 2)}`);

    const qualified = obs.errLine.includes('交易加载失败')
      && obs.injectMsgVisible
      && obs.ariaBusyCount === 0
      && obs.skeletonBars === 0
      && obs.dialogsOpen === 0
      && obs.emptyBoxCount >= 1
      && obs.emptyHeight > 0;
    log(`截图前置自检: ${qualified ? 'PASS（允许产出截图）' : 'FAIL（不产出截图，避免不合格证据）'}`);

    if (!qualified) {
      rc = 3;
    } else {
      // animations:'disabled' 是 Playwright 官方选项：把 CSS 动画/过渡快进到终态，
      // 专门用于避开此前 I-TXNS-19 / I-ACCT-06 截图出现的「过渡态几乎全白」问题。
      await page.screenshot({ path: SHOT_PATH, fullPage: false, animations: 'disabled' });
      const st = fs.statSync(SHOT_PATH);
      log(`截图已保存: ${path.relative(REPO_ROOT, SHOT_PATH)}（${st.size} bytes）`);
      log("截图参数: page.screenshot({ fullPage:false, animations:'disabled' })；未覆盖任何既有截图");
    }
    await ctx.close().catch(() => {});
  } catch (e) {
    log(`执行异常: ${String(e && e.stack ? e.stack : e)}`);
    rc = 2;
  } finally {
    if (bh) await bh.browser.close().catch(() => {});
    if (server) await server.stop().catch(() => {});
    try { fs.rmSync(dataDir, { recursive: true, force: true }); } catch (_) { /* ignore */ }
  }
  log(`结束(UTC): ${new Date().toISOString()}`);
  log(`=== EXIT_CODE=${rc} ===`);
  fs.writeFileSync(LOG_PATH, `${lines.join('\n')}\n`);
  process.exitCode = rc;
}

main();
