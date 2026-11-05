'use strict';
/**
 * 独立测试环境：临时 SQLite + 随机端口 + 独立 JWT 密钥。
 * 不碰生产数据库、不碰生产进程、不读本机任何既有状态。
 */
const { spawn, spawnSync } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const net = require('node:net');
const path = require('node:path');

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..', '..');

function serverBinary() {
  if (process.env.R0_SERVER_BIN) {
    if (!fs.existsSync(process.env.R0_SERVER_BIN)) throw new Error(`R0_SERVER_BIN 不存在：${process.env.R0_SERVER_BIN}`);
    return process.env.R0_SERVER_BIN;
  }
  const cands = [path.join(REPO_ROOT, 'server', process.platform === 'win32' ? 'fastledger.exe' : 'fastledger')];
  const hit = cands.find((p) => fs.existsSync(p));
  if (!hit) throw new Error('找不到服务二进制：请先 `go build -o server/fastledger ./cmd/server`，或设置 R0_SERVER_BIN');
  return hit;
}

function freePort() {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.unref();
    srv.on('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      const p = srv.address().port;
      srv.close(() => resolve(p));
    });
  });
}

async function waitHealth(base, timeoutMs = 90000) {
  const deadline = Date.now() + timeoutMs;
  let last = '';
  while (Date.now() < deadline) {
    try {
      const r = await fetch(`${base}/healthz`, { signal: AbortSignal.timeout(2000) });
      if (r.ok) {
        const j = await r.json();
        if (j && j.data && j.data.status === 'up') return j.data;
        last = JSON.stringify(j);
      } else {
        last = `HTTP ${r.status}`;
      }
    } catch (e) {
      last = String(e && e.message ? e.message : e);
    }
    await new Promise((r) => setTimeout(r, 300));
  }
  throw new Error(`服务在 ${timeoutMs}ms 内未就绪（最后状态：${last}）`);
}

function cli(bin, args, env) {
  const r = spawnSync(bin, args, { env: { ...process.env, ...env }, encoding: 'utf8' });
  return { status: r.status, stdout: r.stdout || '', stderr: r.stderr || '' };
}

/**
 * 启动一个隔离实例。
 * @param {{dataDir:string, port?:number, register?:boolean, reseed?:boolean}} opts
 */
async function startServer(opts = {}) {
  const bin = serverBinary();
  const dataDir = path.resolve(opts.dataDir);
  fs.mkdirSync(dataDir, { recursive: true });
  const port = opts.port || (await freePort());
  const env = {
    EBK_DATA_DIR: dataDir,
    EBK_DB_PATH: path.join(dataDir, 'fastledger.db'),
    EBK_HTTP_PORT: String(port),
    EBK_ENABLE_REGISTER: opts.register === false ? 'false' : 'true',
    // 每次运行都生成新密钥：不复用任何历史/生产密钥，也不落盘
    EBK_JWT_SECRET: crypto.randomBytes(48).toString('base64'),
  };

  const mig = cli(bin, ['db', 'migrate'], env);
  if (mig.status !== 0) throw new Error(`db migrate 失败：${mig.stderr || mig.stdout}`);

  const logPath = path.join(dataDir, 'server.log');
  const logFd = fs.openSync(logPath, 'a');
  const proc = spawn(bin, [], { env: { ...process.env, ...env }, stdio: ['ignore', logFd, logFd] });

  const base = `http://127.0.0.1:${port}`;
  let health;
  try {
    health = await waitHealth(base, Number(process.env.R0_HEALTH_TIMEOUT_MS || 90000));
  } catch (e) {
    try { proc.kill('SIGKILL'); } catch (_) { /* ignore */ }
    const tail = fs.existsSync(logPath) ? fs.readFileSync(logPath, 'utf8').split(/\r?\n/).slice(-15).join('\n') : '(no log)';
    throw new Error(`${e.message}\n--- server.log 尾部 ---\n${tail}`);
  }

  const startedAt = new Date().toISOString();
  return {
    base,
    port,
    dataDir,
    logPath,
    health,
    startedAt,
    bin,
    cliEnv: env,
    /** 用 CLI 建本地用户（不走 HTTP 注册，便于固定口令与角色） */
    createUser(username, password, { owner = false } = {}) {
      const args = ['user', 'create', '--username', username, '--password', password];
      if (owner) args.push('--owner');
      const r = cli(bin, args, env);
      if (r.status !== 0) throw new Error(`user create 失败（${username}）：${r.stderr || r.stdout}`);
      return r.stdout.trim();
    },
    async stop() {
      try { proc.kill('SIGTERM'); } catch (_) { /* ignore */ }
      const deadline = Date.now() + 8000;
      while (proc.exitCode === null && Date.now() < deadline) {
        await new Promise((r) => setTimeout(r, 100));
      }
      if (proc.exitCode === null) {
        try { proc.kill('SIGKILL'); } catch (_) { /* ignore */ }
      }
      try { fs.closeSync(logFd); } catch (_) { /* ignore */ }
    },
  };
}

module.exports = { startServer, waitHealth, freePort, serverBinary, REPO_ROOT };
