#!/usr/bin/env node
'use strict';

/**
 * FastLedger R0 E2E —— RV-14 修复前后对照证据生成器（可复现）。
 *
 * 从 git 基线 commit（默认 2699c9c，第九次复核时的 PR HEAD）用 `git show` 取出
 * 修复前扫描器，与当前修复后扫描器对同一批「合法 PNG 魔数」合成样本运行，
 * 打印两者的原始退出码与命中类型，证明：
 *   · 修复前：knownBinary 跳过噪声复扫 → CLEAN（rc=0，绕过成立）
 *   · 修复后：全部 BLOCKED（rc=2）
 *   · 正常 PNG / 普通业务文字 PNG：修复前后都 CLEAN（无误报放大）
 *   · 扫描输出绝不含合成匹配值
 *
 * 所有样本运行期生成于 os.tmpdir()，结束即删除；合成凭据带 SYNTHETIC/FAKE 标记，
 * 无真实凭据、无真实 trace/HAR/数据库。
 *
 * 用法：node tools/r0/e2e/scripts/rv14-binary-evidence.js [基线commit]
 * 退出码：0 = 全部断言通过 / 1 = 存在失败。
 */

const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { makePng, highEntropyBytes } = require('./lib/png-fixture');

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..', '..');
const NEW_SCANNER = path.join(REPO_ROOT, 'tools/r0/e2e/scripts/scan-artifacts.js');
const OLD_SCANNER = path.join(os.tmpdir(), 'rv14-old-scan-artifacts.js');
const BASELINE = process.argv[2] || '2699c9c3db2e40915082da2acbe2392ad40650de';

const NUL = String.fromCharCode(0);
const TOK = ['SYNTHETIC', 'FAKE', '000000000000'].join('');

const samples = [
  ['RV-14-01 PNG 明文 Authorization/Bearer', makePng({ text: ['Authorization', ': ', 'Bearer', ' ', TOK].join('') }), 2],
  ['RV-14-02 PNG token 值内部 NUL', makePng({ text: ['Bearer ', 'SYN', NUL, 'THETIC', TOK].join('') }), 2],
  ['RV-14-03 PNG Bearer 关键词内部 NUL', makePng({ text: ['Bea', NUL, 'rer ', TOK].join('') }), 2],
  ['RV-14-04 PNG x-api-key', makePng({ text: ['x-api-key', ': ', TOK].join('') }), 2],
  ['RV-14-05 正常 PNG（含 4KiB 高熵压缩数据）', makePng({ highEntropyTail: highEntropyBytes(4096) }), 0],
  ['RV-14-06 PNG 普通业务文字（不误报）', makePng({ text: 'unit: kg\nnote: groceries\nstatus: ok\nqty: 2\n' }), 0],
];

/** 运行扫描器，返回退出码与输出。 */
function run(scanner, dir) {
  const r = spawnSync(process.execPath, [scanner, dir], { encoding: 'utf8' });
  const out = `${r.stdout || ''}\n${r.stderr || ''}`;
  const hits = (r.stdout || '').split('\n')
    .filter((l) => l.startsWith('[HIT]') && !l.includes('（无）'))
    .map((l) => l.replace('[HIT] ', ''));
  return { rc: r.status, out, hits };
}

function main() {
  const checks = [];
  const check = (name, ok, detail = '') => {
    checks.push({ name, ok });
    process.stdout.write(`${ok ? 'PASS' : 'FAIL'} | ${name}${detail ? ` | ${detail}` : ''}\n`);
  };

  // 取出修复前扫描器
  const extract = spawnSync(
    'git', ['show', `${BASELINE}:tools/r0/e2e/scripts/scan-artifacts.js`],
    { cwd: REPO_ROOT, encoding: 'utf8' },
  );
  if (extract.status !== 0) {
    process.stdout.write(`无法从基线 ${BASELINE} 提取旧扫描器：${extract.stderr}\n`);
    return 1;
  }
  fs.writeFileSync(OLD_SCANNER, extract.stdout);

  process.stdout.write('=== RV-14 修复前后对照证据（合法 PNG 魔数样本，合成凭据） ===\n');
  process.stdout.write(`基线 commit：${BASELINE}（第九次复核时的 PR HEAD）\n`);
  process.stdout.write(`修复前扫描器：${OLD_SCANNER}\n`);
  process.stdout.write(`修复后扫描器：${NEW_SCANNER}\n\n`);

  const work = fs.mkdtempSync(path.join(os.tmpdir(), 'rv14-evidence-'));
  try {
    for (const [name, buf, expectedNew] of samples) {
      const dir = fs.mkdtempSync(path.join(work, 'case-'));
      fs.writeFileSync(path.join(dir, 'sample.png'), buf);
      const oldR = run(OLD_SCANNER, dir);
      const newR = run(NEW_SCANNER, dir);
      const oldVerdict = oldR.out.includes('结论：CLEAN') ? 'CLEAN(bypass)' : (oldR.rc === 2 ? 'BLOCKED' : `rc=${oldR.rc}`);
      const newVerdict = newR.out.includes('结论：CLEAN') ? 'CLEAN' : (newR.rc === 2 ? 'BLOCKED' : `rc=${newR.rc}`);
      process.stdout.write(`${name}\n`);
      process.stdout.write(`  old(${BASELINE.slice(0, 7)}) rc=${oldR.rc} verdict=${oldVerdict} hits=${oldR.hits.join(' | ') || '(none)'}\n`);
      process.stdout.write(`  new(fixed)  rc=${newR.rc} verdict=${newVerdict} hits=${newR.hits.join(' | ') || '(none)'}\n`);

      if (expectedNew === 2) {
        check(`${name} → 修复后 BLOCKED（rc=2）`, newR.rc === 2, `rc=${newR.rc}`);
      } else {
        check(`${name} → 修复后 CLEAN（rc=0，无误报）`, newR.rc === 0, `rc=${newR.rc}`);
      }
      // 输出绝不含真实/合成匹配值本身
      const leaked = [TOK, 'SYNTHETIC', 'FAKE', 'x-api-key'].some((v) => newR.out.includes(v));
      check(`${name} → 扫描输出不含匹配值`, !leaked);
      // 对阻断场景，额外记录修复前是否为 CLEAN（绕过实证；不强制，因明文样本旧版 latin1 可能已捕获）
      if (expectedNew === 2 && oldR.rc === 0) {
        process.stdout.write(`  [绕过实证] 修复前 CLEAN(rc=0)，修复后 BLOCKED(rc=2)\n`);
      }
      process.stdout.write('\n');
      fs.rmSync(dir, { recursive: true, force: true });
    }

    // 汇总：至少要有 NUL 插入类样本在修复前确为 CLEAN（RV-14 盲区实证）
    const bypassCount = samples.filter(([, , e]) => e === 2).filter(([, buf]) => {
      const dir = fs.mkdtempSync(path.join(work, 'bypass-'));
      fs.writeFileSync(path.join(dir, 'sample.png'), buf);
      const r = run(OLD_SCANNER, dir);
      fs.rmSync(dir, { recursive: true, force: true });
      return r.rc === 0;
    }).length;
    check(`修复前 CLEAN 绕过样本数 >= 2（RV-14 盲区真实可复现）`, bypassCount >= 2, `count=${bypassCount}`);
  } finally {
    fs.rmSync(work, { recursive: true, force: true });
    fs.rmSync(OLD_SCANNER, { force: true });
  }

  const failed = checks.filter((c) => !c.ok).length;
  process.stdout.write('=== 验收断言汇总 ===\n');
  for (const c of checks) process.stdout.write(`  ${c.ok ? 'PASS' : 'FAIL'} | ${c.name}\n`);
  process.stdout.write(`  合计：${checks.length} 项，PASS ${checks.length - failed}，FAIL ${failed}\n`);
  process.stdout.write(`  证据脚本退出码：${failed === 0 ? 0 : 1}\n`);
  return failed === 0 ? 0 : 1;
}

process.exitCode = main();
