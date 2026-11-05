'use strict';

/**
 * FastLedger R0 E2E —— scan-artifacts.js 负向合成测试（P1 安全整改自证）。
 *
 * 运行：node --test tools/r0/e2e/test-scan-artifacts.js
 *
 * 安全约定：所有「疑似凭据」样例均由运行时字符串拼接生成，并统一带
 * SYNTHETIC / FAKE 标记；仓库源码中不残留任何可被误判为真实凭据的字面量。
 * 样例文件写入系统临时目录（os.tmpdir），测试结束即删除，不落入仓库，
 * 因此 tools/r0/scan-credentials.js --strict 不会把它们计入真实凭证候选。
 *
 * 每组都会打印期望与实际退出码。
 */

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const SCANNER = path.resolve(__dirname, 'scripts', 'scan-artifacts.js');
const EXIT_CLEAN = 0;
const EXIT_ERROR = 1;
const EXIT_BLOCKED = 2;

/**
 * 生成带 SYNTHETIC/FAKE 标记的运行期合成值。
 * @param {string} tag 用例标签。
 * @return {string} 合成值。
 */
function syntheticValue(tag) {
  return ['SYNTHETIC', 'FAKE', tag, '000000000000'].join('');
}

/**
 * 运行期拼接出 JWT 三段式形态，仅用于负向测试。
 * @return {string} 合成三段式样例。
 */
function syntheticJwt() {
  const head = ['ey', 'J', 'SYNTHETIC', 'FAKE', 'HEADER'].join('');
  const body = ['SYNTHETIC', 'FAKE', 'PAYLOAD', '0001'].join('');
  const sig = ['SYNTHETIC', 'FAKE', 'SIGNATURE', '0002'].join('');
  return [head, body, sig].join('.');
}

/**
 * 创建位于系统临时目录的用例目录，并注册自动清理。
 * @param {import('node:test').TestContext} t 测试上下文。
 * @param {string} label 用例短标签。
 * @return {string} 临时目录绝对路径。
 */
function makeTmp(t, label) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), `r0-p1-${label}-`));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

/**
 * 写入一组不含任何敏感形态的干净证据文件（含一张最小 PNG）。
 * @param {string} dir 用例目录。
 */
function writeCleanSet(dir) {
  fs.writeFileSync(path.join(dir, 'report.md'), '# R0 report\n\n- status: PASS\n', 'utf8');
  fs.writeFileSync(
    path.join(dir, 'report.json'),
    `${JSON.stringify({ suite: 'a', passed: 11, failed: 0 }, null, 2)}\n`,
    'utf8',
  );
  fs.writeFileSync(path.join(dir, 'SHA256SUMS.txt'), '0000  report.md\n', 'utf8');
  fs.writeFileSync(path.join(dir, 'run.log'), '[info] run finished\n', 'utf8');
  fs.writeFileSync(
    path.join(dir, 'shot.png'),
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0]),
  );
}

/**
 * 以子进程运行扫描器并断言退出码。
 * @param {string} caseName 用例名。
 * @param {string} dir 待扫描目录。
 * @param {number} expected 期望退出码。
 * @return {{code: number|null, stdout: string, stderr: string}} 运行结果。
 */
function runScanner(caseName, dir, expected) {
  const result = spawnSync(process.execPath, [SCANNER, dir], { encoding: 'utf8' });
  const code = result.status;
  console.log(`[负向测试] ${caseName} | 期望退出码=${expected} | 实际退出码=${code}`);
  assert.equal(code, expected, `${caseName}：退出码不符（stderr=${result.stderr || ''}）`);
  return { code, stdout: result.stdout || '', stderr: result.stderr || '' };
}

test('case-01 干净文件集 → 退出码 0', (t) => {
  const dir = makeTmp(t, 'clean');
  writeCleanSet(dir);
  const result = runScanner('case-01 干净文件集', dir, EXIT_CLEAN);
  assert.match(result.stdout, /\[scan\] 结论：CLEAN/);
});

test('case-02 合成 JWT 三段式 → 退出码 2', (t) => {
  const dir = makeTmp(t, 'jwt');
  writeCleanSet(dir);
  fs.writeFileSync(path.join(dir, 'run.log'), `[info] sample=${syntheticJwt()}\n`, 'utf8');
  const result = runScanner('case-02 合成 JWT 三段式', dir, EXIT_BLOCKED);
  assert.match(result.stdout, /JWT_LITERAL/);
});

test('case-03 名为 trace.zip 的归档 → 退出码 2', (t) => {
  const dir = makeTmp(t, 'trace');
  writeCleanSet(dir);
  fs.writeFileSync(path.join(dir, 'trace.zip'), Buffer.from([0x50, 0x4b, 0x03, 0x04]));
  const result = runScanner('case-03 trace.zip 黑名单文件名', dir, EXIT_BLOCKED);
  assert.match(result.stdout, /TRACE_ARCHIVE/);
});

test('case-04 Bearer 与鉴权头赋值 → 退出码 2', (t) => {
  const dir = makeTmp(t, 'headers');
  writeCleanSet(dir);
  fs.writeFileSync(
    path.join(dir, 'headers.log'),
    `${['Bearer', ' ', syntheticValue('BEARER')].join('')}\n`,
    'utf8',
  );
  fs.writeFileSync(
    path.join(dir, 'auth.json'),
    `${JSON.stringify({ note: ['Authorization', ': ', syntheticValue('AUTH')].join('') })}\n`,
    'utf8',
  );
  const result = runScanner('case-04 Bearer / 鉴权头', dir, EXIT_BLOCKED);
  assert.match(result.stdout, /BEARER_TOKEN/);
  assert.match(result.stdout, /AUTH_HEADER/);
});

test('case-05 Cookie 与 Set-Cookie 赋值 → 退出码 2', (t) => {
  const dir = makeTmp(t, 'cookies');
  writeCleanSet(dir);
  fs.writeFileSync(
    path.join(dir, 'cookies.log'),
    `${['Set-Cookie', ': session=', syntheticValue('COOKIE')].join('')}\n`,
    'utf8',
  );
  fs.writeFileSync(
    path.join(dir, 'cookie2.log'),
    `${['Cookie', ': sid=', syntheticValue('COOKIE2')].join('')}\n`,
    'utf8',
  );
  const result = runScanner('case-05 Cookie / Set-Cookie', dir, EXIT_BLOCKED);
  assert.match(result.stdout, /COOKIE_HEADER/);
});

test('case-06 server.log 服务原始日志 → 退出码 2', (t) => {
  const dir = makeTmp(t, 'serverlog');
  writeCleanSet(dir);
  fs.writeFileSync(path.join(dir, 'server.log'), '[info] server started\n', 'utf8');
  const result = runScanner('case-06 server.log', dir, EXIT_BLOCKED);
  assert.match(result.stdout, /SERVER_LOG/);
});

test('case-07 JWT 签名密钥赋值 → 退出码 2', (t) => {
  const dir = makeTmp(t, 'jwtenv');
  writeCleanSet(dir);
  fs.writeFileSync(
    path.join(dir, 'env-dump.log'),
    `${['EBK_JWT_SECRET', '=', syntheticValue('SECRET')].join('')}\n`,
    'utf8',
  );
  const result = runScanner('case-07 密钥赋值', dir, EXIT_BLOCKED);
  assert.match(result.stdout, /EBK_JWT_SECRET_ASSIGN/);
});

test('case-08 目录不存在（参数/环境错误）→ 退出码 1', (t) => {
  const parent = makeTmp(t, 'missing');
  const missing = path.join(parent, 'not-created');
  const result = runScanner('case-08 目录不存在', missing, EXIT_ERROR);
  assert.match(result.stderr, /\[scan\]\[ERROR\]/);
});

test('case-09 二进制内容按设计跳过扫描 → 退出码 0', (t) => {
  const dir = makeTmp(t, 'binaryskip');
  writeCleanSet(dir);
  // 二进制扩展名内的文本形态字节不做内容扫描；文件名黑名单仍照常生效。
  fs.writeFileSync(
    path.join(dir, 'embedded.png'),
    Buffer.from(`[info] sample=${syntheticJwt()}\n`, 'utf8'),
  );
  const result = runScanner('case-09 二进制跳过内容扫描', dir, EXIT_CLEAN);
  assert.doesNotMatch(result.stdout, /JWT_LITERAL/);
});

test('case-10 二进制文件名含敏感词仍被阻断 → 退出码 2', (t) => {
  const dir = makeTmp(t, 'binaryname');
  writeCleanSet(dir);
  fs.writeFileSync(
    path.join(dir, ['token', '-shot.png'].join('')),
    Buffer.from([0x89, 0x50, 0x4e, 0x47]),
  );
  const result = runScanner('case-10 二进制文件名黑名单', dir, EXIT_BLOCKED);
  assert.match(result.stdout, /SENSITIVE_FILENAME/);
});
