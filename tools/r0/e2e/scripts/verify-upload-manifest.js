#!/usr/bin/env node
'use strict';

/**
 * FastLedger R0 E2E —— 最终上传目录「递归实际字节校验」独立工具（RV-13）。
 *
 * 与 prepare-upload.js 共用 lib/manifest.js 的同一套口径，因此可独立复验：
 *   · CI 生成并上传的 Artifact 下载解压后的目录；
 *   · 被故意篡改 / 删除文件后的目录（必须报错，证明校验真实生效）。
 *
 * 校验内容：重新遍历目录、重新读取每个文件字节、重新计算 SHA256，与唯一权威根清单
 * 逐条比对；并检查 missing / mismatch / uncovered / staleRefs / nestedManifests /
 * selfRef / malformed / 符号链接。不是只校验清单格式或记录条数。
 *
 * 用法：node verify-upload-manifest.js <最终上传目录>
 * 退出码：0 = 校验通过 / 2 = 校验失败（禁止上传）/ 1 = 参数或环境错误。
 */

const fs = require('node:fs');
const path = require('node:path');
const manifest = require('./lib/manifest');

const EXIT_OK = 0;
const EXIT_ERROR = 1;
const EXIT_FAILED = 2;

function main(argv) {
  const dirs = argv.filter((a) => !a.startsWith('--'));
  if (dirs.length !== 1 || argv.includes('--help') || argv.includes('-h')) {
    process.stdout.write('用法：node verify-upload-manifest.js <最终上传目录>\n');
    process.stdout.write('退出码：0 校验通过 / 2 校验失败（禁止上传）/ 1 参数或环境错误\n');
    return argv.includes('--help') || argv.includes('-h') ? EXIT_OK : EXIT_ERROR;
  }
  const dest = path.resolve(process.cwd(), dirs[0]);
  if (!fs.existsSync(dest) || !fs.statSync(dest).isDirectory()) {
    process.stderr.write(`[verify][ERROR] 目录不存在或不是目录：${dest}\n`);
    return EXIT_ERROR;
  }

  let r;
  try {
    r = manifest.verifyUploadTree(dest);
  } catch (err) {
    process.stderr.write(`[verify][ERROR] 校验异常：${err.message}\n`);
    return EXIT_ERROR;
  }

  process.stdout.write(`[verify] 待校验目录：${dest}\n`);
  for (const line of manifest.formatVerification(r)) {
    process.stdout.write(`[verify] ${line}\n`);
  }
  if (r.ok) {
    process.stdout.write('[verify] 退出码：0（0 missing / 0 mismatch / 全部实际文件均有可验证 SHA256）\n');
    return EXIT_OK;
  }
  process.stdout.write('[verify] 退出码：2（校验失败，禁止上传）\n');
  return EXIT_FAILED;
}

process.exitCode = main(process.argv.slice(2));
