#!/usr/bin/env node
'use strict';

/**
 * FastLedger R0 E2E —— RV-13 原始证据生成器（可复现）。
 *
 * 真实执行 prepare-upload.js 与 verify-upload-manifest.js（子进程），打印：
 *   1. 构造的多层源目录清单（含各级旧 SHA256SUMS、trace.zip 及其它被排除文件）
 *   2. prepare-upload.js 的原始 stdout / stderr / 退出码
 *   3. 最终上传目录的完整递归文件清单
 *   4. verify-upload-manifest.js 的原始输出 / 退出码（0 missing / 0 mismatch）
 *   5. 故意篡改一个最终文件 → 递归校验必须失败（退出码 2）
 *   6. 故意删除一个最终文件 → 递归校验必须失败（退出码 2）
 *   7. 恢复后再次校验通过
 *   8. 逐项验收断言汇总
 *
 * 用法：node tools/r0/e2e/scripts/rv13-manifest-evidence.js
 * 退出码：0 = 全部验收通过 / 1 = 存在验收失败。
 * 所有临时数据写入 os.tmpdir()，结束即删除；合成数据，无真实凭据 / trace / 数据库。
 */

const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const manifest = require('./lib/manifest');
const { buildSource, STALE_TRACE_REFS } = require('./lib/rv13-fixture');

const SCRIPTS = __dirname;
const PREPARE = path.join(SCRIPTS, 'prepare-upload.js');
const VERIFY = path.join(SCRIPTS, 'verify-upload-manifest.js');

const checks = [];
function check(name, ok, detail = '') {
  checks.push({ name, ok, detail });
  process.stdout.write(`${ok ? 'PASS' : 'FAIL'} | ${name}${detail ? ` | ${detail}` : ''}\n`);
}

function run(script, args) {
  return spawnSync(process.execPath, [script, ...args], { encoding: 'utf8' });
}

function listRecursive(dir) {
  return manifest.walkFiles(dir).files;
}

function printRaw(tag, r) {
  if (r.stdout) process.stdout.write(r.stdout.split('\n').map((l) => `  [${tag}.stdout] ${l}`).join('\n') + '\n');
  if (r.stderr) process.stdout.write(r.stderr.split('\n').map((l) => `  [${tag}.stderr] ${l}`).join('\n') + '\n');
  process.stdout.write(`  [${tag}.exit_code] ${r.status}\n`);
}

function main() {
  const work = fs.mkdtempSync(path.join(os.tmpdir(), 'rv13-evidence-'));
  const src = path.join(work, 'source');
  const dest = path.join(work, 'upload');
  let failures = 0;

  try {
    const fixture = buildSource(src);
    process.stdout.write('=== RV-13 原始证据（合成数据，递归实际字节校验） ===\n\n');
    process.stdout.write('--- [1] 构造的多层源目录 ---\n');
    const srcAll = listRecursive(src);
    process.stdout.write(`  源文件总数=${srcAll.length}\n`);
    for (const rel of srcAll) process.stdout.write(`  - ${rel}\n`);
    process.stdout.write(`  旧清单（应全部被排除）：根 SHA256SUMS.txt + A/D/C/SHA256SUMS.txt 共 4 份\n`);
    process.stdout.write(`  失效引用（指向已排除 trace.zip，镜像复核报告 A 子目录 12 条）：${STALE_TRACE_REFS.length} 条\n`);
    process.stdout.write(`  被排除文件：${fixture.excludedFiles.join(', ')}\n\n`);

    // --- [2] 真实执行 prepare-upload.js ---
    process.stdout.write('--- [2] 真实执行 prepare-upload.js（白名单收敛 + 根清单生成 + 递归字节校验） ---\n');
    const prep = run(PREPARE, ['--source', src, '--dest', dest]);
    printRaw('prepare', prep);
    check('prepare-upload.js 退出码 = 0', prep.status === 0, `rc=${prep.status}`);
    if (prep.status !== 0) failures += 1;

    // --- [3] 最终上传目录完整清单 ---
    process.stdout.write('\n--- [3] 最终上传目录递归文件清单 ---\n');
    const finalFiles = listRecursive(dest);
    process.stdout.write(`  最终文件总数=${finalFiles.length}\n`);
    for (const rel of finalFiles) process.stdout.write(`  - ${rel}\n`);
    const nested = finalFiles.filter((rel) => rel !== manifest.MANIFEST_NAME && manifest.isManifestName(rel));
    const rootManifestText = fs.readFileSync(path.join(dest, manifest.MANIFEST_NAME), 'utf8');
    process.stdout.write(`  根清单内容（${rootManifestText.trim().split('\n').length} 行，仅相对路径 + 摘要）：\n`);
    for (const l of rootManifestText.trim().split('\n')) process.stdout.write(`    ${l}\n`);
    process.stdout.write(`  根清单自身处理规则：${manifest.MANIFEST_NAME} 为唯一例外，不列出自身、不哈希自身\n\n`);

    // --- [4] 独立递归校验 ---
    process.stdout.write('--- [4] 独立执行 verify-upload-manifest.js（递归实际字节校验） ---\n');
    const v1 = run(VERIFY, [dest]);
    printRaw('verify-initial', v1);
    check('初始递归校验退出码 = 0', v1.status === 0, `rc=${v1.status}`);
    if (v1.status !== 0) failures += 1;

    const r1 = manifest.verifyUploadTree(dest);
    check('0 missing', r1.missing.length === 0, `missing=${r1.missing.length}`);
    check('0 mismatch', r1.mismatch.length === 0, `mismatch=${r1.mismatch.length}`);
    check('0 uncovered（所有实际文件均有可验证 SHA256）', r1.uncovered.length === 0, `uncovered=${r1.uncovered.length}`);
    check('0 staleRefs（无指向 trace.zip / 其它被排除文件的引用）', r1.staleRefs.length === 0, `staleRefs=${r1.staleRefs.length}`);
    check('0 nestedManifests（无嵌套/旧清单残留）', r1.nestedManifests.length === 0, `nested=${r1.nestedManifests.length}`);
    check('无自引用 / 循环引用', !r1.selfRef && !rootManifestText.includes(`  ${manifest.MANIFEST_NAME}`), `selfRef=${r1.selfRef}`);
    check('无 trace.zip / .db / .har / server.log 等被排除文件进入最终目录',
      fixture.excludedFiles.every((f) => !finalFiles.includes(f)),
      `excluded_present=${fixture.excludedFiles.filter((f) => finalFiles.includes(f)).join(',') || 'none'}`);
    check('预期普通文件全部进入最终目录',
      fixture.expectedFiles.every((f) => finalFiles.includes(f)),
      `expected=${fixture.expectedFiles.length}`);
    check('根清单条目数 = 最终文件数 - 1（根清单自身为唯一例外）',
      r1.entries === finalFiles.length - 1, `entries=${r1.entries} files=${finalFiles.length}`);

    // --- [5] 故意篡改一个最终文件 → 校验必须失败 ---
    process.stdout.write('\n--- [5] 负向：故意篡改一个最终文件 → 递归字节校验必须失败 ---\n');
    const tamperRel = 'A/a-report.md';
    const tamperAbs = path.join(dest, ...tamperRel.split('/'));
    const original = fs.readFileSync(tamperAbs);
    fs.writeFileSync(tamperAbs, `${original.toString('utf8')}\nTAMPERED-BY-RV13-EVIDENCE\n`);
    process.stdout.write(`  篡改文件：${tamperRel}（追加一行文本，改变实际字节）\n`);
    const v2 = run(VERIFY, [dest]);
    printRaw('verify-tampered', v2);
    const r2 = manifest.verifyUploadTree(dest);
    check('篡改后校验退出码 = 2（失败，禁止上传）', v2.status === 2, `rc=${v2.status}`);
    check('篡改后 mismatch >= 1', r2.mismatch.length >= 1, `mismatch=${r2.mismatch.length}`);
    if (v2.status !== 2 || r2.mismatch.length < 1) failures += 1;
    fs.writeFileSync(tamperAbs, original); // 恢复

    // --- [6] 故意删除一个最终文件 → 校验必须失败 ---
    process.stdout.write('\n--- [6] 负向：故意删除一个最终文件 → 递归字节校验必须失败 ---\n');
    const deleteRel = 'D/d-report.md';
    const deleteAbs = path.join(dest, ...deleteRel.split('/'));
    const saved = fs.readFileSync(deleteAbs);
    fs.rmSync(deleteAbs);
    process.stdout.write(`  删除文件：${deleteRel}（清单仍保留其条目 → 应报 missing，且文件不再被覆盖）\n`);
    const v3 = run(VERIFY, [dest]);
    printRaw('verify-deleted', v3);
    const r3 = manifest.verifyUploadTree(dest);
    check('删除后校验退出码 = 2（失败，禁止上传）', v3.status === 2, `rc=${v3.status}`);
    check('删除后 missing >= 1', r3.missing.length >= 1, `missing=${r3.missing.length}`);
    if (v3.status !== 2 || r3.missing.length < 1) failures += 1;
    fs.writeFileSync(deleteAbs, saved); // 恢复

    // --- [7] 恢复后再次校验 ---
    process.stdout.write('\n--- [7] 恢复后再次独立校验 ---\n');
    const v4 = run(VERIFY, [dest]);
    printRaw('verify-restored', v4);
    check('恢复后校验退出码 = 0', v4.status === 0, `rc=${v4.status}`);
    if (v4.status !== 0) failures += 1;

    process.stdout.write('\n=== 验收断言汇总 ===\n');
    for (const c of checks) process.stdout.write(`  ${c.ok ? 'PASS' : 'FAIL'} | ${c.name}\n`);
    const failed = checks.filter((c) => !c.ok).length;
    process.stdout.write(`  合计：${checks.length} 项，PASS ${checks.length - failed}，FAIL ${failed}\n`);
    process.stdout.write(`  证据脚本退出码：${failed === 0 ? 0 : 1}\n`);
    return failed === 0 ? 0 : 1;
  } finally {
    fs.rmSync(work, { recursive: true, force: true });
  }
}

process.exitCode = main();
