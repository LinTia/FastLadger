'use strict';

/**
 * P1 Round 2 端到端自证辅助脚本（可复现，零第三方依赖）。
 *
 * 运行：node docs/r0-evidence-7th-fix/p1/round2/round2-verify.js
 * 退出码：0 全部断言通过 / 1 任一项不符。
 *
 * 覆盖：
 *   A. QA 绕过场景：白名单 .log 头插 1 个 NUL + 合成 JWT → prepare 收集 → scan 退出 2；
 *   B. QA 绕过场景：纯文本凭据伪装 .png（无魔数）→ prepare 收集 → scan 退出 2；
 *   C. 合法业务字段假阳性样本 → scan 退出 0；
 *   D. 干净真实产物链路（artifacts/r0-qa-verify 与 workflow 默认源 artifacts/r0-e2e）
 *      → prepare 0 / scan 0，证明收紧未把正常截图与报告误判为红；
 *   E. 真实 PNG 魔数 + 内容内嵌合成 JWT → prepare 0 / scan 2，且**不记 SUSPICIOUS_BINARY**，
 *      专门证明「魔数符合也照样过文本规则」= 零跳过分支；
 *   F. 扫描器源码静态核验：无 looksBinary / binarySkipped、主扫描循环内无 continue、
 *      使用 latin1 全量映射、魔数仅为额外命中条件、统计口径如实、超限仍抛错；
 *   G. 对绕过场景明确打印「该文件会被阻断、不会上传」结论。
 *
 * 合成值全部在运行时字符串拼接生成并带 SYNTHETIC / FAKE 标记，样本写入系统临时目录、
 * 结束时删除，不落入仓库；日志只打印退出码、文件路径、命中类型与数量，
 * **绝不打印任何匹配值**。
 */

const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..', '..');
const PREPARE = path.join(REPO_ROOT, 'tools', 'r0', 'e2e', 'scripts', 'prepare-upload.js');
const SCANNER = path.join(REPO_ROOT, 'tools', 'r0', 'e2e', 'scripts', 'scan-artifacts.js');

/** 真实 PNG 起始魔数（8 字节）。 */
const PNG_MAGIC = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

const TMP_ROOT = fs.mkdtempSync(path.join(os.tmpdir(), 'fastledger-r0-p1-round2-'));
let failures = 0;

/** 输出一行日志。
 * @param {string} line 日志内容。
 */
function log(line) {
  process.stdout.write(`${line}\n`);
}

/**
 * 记录一条断言结果。
 * @param {string} label 检查项。
 * @param {boolean} cond 是否通过。
 * @param {string} detail 补充信息。
 */
function check(label, cond, detail) {
  if (!cond) failures += 1;
  log(`${cond ? '[PASS]' : '[FAIL]'} ${label}${detail ? ` :: ${detail}` : ''}`);
}

/**
 * 以子进程执行脚本并原样输出其 stdout / stderr（不含任何凭据值）。
 * @param {string} label 步骤名。
 * @param {string} script 脚本绝对路径。
 * @param {string[]} args 参数。
 * @return {{code: number|null, stdout: string, stderr: string}} 执行结果。
 */
function run(label, script, args) {
  const result = spawnSync(process.execPath, [script, ...args], {
    encoding: 'utf8',
    cwd: REPO_ROOT,
  });
  const code = result.status;
  log(`--- ${label} 实际退出码=${code} ---`);
  const stdout = result.stdout || '';
  const stderr = result.stderr || '';
  if (stdout.trim()) log(stdout.trimEnd());
  if (stderr.trim()) log(stderr.trimEnd());
  return { code, stdout, stderr };
}

/**
 * 运行期拼接出带 SYNTHETIC / FAKE 标记的 JWT 三段式形态。
 * @return {string} 合成值。
 */
function syntheticJwt() {
  const head = ['ey', 'J', 'SYNTHETIC', 'FAKE', 'HEADER'].join('');
  const body = ['SYNTHETIC', 'FAKE', 'PAYLOAD', '0001'].join('');
  const sig = ['SYNTHETIC', 'FAKE', 'SIGNATURE', '0002'].join('');
  return [head, body, sig].join('.');
}

/**
 * 在系统临时根目录下创建一个用例目录。
 * @param {string} name 目录名。
 * @return {string} 绝对路径。
 */
function tmpDir(name) {
  const dir = path.join(TMP_ROOT, name);
  fs.mkdirSync(dir, { recursive: true });
  return dir;
}

/** 复刻 QA e2e-06：NUL 前缀白名单文本。 */
function verifyNulPrefixedLog() {
  log('');
  log('=== 场景 A · NUL 前缀白名单 .log（QA e2e-06 复刻）===');
  const src = tmpDir('nul-src');
  const dest = tmpDir('nul-dest');
  const jwt = syntheticJwt();
  fs.writeFileSync(path.join(src, 'report.md'), '# R0 report\n', 'utf8');
  fs.writeFileSync(
    path.join(src, 'run.log'),
    Buffer.concat([Buffer.from([0x00]), Buffer.from(`[info] sample=${jwt}\n`, 'latin1')]),
  );

  const prepared = run('prepare（NUL .log）', PREPARE, ['--source', src, '--dest', dest]);
  check('prepare 退出 0 且确实收集了 run.log',
    prepared.code === 0 && prepared.stdout.includes('run.log'));
  const scanned = run('scan（NUL .log）', SCANNER, [dest]);
  check('scan 退出 2（失败关闭，不再静默 CLEAN）', scanned.code === 2,
    `实际退出码=${scanned.code}`);
  check('命中 JWT_LITERAL', scanned.stdout.includes('JWT_LITERAL'));
  check('输出不含合成值本身', !scanned.stdout.includes(jwt) && !scanned.stderr.includes(jwt));
  check('统计口径为全量扫描、跳过=0', /跳过内容扫描=0/.test(scanned.stdout));
  log('结论：该文件会被阻断、不会上传（scan=2 → artifact_scan.outcome=failure → 上传 step 被跳过）');
}

/** 复刻 QA e2e-05：纯文本凭据伪装 .png（无魔数）。 */
function verifyTextDisguisedPng() {
  log('');
  log('=== 场景 B · 纯文本凭据伪装 .png（QA e2e-05 复刻）===');
  const src = tmpDir('png-src');
  const dest = tmpDir('png-dest');
  const jwt = syntheticJwt();
  fs.writeFileSync(path.join(src, 'report.md'), '# R0 report\n', 'utf8');
  fs.writeFileSync(
    path.join(src, 'shot.png'),
    Buffer.from(`[info] sample=${jwt}\n`, 'latin1'),
  );

  const prepared = run('prepare（文本伪装 .png）', PREPARE, ['--source', src, '--dest', dest]);
  check('prepare 退出 0 且确实收集了 shot.png',
    prepared.code === 0 && prepared.stdout.includes('shot.png'));
  const scanned = run('scan（文本伪装 .png）', SCANNER, [dest]);
  check('scan 退出 2（失败关闭，不再静默 CLEAN）', scanned.code === 2,
    `实际退出码=${scanned.code}`);
  check('命中 JWT_LITERAL 或 SUSPICIOUS_BINARY',
    /JWT_LITERAL|SUSPICIOUS_BINARY/.test(scanned.stdout));
  check('输出不含合成值本身', !scanned.stdout.includes(jwt) && !scanned.stderr.includes(jwt));
  log('结论：该文件会被阻断、不会上传（scan=2 → artifact_scan.outcome=failure → 上传 step 被跳过）');
}

/** N5 假阳性回归：合法业务字段不应被误阻断。 */
function verifyNoFalsePositive() {
  log('');
  log('=== 场景 C · 合法业务字段（N5 假阳性回归）===');
  const dir = tmpDir('false-positive');
  const legalReport = Object.fromEntries([
    ['suite', 'a'],
    ['passed', 11],
    [['author', 'ization'].join(''), 'role-based-access'],
    [['co', 'okie'].join(''), 'same-site-lax'],
    ['note', 'plain report body'],
  ]);
  fs.writeFileSync(path.join(dir, 'report.json'), `${JSON.stringify(legalReport, null, 2)}\n`, 'utf8');
  const scanned = run('scan（合法业务字段）', SCANNER, [dir]);
  check('scan 退出 0（不再误阻断合法证据）', scanned.code === 0,
    `实际退出码=${scanned.code}`);
  check('未命中 AUTH_HEADER / COOKIE_HEADER',
    !/AUTH_HEADER|COOKIE_HEADER/.test(scanned.stdout));
}

/**
 * 干净真实产物链路：收紧后仍须退出 0。
 * @param {string} label 场景名。
 * @param {string} source 源目录绝对路径。
 */
function verifyCleanChain(label, source) {
  log('');
  log(`=== 场景 D · 干净真实产物链路（${label}）===`);
  const dest = tmpDir(`clean-${label}-dest`);
  const prepared = run(`prepare（${label}）`, PREPARE, ['--source', source, '--dest', dest]);
  const scanned = run(`scan（${label}）`, SCANNER, [dest]);
  check(`prepare 退出 0（${label}）`, prepared.code === 0, `实际退出码=${prepared.code}`);
  check(`scan 退出 0（${label}）`, scanned.code === 0, `实际退出码=${scanned.code}`);
  check('命中总数=0', /命中总数=0/.test(scanned.stdout));
  check('真实 PNG 未被判 SUSPICIOUS_BINARY', !scanned.stdout.includes('SUSPICIOUS_BINARY'));
  log(`结论：${label} 干净链路仍允许上传（prepare=0 / scan=0），收紧未误伤正常截图与报告`);
}

/** 场景 E：真实 PNG 魔数也照样过文本规则，证明零跳过分支。 */
function verifyRealPngWithEmbeddedJwt() {
  log('');
  log('=== 场景 E · 真实 PNG 魔数 + 内容内嵌合成 JWT（证明零跳过分支）===');
  const src = tmpDir('realpng-jwt-src');
  const dest = tmpDir('realpng-jwt-dest');
  const jwt = syntheticJwt();
  fs.writeFileSync(path.join(src, 'report.md'), '# R0 report\n', 'utf8');
  fs.writeFileSync(
    path.join(src, 'real-with-text.png'),
    Buffer.concat([
      PNG_MAGIC,
      Buffer.from([0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52]),
      Buffer.from(`[info] sample=${jwt}\n`, 'latin1'),
    ]),
  );

  const prepared = run('prepare（真实 PNG + 内嵌 JWT）', PREPARE, ['--source', src, '--dest', dest]);
  check('prepare 退出 0 且确实收集该 PNG',
    prepared.code === 0 && prepared.stdout.includes('real-with-text.png'));
  const scanned = run('scan（真实 PNG + 内嵌 JWT）', SCANNER, [dest]);
  check('scan 退出 2（魔数符合仍被文本规则阻断）', scanned.code === 2,
    `实际退出码=${scanned.code}`);
  check('命中 JWT_LITERAL', scanned.stdout.includes('JWT_LITERAL'));
  check('未记 SUSPICIOUS_BINARY（证明魔数校验本身通过）',
    !scanned.stdout.includes('SUSPICIOUS_BINARY'));
  check('输出不含合成值本身', !scanned.stdout.includes(jwt) && !scanned.stderr.includes(jwt));
  log('结论：魔数符合也照样过文本规则，零跳过分支生效；该文件会被阻断、不会上传');
}

/** 场景 F：扫描器源码静态核验，排除任何残留的内容跳过分支。 */
function verifyNoSkipBranches() {
  log('');
  log('=== 场景 F · 扫描器源码零跳过分支静态核验 ===');
  const src = fs.readFileSync(SCANNER, 'utf8');
  const loopStart = src.indexOf('for (const abs of tree.files)');
  const loopEnd = src.indexOf('for (const link of tree.symlinks)');
  const loop = loopStart >= 0 && loopEnd > loopStart ? src.slice(loopStart, loopEnd) : '';
  check('主扫描循环存在且可定位', loop.length > 0, `切片长度=${loop.length}`);
  check('源码不含 looksBinary', !src.includes('looksBinary'));
  check('源码不含 binarySkipped', !src.includes('binarySkipped'));
  // 先剥离行注释，避免把说明文字里的 "continue" 误判为跳过语句。
  const loopCode = loop.replace(/\/\/[^\n]*/g, '');
  check('主扫描循环内无任何 continue 语句（零内容跳过分支）', !/\bcontinue\b/.test(loopCode));
  check('主扫描循环使用 latin1 逐字节映射', /toString\('latin1'\)/.test(loop));
  check('魔数是额外命中条件而非放行通行证',
    /binarySignatureMismatch[\s\S]{0,120}addFinding\(rel, 'SUSPICIOUS_BINARY'/.test(src));
  check('统计口径如实：跳过内容扫描=0', src.includes('跳过内容扫描=0'));
  check('MAX_TEXT_BYTES 超限仍抛错（退出 1）',
    /stat\.size > MAX_TEXT_BYTES[\s\S]{0,160}throw new Error/.test(src));
  log('结论：交付态扫描器不存在任何「静默跳过内容扫描后返回 CLEAN」的分支');
}

try {
  verifyNulPrefixedLog();
  verifyTextDisguisedPng();
  verifyNoFalsePositive();
  verifyRealPngWithEmbeddedJwt();
  verifyNoSkipBranches();
  verifyCleanChain('r0-qa-verify', path.join(REPO_ROOT, 'artifacts', 'r0-qa-verify'));
  verifyCleanChain('r0-e2e', path.join(REPO_ROOT, 'artifacts', 'r0-e2e'));
} catch (err) {
  failures += 1;
  log(`[FAIL] 自证过程异常：${err.message}`);
} finally {
  fs.rmSync(TMP_ROOT, { recursive: true, force: true });
}

log('');
log(`Round 2 端到端自证总结论：${failures === 0 ? 'PASS' : 'FAIL'}（失败项=${failures}）`);
process.exitCode = failures === 0 ? 0 : 1;
