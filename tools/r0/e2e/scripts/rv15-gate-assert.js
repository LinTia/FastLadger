#!/usr/bin/env node
'use strict';

/**
 * FastLedger R0 E2E —— RV-15 真实 GitHub Actions step outcome 断言。
 *
 * 本脚本在真实 workflow run 内运行，输入是 GitHub Actions 表达式
 * `${{ steps.<id>.outcome }}` 的实测值（由 workflow 通过环境变量传入），
 * 不是本地模拟的布尔值。断言「upload 前置条件失败时，真实 upload step 必须 skipped」。
 *
 * 用法（环境变量）：
 *   RV15_SCENARIO=positive|scan_fail|fail_closed_fail|prepare_fail
 *   RV15_OUTCOME_PREPARE / RV15_OUTCOME_SCAN / RV15_OUTCOME_FC / RV15_OUTCOME_UPLOAD
 * 退出码：0 = 全部断言通过 / 1 = 存在断言失败。
 */

const REQUIRED = {
  positive: { prepare: 'success', scan: 'success', fc: 'success', upload: 'success' },
  scan_fail: { prepare: 'success', scan: 'failure', fc: 'success', upload: 'skipped' },
  fail_closed_fail: { prepare: 'success', scan: 'success', fc: 'failure', upload: 'skipped' },
  prepare_fail: { prepare: 'failure', scan: 'skipped', fc: 'success', upload: 'skipped' },
};

function main() {
  const scenario = process.env.RV15_SCENARIO || '';
  const actual = {
    prepare: process.env.RV15_OUTCOME_PREPARE || '(unset)',
    scan: process.env.RV15_OUTCOME_SCAN || '(unset)',
    fc: process.env.RV15_OUTCOME_FC || '(unset)',
    upload: process.env.RV15_OUTCOME_UPLOAD || '(unset)',
  };
  const expected = REQUIRED[scenario];
  if (!expected) {
    process.stderr.write(`未知场景：${scenario}\n`);
    return 1;
  }

  process.stdout.write('=== RV-15 真实 GitHub Actions step outcome 断言 ===\n');
  process.stdout.write(`scenario=${scenario}\n`);
  let failed = 0;
  for (const key of ['prepare', 'scan', 'fc', 'upload']) {
    const ok = actual[key] === expected[key];
    if (!ok) failed += 1;
    process.stdout.write(
      `  ${ok ? 'PASS' : 'FAIL'} | ${key.padEnd(7)} 实测=${String(actual[key]).padEnd(9)} 期望=${expected[key]}\n`,
    );
  }
  // 关键结论：失败路径不得允许上传（真实 upload step 必须 skipped）
  if (scenario !== 'positive') {
    const noUpload = actual.upload === 'skipped';
    if (!noUpload) failed += 1;
    process.stdout.write(`  ${noUpload ? 'PASS' : 'FAIL'} | 失败路径真实 upload step 被跳过（不得产生 Artifact）\n`);
  } else {
    const uploaded = actual.upload === 'success';
    if (!uploaded) failed += 1;
    process.stdout.write(`  ${uploaded ? 'PASS' : 'FAIL'} | 正向场景真实 upload step 执行成功（应产生 Artifact）\n`);
  }
  process.stdout.write(`结论：${failed === 0 ? 'PASS' : 'FAIL'}（失败断言 ${failed} 项）\n`);
  return failed === 0 ? 0 : 1;
}

process.exitCode = main();
