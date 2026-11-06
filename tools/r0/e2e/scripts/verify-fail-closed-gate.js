#!/usr/bin/env node
'use strict';

/**
 * FastLedger R0 E2E —— 失败关闭（fail-closed）上传闸门验证器。
 *
 * 复刻 .github/workflows/r0-e2e.yml 中 artifact_upload 的判定逻辑：
 *   上传仅在「上传前扫描 artifact_scan 成功」且「fail-closed 集成验证 fail_closed_check 成功」时才会发生。
 *
 * 本脚本以可复用方式验证该闸门：
 *   1. 用给定 scanner 扫描「含凭据样本目录」→ 期望 rc == 2（被阻断）；
 *   2. 用同一 scanner 扫描「干净样本目录」→ 期望 rc == 0（放行）；
 *   3. fail_closed_passed = (cred_rc == 2) && (clean_rc == 0)；上传闸门即以此为准。
 *
 * 两种运行模式：
 *   - 正向（默认）：以真实扫描器运行；fail_closed_passed 必须为 true，否则判定 FAIL-OPEN 并退出 1。
 *   - 负向（--expect-skip）：以「破损扫描器替身」运行；fail_closed_passed 必须为 false，
 *     证明验证失败时上传被跳过（Artifact 数量 = 0），否则判定负向测试失败并退出 1。
 *
 * 用法：
 *   node verify-fail-closed-gate.js --scanner <path> --cred-dir <dir> --clean-dir <dir> [--expect-skip]
 */

const { spawnSync } = require('node:child_process');

function main() {
  const argv = process.argv.slice(2);
  let scanner;
  let credDir;
  let cleanDir;
  let expectSkip = false;
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--scanner') scanner = argv[++i];
    else if (arg === '--cred-dir') credDir = argv[++i];
    else if (arg === '--clean-dir') cleanDir = argv[++i];
    else if (arg === '--expect-skip') expectSkip = true;
    else {
      process.stderr.write(`未知参数：${arg}\n`);
      return 2;
    }
  }
  if (!scanner || !credDir || !cleanDir) {
    process.stderr.write(
      '用法：verify-fail-closed-gate.js --scanner <p> --cred-dir <d> --clean-dir <d> [--expect-skip]\n',
    );
    return 2;
  }

  const credRc = spawnSync(process.execPath, [scanner, credDir], { encoding: 'utf8' }).status;
  const cleanRc = spawnSync(process.execPath, [scanner, cleanDir], { encoding: 'utf8' }).status;

  // 复刻上传闸门判定：凭据样本必须被严格阻断（rc==2），干净样本必须严格放行（rc==0）。
  const failClosedPassed = credRc === 2 && cleanRc === 0;
  const uploadAllowed = failClosedPassed;

  console.log(
    `[gate] CRED_RC=${credRc} CLEAN_RC=${cleanRc} ` +
      `fail_closed_passed=${failClosedPassed} upload_allowed=${uploadAllowed}`,
  );

  if (expectSkip) {
    if (uploadAllowed) {
      process.stderr.write(
        '::error::NEGATIVE TEST FAILED: 失败时关闭验证失败时，上传闸门仍允许上传（Artifact 数量非 0）\n',
      );
      return 1;
    }
    console.log('[gate] NEGATIVE OK: 验证失败时上传被跳过，Artifact 数量 = 0');
    return 0;
  }

  if (!uploadAllowed) {
    process.stderr.write(
      `::error::FAIL-OPEN / FAIL: 网关本应允许上传却未通过` +
        `（CRED_RC=${credRc}, CLEAN_RC=${cleanRc}）；期望凭据样本 rc=2、干净样本 rc=0\n`,
    );
    return 1;
  }
  console.log('[gate] POSITIVE OK: 失败关闭链路成立——凭据样本被阻断(rc=2)、干净样本放行(rc=0)，上传允许');
  return 0;
}

process.exitCode = main();
