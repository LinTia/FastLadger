'use strict';

/**
 * FastLedger R0-RV-01 隔离运行时复验（Edge/Chromium headless + 原生 CDP）。
 *
 * 与泛化拦截的区别：Fetch 域只暂停 GET /api/v1/books/<id>/transactions，
 * 放行 /books、/accounts、/categories、/reports/monthly 等全部依赖请求。
 *
 * 可移植性（RV-06）：
 *   - 所有路径由脚本位置 / 仓库根 / 环境变量推导，不含个人目录、盘符或工作区绝对路径；
 *   - 凭证只从 R0_TOKEN 环境变量读取，缺失即安全失败，不打印、不落盘；
 *   - 默认输出到操作系统临时目录（仓库外），绝不覆盖 docs/r0-screenshots/runtime/；
 *   - 只有同时设置 R0_WRITE_EVIDENCE=1 与 R0_WRITE_EVIDENCE_CONFIRM=YES
 *     才允许写入正式证据目录（危险开关，默认关闭）；
 *   - 所有日志在写盘前经 ./redaction.js 统一脱敏，并做「原始令牌不得出现」自检。
 *
 * 依赖：Node >= 22（使用内置 WebSocket / fetch）、Edge 或 Chromium、已启动的 FastLedger 后端。
 * 详见 tools/r0/README.md。
 *
 * 退出码：0 成功 / 1 采集运行错误 / 2 配置缺失 / 3 危险开关未确认 / 4 脱敏自检失败。
 */

const { spawn, execFileSync } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { scrub } = require('./redaction.js');

const REPO_ROOT = path.resolve(__dirname, '..', '..');
const EVIDENCE_DIR = path.join(REPO_ROOT, 'docs', 'r0-screenshots', 'runtime');

function failConfig(missing) {
  process.stderr.write(`RV01_CONFIG_ERROR 缺少必需配置：${missing}\n`);
  process.stderr.write('详见 tools/r0/README.md；配置值不会被打印。\n');
  process.exit(2);
}

function resolveEdge() {
  if (process.env.R0_EDGE_PATH) {
    if (!fs.existsSync(process.env.R0_EDGE_PATH)) {
      failConfig('R0_EDGE_PATH 指向的浏览器可执行文件不存在');
    }
    return process.env.R0_EDGE_PATH;
  }
  // 仅内置无用户名、无盘符、无工作区路径的系统级候选。
  const candidates = [
    '/usr/bin/microsoft-edge',
    '/usr/bin/microsoft-edge-stable',
    '/usr/bin/chromium',
    '/usr/bin/chromium-browser',
    '/usr/bin/google-chrome',
    '/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge',
  ];
  const found = candidates.find((p) => fs.existsSync(p));
  if (!found) failConfig('R0_EDGE_PATH（未找到默认浏览器候选；Windows 请显式设置）');
  return found;
}

// 凭证检查优先于浏览器解析，确保「缺 R0_TOKEN」能被明确报出。
const TOKEN = (process.env.R0_TOKEN || '').trim();
if (!TOKEN) failConfig('R0_TOKEN（登录接口返回的 accessToken）');
const EDGE = resolveEdge();

const WHOAMI = process.env.R0_WHOAMI || 'test-user';
const BOOK_ID = process.env.R0_BOOK_ID || '1';
const BASE = (process.env.R0_BASE_URL || 'http://127.0.0.1:8080/').replace(/\/?$/, '/');
const PORT = Number(process.env.R0_CDP_PORT || 9231);
const PROFILE = process.env.R0_PROFILE_DIR || path.join(os.tmpdir(), 'fastledger-r0-rv01-profile');

const TARGET_PATH = `/api/v1/books/${BOOK_ID}/transactions`;
const URL_PATTERN = `*/api/v1/books/${BOOK_ID}/transactions*`;

const WRITE_EVIDENCE = process.env.R0_WRITE_EVIDENCE === '1';
let OUT;
if (WRITE_EVIDENCE) {
  if (process.env.R0_WRITE_EVIDENCE_CONFIRM !== 'YES') {
    process.stderr.write('RV01_GUARD 拒绝写入正式证据目录：R0_WRITE_EVIDENCE=1 时必须同时设置 R0_WRITE_EVIDENCE_CONFIRM=YES。\n');
    process.exit(3);
  }
  OUT = EVIDENCE_DIR;
  process.stdout.write(`RV01_GUARD 警告：本次将写入正式证据目录 ${OUT}\n`);
} else {
  OUT = process.env.R0_OUT_DIR || path.join(os.tmpdir(), 'fastledger-r0-rv01-output');
}

const FAIL_503_BODY = JSON.stringify({
  code: 503,
  msg: '模拟故障：账单列表服务暂不可用（R0-RV-01 隔离注入）',
});

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const iso = () => new Date().toISOString();
const sha256 = (buf) => crypto.createHash('sha256').update(buf).digest('hex');

/** 写盘前自检：序列化结果中不得出现原始令牌（不打印任何值）。 */
function assertNoOriginalToken(text) {
  if (TOKEN && text.includes(TOKEN)) {
    process.stderr.write('RV01_REDACTION_ERROR 日志中检测到原始令牌，已中止写入（值不打印）。\n');
    process.exit(4);
  }
}

/** 极简 CDP 客户端：自实现 send / on，基于 Node 内置 WebSocket。 */
class CDP {
  constructor(ws) {
    this.ws = ws;
    this.id = 0;
    this.pending = new Map();
    this.handlers = new Map();
    ws.addEventListener('message', (ev) => {
      const m = JSON.parse(typeof ev.data === 'string' ? ev.data : ev.data.toString());
      if (m.id && this.pending.has(m.id)) {
        const { resolve, reject } = this.pending.get(m.id);
        this.pending.delete(m.id);
        if (m.error) reject(new Error(JSON.stringify(m.error)));
        else resolve(m.result);
      } else if (m.method) {
        for (const h of this.handlers.get(m.method) || []) {
          try { h(m.params); } catch (e) { process.stderr.write('HANDLER_ERR ' + e.stack + '\n'); }
        }
      }
    });
  }

  send(method, params = {}) {
    return new Promise((resolve, reject) => {
      const id = ++this.id;
      this.pending.set(id, { resolve, reject });
      this.ws.send(JSON.stringify({ id, method, params }));
    });
  }

  on(method, h) {
    if (!this.handlers.has(method)) this.handlers.set(method, []);
    this.handlers.get(method).push(h);
  }
}

async function waitDevtools() {
  for (let i = 0; i < 80; i += 1) {
    try {
      const r = await fetch(`http://127.0.0.1:${PORT}/json/version`);
      if (r.ok) return await r.json();
    } catch (_) { /* devtools not up yet */ }
    await sleep(250);
  }
  throw new Error('浏览器 devtools 未就绪');
}

async function connectPageWs() {
  const r = await fetch(`http://127.0.0.1:${PORT}/json/list`);
  const list = await r.json();
  const page = list.find((t) => t.type === 'page');
  if (!page) throw new Error('未找到 page target');
  return page.webSocketDebuggerUrl;
}

async function servedSha(relPath) {
  const r = await fetch(BASE + relPath);
  return sha256(Buffer.from(await r.arrayBuffer()));
}

function repoSha(relPath) {
  const p = path.join(REPO_ROOT, relPath);
  return fs.existsSync(p) ? sha256(fs.readFileSync(p)) : 'unavailable: file missing';
}

function headSha() {
  try {
    return execFileSync('git', ['rev-parse', 'HEAD'], { cwd: REPO_ROOT, encoding: 'utf8' }).trim();
  } catch (_) {
    return 'unavailable: not a git checkout';
  }
}

const DOM_EXPR = [
  '(() => {',
  '  const q = (s) => document.querySelectorAll(s).length;',
  '  const t = document.querySelector("#toast");',
  '  const v = document.querySelector("#view");',
  '  const retry = Array.from(document.querySelectorAll("button, a"))',
  '    .filter((e) => /重试|重新加载|刷新/.test(e.textContent || ""))',
  '    .map((e) => (e.textContent || "").trim());',
  '  return {',
  '    href: location.href,',
  '    skelCount: q(".skel"),',
  '    ariaBusyCount: q("[aria-busy=true]"),',
  '    toast: t ? { text: t.textContent, className: t.className, hidden: t.classList.contains("hidden") } : null,',
  '    retryControls: retry,',
  '    viewText: (v ? v.innerText : "").slice(0, 400),',
  '    bodyText: (document.body.innerText || "").slice(0, 600)',
  '  };',
  '})()',
].join('\n');

const NETWORK_EVENTS = [
  'Network.requestWillBeSent',
  'Network.responseReceived',
  'Network.loadingFailed',
  'Network.loadingFinished',
  'Network.requestServedFromCache',
];
const CONSOLE_EVENTS = ['Runtime.consoleAPICalled', 'Runtime.exceptionThrown', 'Log.entryAdded'];

const SCENARIOS = [
  { name: 'pending-390x844', file: '01-pending-390x844.png', w: 390, h: 844, mobile: true, mode: 'pending', waitMs: 1800 },
  { name: 'rejected-network-390x844', file: '02-rejected-network-390x844.png', w: 390, h: 844, mobile: true, mode: 'fail', waitMs: 900 },
  { name: 'rejected-503-390x844', file: '03-rejected-503-390x844.png', w: 390, h: 844, mobile: true, mode: '503', waitMs: 900 },
  { name: 'pending-1280x800', file: '04-pending-1280x800.png', w: 1280, h: 800, mobile: false, mode: 'pending', waitMs: 1800 },
  { name: 'recovered-390x844', file: '05-recovered-390x844.png', w: 390, h: 844, mobile: true, mode: 'recovered', waitMs: 1800 },
];

function writeLogs(networkLog, consoleLog, domSnapshots) {
  const files = {
    'network-log.json': networkLog,
    'console-log.json': consoleLog,
    'dom-snapshots.json': domSnapshots,
  };
  // 统一脱敏 + 原始令牌自检，全部通过后才落盘。
  const serialized = {};
  for (const [name, value] of Object.entries(files)) {
    const text = JSON.stringify(scrub(value), null, 2) + '\n';
    assertNoOriginalToken(text);
    serialized[name] = text;
  }
  for (const [name, text] of Object.entries(serialized)) {
    fs.writeFileSync(path.join(OUT, name), text, 'utf8');
  }
}

(async () => {
  fs.rmSync(PROFILE, { recursive: true, force: true });
  fs.mkdirSync(OUT, { recursive: true });

  const edge = spawn(EDGE, [
    '--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check',
    '--hide-scrollbars', '--disable-extensions', '--no-sandbox',
    `--remote-debugging-port=${PORT}`, `--user-data-dir=${PROFILE}`,
    '--window-size=390,844', 'about:blank',
  ], { stdio: 'ignore' });

  let cdp = null;
  const networkLog = [];
  const consoleLog = [];
  const domSnapshots = [];
  const scriptActions = [];
  const meta = {
    purpose: 'R0-RV-01：隔离 GET /api/v1/books/<id>/transactions 后的 pending / rejected / 恢复态运行时证据',
    generatedAt: iso(),
    headSha: headSha(),
    repoRootRelative: '<repo-root> (由脚本位置推导，未记录绝对路径)',
    targetPath: TARGET_PATH,
    fetchUrlPattern: URL_PATTERN,
    exactMatchRule: 'Fetch.requestPaused 中再按 request.method === GET 且 URL.pathname 严格等于目标路径判定；其余一律 Fetch.continueRequest',
    tokenInjection: 'Page.addScriptToEvaluateOnNewDocument: localStorage.setItem("fl_token", <R0_TOKEN>); localStorage.setItem("fl_whoami", <R0_WHOAMI>)',
    tokenLength: TOKEN.length,
    redaction: 'tools/r0/redaction.js scrub()；覆盖请求头/响应头/headersText/Cookie/Authorization/Set-Cookie/URL 查询参数/JSON 请求体/JWT 字面量',
    writeEvidenceGuard: WRITE_EVIDENCE ? 'ENABLED (显式确认)' : 'DISABLED (默认，输出位于仓库外临时目录)',
    otherRequestsAllowed: [
      'GET /api/v1/books',
      `GET /api/v1/books/${BOOK_ID}/accounts`,
      `GET /api/v1/books/${BOOK_ID}/categories`,
      `GET /api/v1/books/${BOOK_ID}/reports/monthly?months=2`,
      'GET / (index.html) /app.js /style.css',
    ],
    scriptActions,
  };

  try {
    const versionInfo = await waitDevtools();
    meta.browserFromDevtools = versionInfo.Browser;
    meta.protocolVersion = versionInfo['Protocol-Version'];
    const wsUrl = await connectPageWs();
    const ws = new WebSocket(wsUrl);
    await new Promise((resolve, reject) => {
      ws.addEventListener('open', resolve);
      ws.addEventListener('error', reject);
    });
    cdp = new CDP(ws);

    const bv = await cdp.send('Browser.getVersion');
    meta.browserGetVersion = bv;

    meta.healthzAtCapture = await (await fetch(BASE + 'healthz')).text();
    meta.servedAssetSha256 = {
      'index.html': { served: await servedSha(''), repo: repoSha('server/web/dist/index.html') },
      'app.js': { served: await servedSha('app.js'), repo: repoSha('server/web/dist/app.js') },
      'style.css': { served: await servedSha('style.css'), repo: repoSha('server/web/dist/style.css') },
    };

    await cdp.send('Page.enable');
    await cdp.send('Runtime.enable');
    await cdp.send('Network.enable');
    await cdp.send('Log.enable');

    // 所有事件在入内存前即脱敏，避免任何中间态带凭证。
    for (const ev of NETWORK_EVENTS) {
      cdp.on(ev, (p) => networkLog.push({ event: ev, capturedAt: iso(), params: scrub(p) }));
    }
    for (const ev of CONSOLE_EVENTS) {
      cdp.on(ev, (p) => consoleLog.push({ event: ev, capturedAt: iso(), params: scrub(p) }));
    }

    let scenarioHandler = null;
    cdp.on('Fetch.requestPaused', (p) => {
      networkLog.push({ event: 'Fetch.requestPaused', capturedAt: iso(), params: scrub(p) });
      if (scenarioHandler) scenarioHandler(p);
    });

    const inject = await cdp.send('Page.addScriptToEvaluateOnNewDocument', {
      source: `localStorage.setItem("fl_token", ${JSON.stringify(TOKEN)}); localStorage.setItem("fl_whoami", ${JSON.stringify(WHOAMI)});`,
    });
    meta.tokenScriptIdentifier = inject.identifier;

    const waitFor = async (pred, timeoutMs, label) => {
      const t0 = Date.now();
      while (Date.now() - t0 < timeoutMs) {
        if (pred()) return true;
        await sleep(50);
      }
      throw new Error(`等待超时: ${label}`);
    };

    for (const sc of SCENARIOS) {
      await cdp.send('Emulation.setDeviceMetricsOverride', {
        width: sc.w, height: sc.h, deviceScaleFactor: 1, mobile: sc.mobile,
      });

      const held = [];
      const pauses = [];
      scenarioHandler = (p) => {
        let u = null;
        try { u = new URL(p.request.url); } catch (_) { /* 忽略非法 URL */ }
        const isTarget = p.request.method === 'GET' && u !== null && u.pathname === TARGET_PATH;
        if (!isTarget) {
          cdp.send('Fetch.continueRequest', { requestId: p.requestId }).catch(() => {});
          return;
        }
        const rec = { requestId: p.requestId, url: p.request.url, method: p.request.method, at: iso(), scenario: sc.name };
        pauses.push(rec);
        if (sc.mode === 'pending') {
          held.push(p.requestId);
          scriptActions.push({ at: iso(), action: 'HOLD (no continue/fail)', scenario: sc.name, requestId: p.requestId, url: p.request.url });
          return;
        }
        if (sc.mode === 'fail') {
          scriptActions.push({ at: iso(), action: 'Fetch.failRequest(Failed)', scenario: sc.name, requestId: p.requestId, url: p.request.url });
          cdp.send('Fetch.failRequest', { requestId: p.requestId, errorReason: 'Failed' }).catch(() => {});
          return;
        }
        if (sc.mode === '503') {
          scriptActions.push({ at: iso(), action: 'Fetch.fulfillRequest(503, JSON)', scenario: sc.name, requestId: p.requestId, url: p.request.url });
          cdp.send('Fetch.fulfillRequest', {
            requestId: p.requestId,
            responseCode: 503,
            responseHeaders: [
              { name: 'Content-Type', value: 'application/json; charset=utf-8' },
              { name: 'Cache-Control', value: 'no-store' },
            ],
            body: Buffer.from(FAIL_503_BODY, 'utf8').toString('base64'),
          }).catch(() => {});
        }
      };

      if (sc.mode !== 'recovered') {
        await cdp.send('Fetch.enable', { patterns: [{ urlPattern: URL_PATTERN, requestStage: 'Request' }] });
        scriptActions.push({ at: iso(), action: 'Fetch.enable', scenario: sc.name, urlPattern: URL_PATTERN, requestStage: 'Request' });
      } else {
        scriptActions.push({ at: iso(), action: '拦截已解除（Fetch.disable，未重新 enable）后重新导航', scenario: sc.name });
      }

      const url = `${BASE}?rv01=${encodeURIComponent(sc.name)}#b/${BOOK_ID}`;
      scriptActions.push({ at: iso(), action: 'Page.navigate', scenario: sc.name, url });
      await cdp.send('Page.navigate', { url });

      if (sc.mode !== 'recovered') {
        await waitFor(() => pauses.length > 0, 8000, `${sc.name} 目标请求被 Fetch 暂停`);
      }
      await sleep(sc.waitMs);

      const evalRes = await cdp.send('Runtime.evaluate', {
        expression: DOM_EXPR, returnByValue: true, awaitPromise: false,
      });
      const dom = evalRes.exceptionDetails
        ? { error: String(evalRes.exceptionDetails.text), detail: evalRes.exceptionDetails }
        : evalRes.result.value;
      domSnapshots.push({
        scenario: sc.name, file: sc.file, viewport: `${sc.w}x${sc.h}`, mobile: sc.mobile,
        mode: sc.mode, capturedAt: iso(), targetPauses: pauses, dom,
      });

      const shot = await cdp.send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false });
      const buf = Buffer.from(shot.data, 'base64');
      fs.writeFileSync(path.join(OUT, sc.file), buf);
      scriptActions.push({ at: iso(), action: 'Page.captureScreenshot', scenario: sc.name, file: sc.file, bytes: buf.length });

      for (const rid of held) {
        scriptActions.push({ at: iso(), action: 'RELEASE held -> Fetch.failRequest(Failed)', scenario: sc.name, requestId: rid });
        await cdp.send('Fetch.failRequest', { requestId: rid, errorReason: 'Failed' }).catch(() => {});
      }
      if (sc.mode !== 'recovered') {
        await cdp.send('Fetch.disable');
        scriptActions.push({ at: iso(), action: 'Fetch.disable', scenario: sc.name });
      }
      scenarioHandler = null;
      process.stdout.write(`OK ${sc.file} (${buf.length}B) pauses=${pauses.length} skel=${dom && dom.skelCount}\n`);
    }

    meta.scenarios = SCENARIOS.map((s) => ({ name: s.name, file: s.file, viewport: `${s.w}x${s.h}`, mode: s.mode }));
    networkLog.push({ event: '__META__', capturedAt: iso(), params: meta });
    consoleLog.push({ event: '__META__', capturedAt: iso(), params: { generatedAt: meta.generatedAt, browser: meta.browserFromDevtools, headSha: meta.headSha, note: 'console/Log/Runtime 事件均为真实 CDP 事件；脚本自身未向页面注入任何 console 调用' } });

    writeLogs(networkLog, consoleLog, domSnapshots);
    process.stdout.write(`TOTAL ${SCENARIOS.length} screenshots, ${networkLog.length} network events, ${consoleLog.length} console events\n`);
    process.stdout.write(`OUTPUT_DIR ${OUT}\n`);
    for (const name of SCENARIOS.map((s) => s.file).concat(['network-log.json', 'console-log.json', 'dom-snapshots.json'])) {
      const p = path.join(OUT, name);
      if (fs.existsSync(p)) {
        const b = fs.readFileSync(p);
        process.stdout.write(`SHA256 ${name} ${b.length} ${sha256(b)}\n`);
      }
    }
  } catch (e) {
    process.stderr.write('RV01_ERROR ' + (e && e.stack ? e.stack : String(e)) + '\n');
    // 出错也落盘已采集到的真实事件（仍经脱敏），便于排查，绝不伪造。
    try { writeLogs(networkLog, consoleLog, domSnapshots); } catch (e2) {
      process.stderr.write('RV01_LOG_WRITE_ERROR ' + String(e2) + '\n');
    }
    process.exitCode = 1;
  } finally {
    try { if (cdp) cdp.ws.close(); } catch (_) { /* ignore */ }
    edge.kill();
    // 只清理脚本自己创建的临时 profile；用户显式指定的目录绝不删除。
    const tmpRoot = path.resolve(os.tmpdir());
    const profileRoot = path.resolve(PROFILE);
    if (!process.env.R0_PROFILE_DIR && profileRoot.startsWith(tmpRoot + path.sep)) {
      try { fs.rmSync(PROFILE, { recursive: true, force: true }); } catch (_) { /* ignore */ }
    }
  }
})();
