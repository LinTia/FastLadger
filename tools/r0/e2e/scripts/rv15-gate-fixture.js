#!/usr/bin/env node
'use strict';

/**
 * FastLedger R0 E2E —— RV-15 workflow 级上传门禁「可控输入」驱动器。
 *
 * 目的：在真实 GitHub Actions run 里，让作为 upload 前置条件的真实 step 真正失败，
 * 从而观察真实 `actions/upload-artifact` step 的 outcome，并用 Artifact API 核对数量。
 * 这与 RV-08 被复核批评的「helper 内部布尔值模拟」不同：这里失败的是 gate 依赖的
 * 同一个 step（artifact_prepare / artifact_scan / fail_closed_check），不是旁路 helper。
 *
 * 子命令：
 *   prepare --scenario <S> --source <dir> --dest <dir>
 *     · positive / fail_closed_fail：构造干净合成证据源目录，真实运行 prepare-upload.js（rc=0）
 *     · scan_fail：源目录含合成凭据 leak.txt（白名单 .txt），prepare 成功后真实扫描器会 rc=2
 *     · prepare_fail：把 --source 指向不存在的目录，真实运行 prepare-upload.js → rc=1（真实失败）
 *   gate --scenario <S>
 *     · positive / scan_fail / prepare_fail：用真实扫描器运行 verify-fail-closed-gate.js → rc=0
 *     · fail_closed_fail：把 gate 的 --scanner 指向「永远返回 CLEAN 的破损扫描器」，
 *       真实 gate 判定 FAIL-OPEN 并退出 1 → fail_closed_check step 真实失败
 *
 * 所有凭据均为运行期拼接的 SYNTHETIC/FAKE 合成串，无真实 token / trace / HAR / 数据库。
 * 用法：node rv15-gate-fixture.js <prepare|gate> --scenario <S> [--source <d>] [--dest <d>]
 */

const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { spawnSync } = require('node:child_process');
const { PNG_MAGIC } = require('./lib/png-fixture');

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..', '..');
const PREPARE = path.join(REPO_ROOT, 'tools/r0/e2e/scripts/prepare-upload.js');
const SCANNER = path.join(REPO_ROOT, 'tools/r0/e2e/scripts/scan-artifacts.js');
const GATE = path.join(REPO_ROOT, 'tools/r0/e2e/scripts/verify-fail-closed-gate.js');
const BROKEN = path.join(REPO_ROOT, 'tools/r0/e2e/scripts/fixtures/broken-scanner.js');

const SYNTHETIC_KEY = ['x-api-key', ': ', 'SYNTHETIC', 'FAKE', 'GATEKEY', '000000000000'].join('');

function parseArgs(argv) {
  const out = { _: [] };
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i].startsWith('--')) out[argv[i].slice(2)] = argv[i + 1] && !argv[i + 1].startsWith('--') ? argv[++i] : true;
    else out._.push(argv[i]);
  }
  return out;
}

function makeCleanSource(src) {
  fs.rmSync(src, { recursive: true, force: true });
  fs.mkdirSync(src, { recursive: true });
  fs.writeFileSync(path.join(src, 'report.md'), '# R0 gate fixture\nstatus: ok\n', 'utf8');
  fs.writeFileSync(path.join(src, 'report.json'), `${JSON.stringify({ gate: 'positive', passed: true }, null, 2)}\n`, 'utf8');
  fs.writeFileSync(path.join(src, 'run.log'), '[info] synthetic gate run finished\n', 'utf8');
  fs.writeFileSync(path.join(src, 'shot.png'), Buffer.concat([PNG_MAGIC, Buffer.from([0, 0, 0, 13, 0x49, 0x48, 0x44, 0x52])]));
}

function cmdPrepare(a) {
  const scenario = a.scenario;
  const source = path.resolve(process.cwd(), a.source);
  const dest = path.resolve(process.cwd(), a.dest);

  if (scenario === 'prepare_fail') {
    // 真实 prepare-upload.js 对不存在的源目录返回 EXIT_ERROR(1) → step 真实失败。
    const missing = `${source}-does-not-exist-rv15`;
    process.stdout.write(`[rv15] scenario=prepare_fail：以不存在的源目录调用真实 prepare-upload.js → 期望 rc=1\n`);
    const r = spawnSync(process.execPath, [PREPARE, '--source', missing, '--dest', dest], { stdio: 'inherit' });
    process.stdout.write(`[rv15] prepare-upload.js 实际退出码=${r.status}\n`);
    return r.status === null ? 1 : r.status;
  }

  makeCleanSource(source);
  if (scenario === 'scan_fail') {
    // 合成凭据落在白名单 .txt 中，prepare 会正常复制；随后真实扫描器应 rc=2。
    fs.writeFileSync(path.join(source, 'leak.txt'), `${SYNTHETIC_KEY}\n`, 'utf8');
    process.stdout.write(`[rv15] scenario=scan_fail：源目录注入合成凭据（白名单 .txt），真实扫描器应 rc=2\n`);
  } else {
    process.stdout.write(`[rv15] scenario=${scenario}：构造干净合成证据源目录\n`);
  }
  const r = spawnSync(process.execPath, [PREPARE, '--source', source, '--dest', dest], { stdio: 'inherit' });
  process.stdout.write(`[rv15] prepare-upload.js 实际退出码=${r.status}\n`);
  return r.status === null ? 1 : r.status;
}

function cmdGate(a) {
  const scenario = a.scenario;
  const work = fs.mkdtempSync(path.join(os.tmpdir(), 'rv15-gate-'));
  const cred = path.join(work, 'cred');
  const clean = path.join(work, 'clean');
  fs.mkdirSync(cred, { recursive: true });
  fs.mkdirSync(clean, { recursive: true });
  fs.writeFileSync(path.join(cred, 'leak.txt'), `${SYNTHETIC_KEY}\n`, 'utf8');
  fs.writeFileSync(path.join(clean, 'biz.json'), 'unit: kg\nnote: groceries\nstatus: ok\n', 'utf8');

  const scanner = scenario === 'fail_closed_fail' ? BROKEN : SCANNER;
  process.stdout.write(
    `[rv15] scenario=${scenario}：fail_closed_check 使用 ${scenario === 'fail_closed_fail' ? '破损扫描器替身（gate 应判 FAIL-OPEN → rc=1）' : '真实扫描器（rc=0）'}\n`,
  );
  const r = spawnSync(
    process.execPath,
    [GATE, '--scanner', scanner, '--cred-dir', cred, '--clean-dir', clean],
    { stdio: 'inherit' },
  );
  process.stdout.write(`[rv15] verify-fail-closed-gate.js 实际退出码=${r.status}\n`);
  fs.rmSync(work, { recursive: true, force: true });
  return r.status === null ? 1 : r.status;
}

function main() {
  const a = parseArgs(process.argv.slice(2));
  const cmd = a._[0];
  const scenario = a.scenario;
  const allowed = ['positive', 'scan_fail', 'fail_closed_fail', 'prepare_fail'];
  if (!cmd || !allowed.includes(scenario)) {
    process.stderr.write('用法：rv15-gate-fixture.js <prepare|gate> --scenario <positive|scan_fail|fail_closed_fail|prepare_fail> [--source <d>] [--dest <d>]\n');
    return 1;
  }
  if (cmd === 'prepare') return cmdPrepare(a);
  if (cmd === 'gate') return cmdGate(a);
  process.stderr.write(`未知子命令：${cmd}\n`);
  return 1;
}

process.exitCode = main();
