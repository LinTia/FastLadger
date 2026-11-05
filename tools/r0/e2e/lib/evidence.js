'use strict';
/**
 * 证据落盘：目录、截图、console/network 摘要、SHA256 清单、环境信息。
 * 约定：所有证据写入 artifacts/r0-e2e/<时间戳>-<sha7>/，不写进 docs/，由 CI artifact 上传。
 */
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..', '..');

function gitSha() {
  try {
    return execFileSync('git', ['rev-parse', 'HEAD'], { cwd: REPO_ROOT, encoding: 'utf8' }).trim();
  } catch (_) {
    return process.env.GITHUB_SHA || 'unknown';
  }
}

function stamp() {
  return new Date().toISOString().replace(/[:.]/g, '-').replace('T', '_').slice(0, 19);
}

function sha256(buf) {
  return crypto.createHash('sha256').update(buf).digest('hex');
}

class RunDir {
  constructor(suite) {
    const root = process.env.R0_ARTIFACT_DIR || path.join(REPO_ROOT, 'artifacts', 'r0-e2e');
    this.dir = path.join(root, `${stamp()}-${gitSha().slice(0, 7)}-${suite}`);
    fs.mkdirSync(this.dir, { recursive: true });
    this.suite = suite;
    this.sub = (name) => {
      const d = path.join(this.dir, name);
      fs.mkdirSync(d, { recursive: true });
      return d;
    };
  }

  rel(p) {
    return path.relative(this.dir, p).split(path.sep).join('/');
  }

  /** 取一个（会自动建父目录的）绝对路径，用于 trace / 附件等 */
  path(name) {
    const p = path.join(this.dir, name);
    fs.mkdirSync(path.dirname(p), { recursive: true });
    return p;
  }

  writeJson(name, obj) {
    const p = path.join(this.dir, name);
    fs.mkdirSync(path.dirname(p), { recursive: true });
    fs.writeFileSync(p, JSON.stringify(obj, null, 2) + '\n');
    return p;
  }

  writeText(name, text) {
    const p = path.join(this.dir, name);
    fs.mkdirSync(path.dirname(p), { recursive: true });
    fs.writeFileSync(p, text);
    return p;
  }

  async screenshot(page, subdir, name) {
    const d = this.sub(subdir);
    const p = path.join(d, `${name}.png`);
    await page.screenshot({ path: p, fullPage: false });
    return p;
  }

  /** 递归计算目录内全部文件的 SHA256，写 manifest-sha256.txt */
  checksums() {
    const rows = [];
    const walk = (dir) => {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) walk(full);
        else {
          const buf = fs.readFileSync(full);
          rows.push(`${sha256(buf)}  ${this.rel(full)}`);
        }
      }
    };
    walk(this.dir);
    // 自身不参与（避免自引用），先写再排除
    const target = path.join(this.dir, 'SHA256SUMS.txt');
    const filtered = rows.filter((r) => !r.endsWith('  SHA256SUMS.txt'));
    fs.writeFileSync(target, filtered.join('\n') + '\n');
    return { file: target, count: filtered.length };
  }
}

/** 收集 console / pageerror / requestfailed，用于失败时取证 */
function attachDiagnostics(page, sink) {
  page.on('console', (msg) => {
    if (msg.type() === 'error' || msg.type() === 'warning') {
      sink.push({ kind: 'console', type: msg.type(), text: msg.text().slice(0, 500) });
    }
  });
  page.on('pageerror', (err) => sink.push({ kind: 'pageerror', text: String(err && err.message ? err.message : err).slice(0, 500) }));
  page.on('requestfailed', (req) => {
    sink.push({ kind: 'requestfailed', url: req.url().split('?')[0], reason: req.failure() ? req.failure().errorText : 'unknown' });
  });
  return sink;
}

function envInfo(extra = {}) {
  return {
    capturedAt: new Date().toISOString(),
    node: process.version,
    platform: `${process.platform} ${process.arch}`,
    os: `${os.type()} ${os.release()}`,
    cpus: os.cpus().length,
    memoryMB: Math.round(os.totalmem() / 1024 / 1024),
    cwd: process.cwd(),
    gitSha: gitSha(),
    ci: {
      githubActions: process.env.GITHUB_ACTIONS === 'true',
      runId: process.env.GITHUB_RUN_ID || null,
      runAttempt: process.env.GITHUB_RUN_ATTEMPT || null,
      workflow: process.env.GITHUB_WORKFLOW || null,
      job: process.env.GITHUB_JOB || null,
      serverUrl: process.env.GITHUB_SERVER_URL || null,
      repository: process.env.GITHUB_REPOSITORY || null,
      ref: process.env.GITHUB_REF || null,
    },
    ...extra,
  };
}

module.exports = { RunDir, sha256, envInfo, attachDiagnostics, gitSha };
