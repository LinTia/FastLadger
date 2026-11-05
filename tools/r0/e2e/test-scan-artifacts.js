'use strict';

/**
 * FastLedger R0 E2E —— scan-artifacts.js 负向合成测试（P1 安全整改 / Round 3，覆盖 K1–K3）。
 *
 * 运行：node --test tools/r0/e2e/test-scan-artifacts.js
 *
 * 安全约定：所有「疑似凭据」样例均由运行时字符串拼接生成，并统一带
 * SYNTHETIC / FAKE 标记；仓库源码中不残留任何可被误判为真实凭据的字面量。
 * 样例文件写入系统临时目录（os.tmpdir），测试结束即删除，不落入仓库，
 * 因此 tools/r0/scan-credentials.js --strict 不会把它们计入真实凭证候选。
 *
 * 每组都会打印期望与实际退出码，并断言扫描输出不含匹配值本身。
 *
 * Round 2（case-01~17）：干净集 / JWT / 黑名单文件名 / Bearer / Cookie / server.log /
 *   密钥赋值 / 目录缺失 / 文本伪装 PNG / 二进制名敏感词 / NUL 前缀 / 合法业务字段假阳性 /
 *   真实 PNG 魔数 / Bearer+JWT / Cookie 长值 / 符号链接 / 真实 PNG+内嵌 JWT。
 *
 * Round 3（case-18~30，本次 K1–K3 修复专测）：
 *   K1 编码绕过：UTF-16LE（K1-01）/ UTF-16BE（K1-02）/ 零散 NUL 插入（K1-03）/
 *     无法可靠解码（K1-04）均须 BLOCK，绝不降级 CLEAN。
 *   K2 短值 / 低熵：短 Token（K2-01）/ 短 API Key（K2-02）/ 短密码（K2-03）均须 BLOCK；
 *     普通短业务字段（K2-04）仍须 CLEAN（无误报）。
 *   K3 字段名 / Base64：x-api-key（K3-01）/ apiKey 与 API_KEY 变体（K3-02）/
 *     Base64 编码凭据（K3-03）/ JSON 嵌套凭据（K3-04）/ 大小写与分隔符变体（K3-05）均须 BLOCK。
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

/** 真实 PNG 起始魔数（8 字节）。 */
const PNG_MAGIC = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

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
 * 手动大端（UTF-16BE）编码：Node Buffer 不支持 'utf16be'，测试侧自实现。
 * @param {string} s 输入字符串。
 * @return {Buffer} UTF-16BE 字节（无 BOM）。
 */
function encodeUtf16BE(s) {
  const out = Buffer.alloc(s.length * 2);
  for (let i = 0; i < s.length; i += 1) {
    const c = s.charCodeAt(i);
    out[i * 2] = (c >> 8) & 0xff;
    out[i * 2 + 1] = c & 0xff;
  }
  return out;
}

/**
 * 创建位于系统临时目录的用例目录，并注册自动清理。
 * @param {import('node:test').TestContext} t 测试上下文。
 * @param {string} label 用例短标签。
 * @return {string} 临时目录绝对路径。
 */
function makeTmp(t, label) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), `r0-p1-r2-${label}-`));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

/**
 * 写入一组不含任何敏感形态的干净证据文件（含一张真实魔数 PNG）。
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
    Buffer.concat([PNG_MAGIC, Buffer.from([0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52])]),
  );
}

/**
 * 以子进程运行扫描器并断言退出码与「输出不含匹配值」。
 * @param {string} caseName 用例名。
 * @param {string} dir 待扫描目录。
 * @param {number} expected 期望退出码。
 * @param {string[]} forbiddenValues 禁止出现在扫描输出中的合成值。
 * @return {{code: number|null, stdout: string, stderr: string}} 运行结果。
 */
function runScanner(caseName, dir, expected, forbiddenValues = []) {
  const result = spawnSync(process.execPath, [SCANNER, dir], { encoding: 'utf8' });
  const code = result.status;
  console.log(`[负向测试] ${caseName} | 期望退出码=${expected} | 实际退出码=${code}`);
  assert.equal(code, expected, `${caseName}：退出码不符（stderr=${result.stderr || ''}）`);
  const output = `${result.stdout || ''}\n${result.stderr || ''}`;
  for (const value of forbiddenValues) {
    assert.ok(!output.includes(value), `${caseName}：扫描输出疑似包含匹配值本身`);
  }
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
  const jwt = syntheticJwt();
  fs.writeFileSync(path.join(dir, 'run.log'), `[info] sample=${jwt}\n`, 'utf8');
  const result = runScanner('case-02 合成 JWT 三段式', dir, EXIT_BLOCKED, [jwt]);
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
  const bearerValue = syntheticValue('BEARER');
  const authValue = syntheticValue('AUTH');
  fs.writeFileSync(
    path.join(dir, 'headers.log'),
    `${['Bearer', ' ', bearerValue].join('')}\n`,
    'utf8',
  );
  fs.writeFileSync(
    path.join(dir, 'auth.json'),
    `${JSON.stringify({ note: ['Authorization', ': ', authValue].join('') })}\n`,
    'utf8',
  );
  const result = runScanner('case-04 Bearer / 鉴权头', dir, EXIT_BLOCKED, [bearerValue, authValue]);
  assert.match(result.stdout, /BEARER_TOKEN/);
  assert.match(result.stdout, /AUTH_HEADER/);
});

test('case-05 Cookie 与 Set-Cookie 赋值 → 退出码 2', (t) => {
  const dir = makeTmp(t, 'cookies');
  writeCleanSet(dir);
  const cookieOne = syntheticValue('COOKIE');
  const cookieTwo = syntheticValue('COOKIE2');
  fs.writeFileSync(
    path.join(dir, 'cookies.log'),
    `${['Set-Cookie', ': session=', cookieOne].join('')}\n`,
    'utf8',
  );
  fs.writeFileSync(
    path.join(dir, 'cookie2.log'),
    `${['Cookie', ': sid=', cookieTwo].join('')}\n`,
    'utf8',
  );
  const result = runScanner('case-05 Cookie / Set-Cookie', dir, EXIT_BLOCKED, [cookieOne, cookieTwo]);
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
  const secretValue = syntheticValue('SECRET');
  fs.writeFileSync(
    path.join(dir, 'env-dump.log'),
    `${['EBK_JWT_SECRET', '=', secretValue].join('')}\n`,
    'utf8',
  );
  const result = runScanner('case-07 密钥赋值', dir, EXIT_BLOCKED, [secretValue]);
  assert.match(result.stdout, /EBK_JWT_SECRET_ASSIGN/);
});

test('case-08 目录不存在（参数/环境错误）→ 退出码 1', (t) => {
  const parent = makeTmp(t, 'missing');
  const missing = path.join(parent, 'not-created');
  const result = runScanner('case-08 目录不存在', missing, EXIT_ERROR);
  assert.match(result.stderr, /\[scan\]\[ERROR\]/);
});

test('case-09 纯文本凭据伪装成 .png → 退出码 2（Round 2 修订）', (t) => {
  const dir = makeTmp(t, 'pngtext');
  writeCleanSet(dir);
  const jwt = syntheticJwt();
  fs.writeFileSync(
    path.join(dir, 'embedded.png'),
    Buffer.from(`[info] sample=${jwt}\n`, 'latin1'),
  );
  const result = runScanner('case-09 文本伪装 .png', dir, EXIT_BLOCKED, [jwt]);
  assert.match(result.stdout, /JWT_LITERAL|SUSPICIOUS_BINARY/);
});

test('case-10 二进制文件名含敏感词仍被阻断 → 退出码 2', (t) => {
  const dir = makeTmp(t, 'binaryname');
  writeCleanSet(dir);
  fs.writeFileSync(
    path.join(dir, ['token', '-shot.png'].join('')),
    Buffer.concat([PNG_MAGIC, Buffer.from([0x00, 0x00, 0x00, 0x0d])]),
  );
  const result = runScanner('case-10 二进制文件名黑名单', dir, EXIT_BLOCKED);
  assert.match(result.stdout, /SENSITIVE_FILENAME/);
});

test('case-11 NUL 前缀白名单 .log 内藏合成 JWT → 退出码 2（N1 回归）', (t) => {
  const dir = makeTmp(t, 'nulllog');
  writeCleanSet(dir);
  const jwt = syntheticJwt();
  const payload = Buffer.from(`[info] sample=${jwt}\n`, 'latin1');
  fs.writeFileSync(path.join(dir, 'run.log'), Buffer.concat([Buffer.from([0x00]), payload]));
  const result = runScanner('case-11 NUL 前缀白名单文本', dir, EXIT_BLOCKED, [jwt]);
  assert.match(result.stdout, /JWT_LITERAL/);
  assert.match(result.stdout, /内容已全量扫描=/);
});

test('case-12 合法业务字段（鉴权与 Cookie 普通值）→ 退出码 0（N5 假阳性回归）', (t) => {
  const dir = makeTmp(t, 'falsepositive');
  writeCleanSet(dir);
  // 键名用运行时拼接，避免仓库源码里出现可被误判为真实凭据的字面量。
  const legalReport = Object.fromEntries([
    ['suite', 'a'],
    ['passed', 11],
    [['author', 'ization'].join(''), 'role-based-access'],
    [['co', 'okie'].join(''), 'same-site-lax'],
    ['note', 'plain report body'],
  ]);
  fs.writeFileSync(path.join(dir, 'report.json'), `${JSON.stringify(legalReport, null, 2)}\n`, 'utf8');
  const result = runScanner('case-12 合法业务字段', dir, EXIT_CLEAN);
  assert.match(result.stdout, /\[scan\] 结论：CLEAN/);
  assert.doesNotMatch(result.stdout, /AUTH_HEADER|COOKIE_HEADER/);
});

test('case-13 真实 PNG 魔数 + 干净内容 → 退出码 0', (t) => {
  const dir = makeTmp(t, 'realpng');
  writeCleanSet(dir);
  fs.writeFileSync(path.join(dir, 'report.md'), '# R0 report\n', 'utf8');
  fs.writeFileSync(
    path.join(dir, 'real.png'),
    Buffer.concat([PNG_MAGIC, Buffer.from([0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52])]),
  );
  const result = runScanner('case-13 真实 PNG 魔数', dir, EXIT_CLEAN);
  assert.match(result.stdout, /\[scan\] 结论：CLEAN/);
  assert.doesNotMatch(result.stdout, /SUSPICIOUS_BINARY/);
});

test('case-14 鉴权头携带 Bearer + 合成 JWT → 退出码 2（硬约束）', (t) => {
  const dir = makeTmp(t, 'bearerjwt');
  writeCleanSet(dir);
  const jwt = syntheticJwt();
  fs.writeFileSync(
    path.join(dir, 'headers.log'),
    `${['Authorization', ': ', 'Bearer', ' ', jwt].join('')}\n`,
    'utf8',
  );
  const result = runScanner('case-14 鉴权头 Bearer + JWT', dir, EXIT_BLOCKED, [jwt]);
  assert.match(result.stdout, /JWT_LITERAL/);
  assert.match(result.stdout, /BEARER_TOKEN/);
});

test('case-15 Cookie 携带长随机会话值 → 退出码 2', (t) => {
  const dir = makeTmp(t, 'longcookie');
  writeCleanSet(dir);
  const sessionValue = ['SYNTHETIC', 'FAKE', 'SESSION', 'A1b2C3d4E5f6', '0000'].join('');
  fs.writeFileSync(
    path.join(dir, 'cookies.log'),
    `${['Cookie', ': session=', sessionValue].join('')}\n`,
    'utf8',
  );
  const result = runScanner('case-15 Cookie 长值', dir, EXIT_BLOCKED, [sessionValue]);
  assert.match(result.stdout, /COOKIE_HEADER/);
});

test('case-16 符号链接 → 退出码 2', (t) => {
  const dir = makeTmp(t, 'symlink');
  writeCleanSet(dir);
  const target = path.join(dir, 'outside.md');
  fs.writeFileSync(target, '[info] outside\n', 'utf8');
  fs.symlinkSync(target, path.join(dir, 'linked.md'), 'file');
  const result = runScanner('case-16 符号链接', dir, EXIT_BLOCKED);
  assert.match(result.stdout, /SYMLINK_ENTRY/);
});

test('case-17 真实 PNG 魔数 + 内容内嵌合成 JWT → 退出码 2（证明零跳过分支）', (t) => {
  const dir = makeTmp(t, 'realpngjwt');
  writeCleanSet(dir);
  const jwt = syntheticJwt();
  // 魔数合法且含 NUL 字节，但内容里带凭据形态：魔数符合也照样必须过文本规则。
  fs.writeFileSync(
    path.join(dir, 'real-with-text.png'),
    Buffer.concat([
      PNG_MAGIC,
      Buffer.from([0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52]),
      Buffer.from(`[info] sample=${jwt}\n`, 'latin1'),
    ]),
  );
  const result = runScanner('case-17 真实 PNG 魔数 + 内嵌 JWT', dir, EXIT_BLOCKED, [jwt]);
  assert.match(result.stdout, /JWT_LITERAL/);
  // 魔数本身是通过的，因此阻断完全来自文本规则，而非魔数不符。
  assert.doesNotMatch(result.stdout, /SUSPICIOUS_BINARY/);
});

// ===== Round 3: K1-K3 security bypass tests =====

test('case-18 [K1-01] UTF-16LE encoded Bearer credential -> exit 2', (t) => {
  const dir = makeTmp(t, 'k1-01');
  writeCleanSet(dir);
  const bearerValue = syntheticValue('UTF16LE');
  // Written with null bytes between characters (UTF-16LE, no BOM) to bypass
  // scanners that only inspect latin1/UTF-8 text.
  fs.writeFileSync(
    path.join(dir, 'leak.txt'),
    Buffer.from(['Bearer', ' ', bearerValue].join(''), 'utf16le'),
  );
  const result = runScanner('case-18 [K1-01] UTF-16LE Bearer', dir, EXIT_BLOCKED, [bearerValue]);
  assert.match(result.stdout, /BEARER_TOKEN/);
});

test('case-19 [K1-02] UTF-16BE encoded auth header credential -> exit 2', (t) => {
  const dir = makeTmp(t, 'k1-02');
  writeCleanSet(dir);
  const bearerValue = syntheticValue('UTF16BE');
  // UTF-16BE big-endian encoding (no BOM) must also be reliably decoded and blocked.
  fs.writeFileSync(
    path.join(dir, 'leak.txt'),
    encodeUtf16BE(['Authorization', ': Bearer ', bearerValue].join('')),
  );
  const result = runScanner('case-19 [K1-02] UTF-16BE auth header', dir, EXIT_BLOCKED, [bearerValue]);
  assert.match(result.stdout, /BEARER_TOKEN/);
});

test('case-20 [K1-03] scattered NUL insertion into token value -> exit 2', (t) => {
  const dir = makeTmp(t, 'k1-03');
  writeCleanSet(dir);
  const tokenValue = syntheticValue('NULINJ');
  // A stray NUL injected between "Bearer " and the token keeps the file
  // "reliably UTF-8" yet breaks the plain Bearer regex; the NUL-squeeze rescan
  // must recover the continuous run and catch it via BEARER_TOKEN.
  fs.writeFileSync(
    path.join(dir, 'leak.txt'),
    ['Bearer ', String.fromCharCode(0), tokenValue].join(''),
    'latin1',
  );
  const result = runScanner('case-20 [K1-03] NUL insertion', dir, EXIT_BLOCKED, [tokenValue]);
  assert.match(result.stdout, /BEARER_TOKEN/);
});

test('case-21 [K1-04] undecodable text-like file -> exit 2 (never CLEAN)', (t) => {
  const dir = makeTmp(t, 'k1-04');
  writeCleanSet(dir);
  // Deterministic invalid-UTF8 / non-binary buffer >= 16 bytes that cannot be
  // reliably decoded by latin1/UTF-8/UTF-16LE/UTF-16BE. Must be BLOCKED, not CLEAN.
  const buf = Buffer.alloc(64);
  for (let i = 0; i < buf.length; i += 1) buf[i] = (i * 37 + 11) % 256;
  fs.writeFileSync(path.join(dir, 'random.bin'), buf);
  const result = runScanner('case-21 [K1-04] undecodable', dir, EXIT_BLOCKED);
  assert.match(result.stdout, /UNDECODABLE/);
});

test('case-22 [K2-01] short token -> exit 2', (t) => {
  const dir = makeTmp(t, 'k2-01');
  writeCleanSet(dir);
  const value = 'token: ab12';
  fs.writeFileSync(path.join(dir, 'env.txt'), value + '\n', 'utf8');
  const result = runScanner('case-22 [K2-01] short token', dir, EXIT_BLOCKED, ['ab12']);
  assert.match(result.stdout, /SENSITIVE_ASSIGN/);
});

test('case-23 [K2-02] short api key -> exit 2', (t) => {
  const dir = makeTmp(t, 'k2-02');
  writeCleanSet(dir);
  const value = 'api_key: xy78';
  fs.writeFileSync(path.join(dir, 'env.txt'), value + '\n', 'utf8');
  const result = runScanner('case-23 [K2-02] short api key', dir, EXIT_BLOCKED, ['xy78']);
  assert.match(result.stdout, /SENSITIVE_ASSIGN/);
});

test('case-24 [K2-03] short password / low entropy -> exit 2', (t) => {
  const dir = makeTmp(t, 'k2-03');
  writeCleanSet(dir);
  const value = 'password: pw9';
  fs.writeFileSync(path.join(dir, 'env.txt'), value + '\n', 'utf8');
  const result = runScanner('case-24 [K2-03] short password', dir, EXIT_BLOCKED, ['pw9']);
  assert.match(result.stdout, /SENSITIVE_ASSIGN/);
});

test('case-25 [K2-04] normal short business fields -> exit 0 (no false positive)', (t) => {
  const dir = makeTmp(t, 'k2-04');
  writeCleanSet(dir);
  const business = JSON.stringify({ unit: 'kg', qty: 2, note: 'milk', code: 'A1', status: 'ok' });
  fs.writeFileSync(path.join(dir, 'biz.json'), business + '\n', 'utf8');
  const result = runScanner('case-25 [K2-04] business fields', dir, EXIT_CLEAN);
  assert.match(result.stdout, /\[scan\] 结论：CLEAN/);
  assert.doesNotMatch(result.stdout, /SENSITIVE_ASSIGN/);
});

test('case-26 [K3-01] x-api-key header -> exit 2', (t) => {
  const dir = makeTmp(t, 'k3-01');
  writeCleanSet(dir);
  const value = syntheticValue('XAPIKEY');
  fs.writeFileSync(
    path.join(dir, 'headers.txt'),
    ['x-api-key: ', value].join('') + '\n',
    'utf8',
  );
  const result = runScanner('case-26 [K3-01] x-api-key', dir, EXIT_BLOCKED, [value]);
  assert.match(result.stdout, /SENSITIVE_ASSIGN/);
});

test('case-27 [K3-02] apiKey and API_KEY variants -> exit 2', (t) => {
  const dir = makeTmp(t, 'k3-02');
  writeCleanSet(dir);
  const v1 = syntheticValue('APIKEY');
  const v2 = syntheticValue('APIKEY2');
  fs.writeFileSync(
    path.join(dir, 'keys.txt'),
    ['apiKey: ', v1, '\nAPI_KEY=', v2].join('') + '\n',
    'utf8',
  );
  const result = runScanner('case-27 [K3-02] apiKey/API_KEY', dir, EXIT_BLOCKED, [v1, v2]);
  assert.match(result.stdout, /SENSITIVE_ASSIGN/);
});

test('case-28 [K3-03] Base64 encoded credential -> exit 2', (t) => {
  const dir = makeTmp(t, 'k3-03');
  writeCleanSet(dir);
  const jwt = syntheticJwt();
  const embedded = ['Authorization: Bearer ', jwt].join('');
  const b64 = Buffer.from(embedded, 'utf8').toString('base64');
  fs.writeFileSync(path.join(dir, 'data.txt'), ['data=', b64].join('') + '\n', 'utf8');
  const result = runScanner('case-28 [K3-03] Base64 credential', dir, EXIT_BLOCKED, [jwt, b64]);
  assert.match(result.stdout, /BASE64_CREDENTIAL/);
});

test('case-29 [K3-04] JSON nested credential (quoted key) -> exit 2', (t) => {
  const dir = makeTmp(t, 'k3-04');
  writeCleanSet(dir);
  const value = 'SYNTHETIC-NESTED-TOKEN-99887766';
  const nested = JSON.stringify({ config: { auth: { token: value } } });
  fs.writeFileSync(path.join(dir, 'nested.json'), nested + '\n', 'utf8');
  const result = runScanner('case-29 [K3-04] JSON nested', dir, EXIT_BLOCKED, [value]);
  assert.match(result.stdout, /SENSITIVE_ASSIGN/);
});

test('case-30 [K3-05] case and separator variants -> exit 2', (t) => {
  const dir = makeTmp(t, 'k3-05');
  writeCleanSet(dir);
  const v1 = syntheticValue('VARKEY');
  const v2 = syntheticValue('VARSECRET');
  fs.writeFileSync(
    path.join(dir, 'variants.txt'),
    ['X_Api_Key: ', v1, '\nclient-secret=', v2].join('') + '\n',
    'utf8',
  );
  const result = runScanner('case-30 [K3-05] variants', dir, EXIT_BLOCKED, [v1, v2]);
  assert.match(result.stdout, /SENSITIVE_ASSIGN/);
});
