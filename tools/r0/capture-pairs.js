'use strict';

/**
 * FastLedger R0 第四次证据补齐 · 包 B：原型 × 实现同视口成对截图采集骨架。
 *
 * 设计目标（可移植、默认安全、可追溯）：
 *   - 静态服务 prototype/（SPA fallback），并采集实现侧（已启动的 FastLedger 后端）。
 *   - 13 页 × 4 视口（360x800 / 390x844 / 430x932 / 1280x800）成对截图。
 *   - 输出 manifest.json：page/viewport/route/role/state/actualPath/bytes/sha256/timestamp。
 *   - 默认输出到操作系统临时目录（仓库外）；写正式证据目录需 R0_WRITE_EVIDENCE=1 + R0_WRITE_EVIDENCE_CONFIRM=YES。
 *   - 凭证只经环境变量读取，不打印、不落盘；截图不含请求头。
 *
 * 依赖：Node >= 22（内置 WebSocket/fetch）、Edge/Chromium。
 * 退出码：0 成功 / 1 采集运行错误 / 2 配置缺失 / 3 危险开关未确认。
 *
 * 用法见 tools/r0/README.md「包 B 采集」。
 */

const { spawn } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');

const REPO_ROOT = path.resolve(__dirname, '..', '..');
const EVIDENCE_DIR = path.join(REPO_ROOT, 'docs', 'r0-screenshots', 'pairs');

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const iso = () => new Date().toISOString();
const sha256 = (buf) => crypto.createHash('sha256').update(buf).digest('hex');

function failConfig(msg) {
  process.stderr.write(`PAIRS_CONFIG_ERROR ${msg}\n`);
  process.exit(2);
}

function resolveEdge() {
  if (process.env.R0_EDGE_PATH) {
    if (!fs.existsSync(process.env.R0_EDGE_PATH)) failConfig('R0_EDGE_PATH 不存在');
    return process.env.R0_EDGE_PATH;
  }
  const candidates = [
    'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
    'C:/Program Files/Microsoft/Edge/Application/msedge.exe',
    '/usr/bin/microsoft-edge',
    '/usr/bin/chromium',
    '/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge',
  ];
  const found = candidates.find((p) => fs.existsSync(p));
  if (!found) failConfig('未找到浏览器，请设置 R0_EDGE_PATH');
  return found;
}

// ---------------- 配置 ----------------
const PROTO_DIR = process.env.R0_PROTO_DIR ? path.resolve(process.env.R0_PROTO_DIR) : '';
const APP_URL = (process.env.R0_APP_URL || 'http://127.0.0.1:8080/').replace(/\/?$/, '/');
const PROTO_PORT = Number(process.env.R0_PROTO_PORT || 8791);
const CDP_PORT = Number(process.env.R0_CDP_PORT || 9241);
const PROFILE = process.env.R0_PROFILE_DIR || path.join(os.tmpdir(), 'fastledger-r0-pairs-profile');
const TOKEN = (process.env.R0_TOKEN || '').trim();
const WHOAMI = process.env.R0_WHOAMI || 'r0-pair-user';
const BOOK_ID = process.env.R0_BOOK_ID || '';
const TX_ID = process.env.R0_TX_ID || '';
const SIDES = (process.env.R0_SIDES || (PROTO_DIR ? 'proto,impl' : 'impl')).split(',').map((s) => s.trim()).filter(Boolean);
const VIEWPORTS = (process.env.R0_VIEWPORTS || '360x800,390x844,430x932,1280x800')
  .split(',').map((v) => v.trim()).filter(Boolean)
  .map((v) => { const [w, h] = v.split('x').map(Number); return { w, h, mobile: w < 600 }; });
const WAIT_MS = Number(process.env.R0_WAIT_MS || 1200);
const ACTION_WAIT_MS = Number(process.env.R0_ACTION_WAIT_MS || 700);

// 同数据集注入：把 seed-from-dataset.js 产出的注入包写进原型 localStorage，
// 使原型不再使用内置演示数据，而是与实现侧完全同一份数据（含账户余额）。
const PROTO_INJECT_PATH = process.env.R0_PROTO_INJECT ? path.resolve(process.env.R0_PROTO_INJECT) : '';
let PROTO_INJECT = null;
if (PROTO_INJECT_PATH) {
  if (!fs.existsSync(PROTO_INJECT_PATH)) failConfig('R0_PROTO_INJECT 指向的文件不存在');
  PROTO_INJECT = JSON.parse(fs.readFileSync(PROTO_INJECT_PATH, 'utf8'));
}

const WRITE_EVIDENCE = process.env.R0_WRITE_EVIDENCE === '1';
let OUT;
if (WRITE_EVIDENCE) {
  if (process.env.R0_WRITE_EVIDENCE_CONFIRM !== 'YES') {
    process.stderr.write('PAIRS_GUARD 拒绝写正式证据目录：需 R0_WRITE_EVIDENCE_CONFIRM=YES。\n');
    process.exit(3);
  }
  OUT = process.env.R0_OUT_DIR || EVIDENCE_DIR;
} else {
  OUT = process.env.R0_OUT_DIR || path.join(os.tmpdir(), 'fastledger-r0-pairs-output');
}

// 13 页路由映射（可按需用 R0_PAGES_JSON 覆盖）。
// proto：路径路由；impl：hash 路由；null 表示该页是弹层/动作，需交互采集，本轮跳过并标 BLOCKED。
const DEFAULT_PAGES = [
  { id: 'login', proto: '/login', impl: '', action: 'login', note: '两侧均为未登录状态页：清会话后访问根路径' },
  { id: 'home', proto: '/home', impl: '#b/{book}' },
  { id: 'add-transaction', proto: '/add', impl: '#b/{book}', action: 'fab', note: '实现侧「记一笔」为弹层：在首页点 #fab 打开 #formDlg（原型 /add 为整页）' },
  { id: 'transaction-detail', proto: '/transaction', impl: '#b/{book}/tx/{tx}' },
  { id: 'accounts', proto: '/accounts', impl: '#b/{book}/accounts' },
  { id: 'categories', proto: '/categories', impl: '#b/{book}/categories' },
  { id: 'invest', proto: '/invest', impl: '#b/{book}/invest' },
  { id: 'budget', proto: '/budget', impl: '#b/{book}/budget' },
  { id: 'report', proto: '/report', impl: '#b/{book}/report' },
  { id: 'export', proto: '/export', impl: '#b/{book}/export' },
  { id: 'members', proto: '/family', impl: '#b/{book}/members' },
  { id: 'mine', proto: '/profile', impl: '#b/{book}/mine' },
  { id: 'settings', proto: '/profile', impl: '#settings', note: '原型设置入口在「我的」二级菜单' },
];

let PAGES = DEFAULT_PAGES;
if (process.env.R0_PAGES_JSON) {
  try { PAGES = JSON.parse(process.env.R0_PAGES_JSON); } catch (e) { failConfig('R0_PAGES_JSON 非法 JSON'); }
}
if (process.env.R0_ONLY) {
  const only = new Set(process.env.R0_ONLY.split(',').map((s) => s.trim()));
  PAGES = PAGES.filter((p) => only.has(p.id));
}

const EDGE = resolveEdge();
if (SIDES.includes('proto') && !PROTO_DIR) failConfig('R0_PROTO_DIR（prototype 目录）');
if (SIDES.includes('impl') && !BOOK_ID) failConfig('R0_BOOK_ID（实现侧账本 id）');
if (SIDES.includes('impl') && !TOKEN) failConfig('R0_TOKEN（实现侧登录 accessToken）');

// ---------------- 静态服务（SPA fallback） ----------------
const MIME = {
  '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8', '.png': 'image/png', '.svg': 'image/svg+xml',
  '.json': 'application/json; charset=utf-8', '.ico': 'image/x-icon', '.webp': 'image/webp',
};

function startStatic(root, port) {
  const server = http.createServer((req, res) => {
    try {
      const u = new URL(req.url, 'http://127.0.0.1');
      let rel = decodeURIComponent(u.pathname);
      let file = path.join(root, rel);
      // 防目录穿越
      if (!path.resolve(file).startsWith(path.resolve(root))) { res.writeHead(403); res.end('forbidden'); return; }
      if (fs.existsSync(file) && fs.statSync(file).isFile()) {
        res.writeHead(200, { 'Content-Type': MIME[path.extname(file).toLowerCase()] || 'application/octet-stream' });
        fs.createReadStream(file).pipe(res);
        return;
      }
      // SPA fallback
      const index = path.join(root, 'index.html');
      res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
      fs.createReadStream(index).pipe(res);
    } catch (e) {
      res.writeHead(500); res.end('server error');
    }
  });
  return new Promise((resolve) => server.listen(port, '127.0.0.1', () => resolve(server)));
}

// ---------------- CDP ----------------
class CDP {
  constructor(ws) {
    this.ws = ws; this.id = 0; this.pending = new Map(); this.handlers = new Map();
    ws.addEventListener('message', (ev) => {
      const m = JSON.parse(typeof ev.data === 'string' ? ev.data : ev.data.toString());
      if (m.id && this.pending.has(m.id)) {
        const { resolve, reject } = this.pending.get(m.id); this.pending.delete(m.id);
        if (m.error) reject(new Error(JSON.stringify(m.error))); else resolve(m.result);
      } else if (m.method) {
        for (const h of this.handlers.get(m.method) || []) { try { h(m.params); } catch (_) { /* ignore */ } }
      }
    });
  }
  send(method, params = {}) {
    return new Promise((resolve, reject) => {
      const id = ++this.id; this.pending.set(id, { resolve, reject });
      this.ws.send(JSON.stringify({ id, method, params }));
    });
  }
  on(method, h) { if (!this.handlers.has(method)) this.handlers.set(method, []); this.handlers.get(method).push(h); }
}

async function waitDevtools(port) {
  for (let i = 0; i < 80; i += 1) {
    try { const r = await fetch(`http://127.0.0.1:${port}/json/version`); if (r.ok) return r.json(); } catch (_) { /* not up */ }
    await sleep(250);
  }
  throw new Error('devtools 未就绪');
}

async function connectPageWs(port) {
  const r = await fetch(`http://127.0.0.1:${port}/json/list`);
  const list = await r.json();
  const page = list.find((t) => t.type === 'page');
  if (!page) throw new Error('未找到 page target');
  return page.webSocketDebuggerUrl;
}

function expand(tpl, book, tx) {
  return tpl.replace('{book}', String(book || '')).replace('{tx}', String(tx || ''));
}

(async () => {
  fs.rmSync(PROFILE, { recursive: true, force: true });
  fs.mkdirSync(OUT, { recursive: true });

  let staticServer = null;
  if (SIDES.includes('proto')) staticServer = await startStatic(PROTO_DIR, PROTO_PORT);

  const edge = spawn(EDGE, [
    '--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check',
    '--hide-scrollbars', '--disable-extensions', '--no-sandbox',
    `--remote-debugging-port=${CDP_PORT}`, `--user-data-dir=${PROFILE}`,
    '--window-size=1280,800', 'about:blank',
  ], { stdio: 'ignore' });

  let cdp = null;
  const manifest = {
    purpose: 'R0 第四次证据补齐 · 包 B：原型×实现同视口成对截图',
    generatedAt: iso(),
    sides: SIDES,
    viewports: VIEWPORTS.map((v) => `${v.w}x${v.h}`),
    protoDir: PROTO_DIR ? '<R0_PROTO_DIR> (未记录绝对路径)' : null,
    appUrl: APP_URL,
    bookId: BOOK_ID || null,
    txId: TX_ID || null,
    role: WHOAMI,
    standalone: '同一脚本、同一浏览器、同一会话；prototype 与实现分别采集，按 page+viewport 配对',
    items: [],
    skipped: [],
    errors: [],
  };

  try {
    const version = await waitDevtools(CDP_PORT);
    manifest.browser = version.Browser;
    const ws = new WebSocket(await connectPageWs(CDP_PORT));
    await new Promise((res, rej) => { ws.addEventListener('open', res); ws.addEventListener('error', rej); });
    cdp = new CDP(ws);
    await cdp.send('Page.enable');
    await cdp.send('Runtime.enable');

    // 注入脚本管理：登录页需要"未登录"态，其余页面需要登录态，因此必须可挂可摘
    const addScript = async (source) => {
      const r = await cdp.send('Page.addScriptToEvaluateOnNewDocument', { source });
      return r.identifier;
    };
    // 用 sessionStorage 标志位控制"本次导航是否为未登录态"，避免 add/remove 脚本与导航的竞态
    const tokenSource = `try{if(!sessionStorage.getItem("__r0_logged_out")){localStorage.setItem("fl_token",${JSON.stringify(TOKEN)});localStorage.setItem("fl_whoami",${JSON.stringify(WHOAMI)});}}catch(e){}`;
    const LOGOUT_FLAG = '__r0_logged_out';

    let implTokenId = null;
    if (SIDES.includes('impl') && TOKEN) implTokenId = await addScript(tokenSource);

    // 原型的数据层由 toolkit 做了命名空间前缀（`__miaoda_<appid>__:__fastledger_loggedIn`），
    // 前缀随构建变化且未登录时不会建键，因此不能硬编码：
    // 先打开 /login，必要时点一次演示登录让 app 建键，再从键名反推前缀，最后清空存储再正式采集。
    let protoPrefix = null;
    async function ensureProtoPrefix() {
      if (protoPrefix !== null) return protoPrefix;
      const readPrefix = async () => {
        const r = await cdp.send('Runtime.evaluate', {
          expression: `(()=>{const suf=':__fastledger_loggedIn';const k=Object.keys(localStorage).find(x=>x.endsWith(suf));return k?k.slice(0,k.length-suf.length):'';})()`,
          returnByValue: true,
        });
        return (r.result && r.result.value) || '';
      };
      await cdp.send('Page.navigate', { url: `http://127.0.0.1:${PROTO_PORT}/login` });
      await sleep(1200);
      protoPrefix = await readPrefix();
      if (!protoPrefix) {
        await cdp.send('Runtime.evaluate', {
          expression: `(()=>{const b=[...document.querySelectorAll('button')].find(x=>/登\\s*录/.test(x.textContent||''));if(b){b.click();return 'clicked';}return 'no-button';})()`,
          returnByValue: true,
        });
        await sleep(1500);
        protoPrefix = await readPrefix();
      }
      try { await cdp.send('Runtime.evaluate', { expression: 'try{localStorage.clear();sessionStorage.clear();}catch(e){}' }); } catch (_) { /* ignore */ }
      return protoPrefix;
    }
    async function applyProtoInjection() {
      const prefix = await ensureProtoPrefix();
      const flat = {};
      for (const [k, v] of Object.entries(PROTO_INJECT || {})) flat[k] = typeof v === 'string' ? v : JSON.stringify(v);
      const r = await cdp.send('Runtime.evaluate', {
        expression: `(()=>{const m=${JSON.stringify(flat)};const p=${JSON.stringify(prefix)};let n=0;for(const k in m){localStorage.setItem(p?(p+':'+k):k,m[k]);n++;}return n;})()`,
        returnByValue: true,
      });
      return r.result && r.result.value;
    }

    manifest.authStrategy = {
      proto: PROTO_INJECT ? 'localStorage 注入同数据集（运行时探测 toolkit 命名空间前缀）' : '内置演示数据 + 点击登录',
      impl: TOKEN ? 'localStorage 注入 fl_token' : '未提供 token',
    };
    if (PROTO_INJECT) {
      const prefix = await ensureProtoPrefix();
      manifest.datasetInjection = {
        path: '<R0_PROTO_INJECT> (未记录绝对路径)',
        storagePrefix: prefix ? `${prefix}:` : '(无前缀)',
        keys: Object.keys(PROTO_INJECT),
        counts: Object.fromEntries(Object.entries(PROTO_INJECT).map(([k, v]) => [k, Array.isArray(v) ? v.length : 1])),
        sha256: sha256(fs.readFileSync(PROTO_INJECT_PATH)),
      };
    }

    for (const vp of VIEWPORTS) {
      await cdp.send('Emulation.setDeviceMetricsOverride', { width: vp.w, height: vp.h, deviceScaleFactor: 1, mobile: vp.mobile });
      for (const page of PAGES) {
        for (const side of SIDES) {
          const isProto = side === 'proto';
          const action = page.action || '';
          const tpl = isProto ? page.proto : page.impl;
          if (!tpl && !action) {
            manifest.skipped.push({ page: page.id, side, reason: page.note || '无路由（动作/弹层）' });
            continue;
          }
          const file = `${page.id}__${side}__${vp.w}x${vp.h}.png`;

          // 登录页：先在**同源**页面里置未登录标志再清 localStorage（写在 about:blank 上无效）
          if (action === 'login') {
            const originUrl = isProto ? `http://127.0.0.1:${PROTO_PORT}/login` : APP_URL;
            try {
              await cdp.send('Page.navigate', { url: originUrl });
              await sleep(600);
              await cdp.send('Runtime.evaluate', {
                expression: `try{sessionStorage.setItem(${JSON.stringify(LOGOUT_FLAG)},'1');localStorage.clear();}catch(e){}`,
              });
            } catch (_) { /* ignore */ }
          } else {
            try {
              await cdp.send('Runtime.evaluate', {
                expression: `try{sessionStorage.removeItem(${JSON.stringify(LOGOUT_FLAG)});}catch(e){}`,
              });
            } catch (_) { /* ignore */ }
            if (isProto && PROTO_INJECT) await applyProtoInjection();
          }

          const url = action === 'login'
            ? (isProto ? `http://127.0.0.1:${PROTO_PORT}/login` : APP_URL)
            : (isProto
              ? `http://127.0.0.1:${PROTO_PORT}${tpl}`
              : `${APP_URL}${expand(tpl || '', BOOK_ID, TX_ID)}`);

          try {
            // 实现侧是 hash 路由：仅改 hash 属同文档跳转，new-document 注入脚本不会执行。
            // 加一个一次性 query 强制新建文档、同时保留 hash（服务端 SPA fallback 忽略 query）。
            const navUrl = isProto
              ? url
              : `${APP_URL}?r0=${encodeURIComponent(page.id)}${expand(tpl || '', BOOK_ID, TX_ID)}`;
            await cdp.send('Page.navigate', { url: navUrl });
            await sleep(WAIT_MS);
            let actionMeta = null;
            if (action === 'fab') {
              // 实现侧「记一笔」是 <dialog>：点 FAB 后等弹层真正打开再截
              const clicked = await cdp.send('Runtime.evaluate', {
                expression: `(()=>{const f=document.querySelector('#fab');if(!f)return 'no-fab';if(f.classList.contains('hidden'))return 'fab-hidden';f.click();return 'clicked';})()`,
                returnByValue: true,
              });
              await sleep(ACTION_WAIT_MS);
              const dlg = await cdp.send('Runtime.evaluate', {
                expression: `(()=>{const d=document.querySelector('#formDlg');if(!d)return {found:false};const r=d.getBoundingClientRect();return {found:true,open:!!d.open,hidden:d.classList.contains('hidden'),w:Math.round(r.width),h:Math.round(r.height)};})()`,
                returnByValue: true,
              });
              actionMeta = { click: clicked.result && clicked.result.value, dialog: dlg.result && dlg.result.value };
            }
            const evalRes = await cdp.send('Runtime.evaluate', {
              expression: '({href:location.href, pathname:location.pathname, hash:location.hash, title:document.title, hasToken:!!localStorage.getItem("fl_token"), text:(document.body.innerText||"").slice(0,200)})',
              returnByValue: true,
            });
            const meta = evalRes.result.value || {};
            const shot = await cdp.send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false });
            const buf = Buffer.from(shot.data, 'base64');
            fs.writeFileSync(path.join(OUT, file), buf);
            manifest.items.push({
              page: page.id, side, viewport: `${vp.w}x${vp.h}`, w: vp.w, h: vp.h,
              state: action === 'login' ? 'logged-out' : (action === 'fab' ? 'dialog-open' : 'default'),
              action: action || '',
              requested: url, navigated: navUrl, actualPath: meta.pathname, actualHash: meta.hash, hasToken: meta.hasToken,
              title: meta.title, bytes: buf.length, sha256: sha256(buf), capturedAt: iso(),
              dataset: PROTO_INJECT ? 'same-dataset' : 'native',
              actionMeta,
              note: page.note || '',
            });
            process.stdout.write(`OK ${file} ${buf.length}B path=${meta.pathname}${meta.hash || ''}${actionMeta ? ` action=${JSON.stringify(actionMeta)}` : ''}\n`);
          } catch (e) {
            manifest.errors.push({ page: page.id, side, viewport: `${vp.w}x${vp.h}`, error: String(e && e.message ? e.message : e) });
            process.stderr.write(`ERR ${page.id}/${side}/${vp.w}x${vp.h} ${e}\n`);
          }
        }
      }
    }

    fs.writeFileSync(path.join(OUT, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n', 'utf8');
    process.stdout.write(`TOTAL items=${manifest.items.length} skipped=${manifest.skipped.length} errors=${manifest.errors.length}\n`);
    process.stdout.write(`OUTPUT_DIR ${OUT}\n`);
    for (const f of fs.readdirSync(OUT)) {
      const b = fs.readFileSync(path.join(OUT, f));
      process.stdout.write(`SHA256 ${f} ${b.length} ${sha256(b)}\n`);
    }
  } catch (e) {
    process.stderr.write('PAIRS_ERROR ' + (e && e.stack ? e.stack : String(e)) + '\n');
    try { fs.writeFileSync(path.join(OUT, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n', 'utf8'); } catch (_) { /* ignore */ }
    process.exitCode = 1;
  } finally {
    try { if (cdp) cdp.ws.close(); } catch (_) { /* ignore */ }
    edge.kill();
    if (staticServer) staticServer.close();
    const tmpRoot = path.resolve(os.tmpdir());
    const profileRoot = path.resolve(PROFILE);
    if (!process.env.R0_PROFILE_DIR && profileRoot.startsWith(tmpRoot + path.sep)) {
      try { fs.rmSync(PROFILE, { recursive: true, force: true }); } catch (_) { /* ignore */ }
    }
  }
})();
