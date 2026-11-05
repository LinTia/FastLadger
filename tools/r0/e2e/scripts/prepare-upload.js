#!/usr/bin/env node
'use strict';

/**
 * FastLedger R0 E2E —— 上传证据白名单收集器（P1 安全整改）。
 *
 * 背景：public 仓库的 CI 曾把 Playwright trace 归档（其中的网络记录含临时实例
 * 真实鉴权头明文）原样上传为公开 artifact。本脚本在上传前把证据收敛为一份
 * 「白名单副本」，从源头剔除一切可能携带网络原始记录或数据库内容的载体。
 *
 * 设计要点：
 *   1. 零第三方依赖，仅用 node:fs / node:path，Windows 本地与 Linux CI 均可运行；
 *   2. 只复制白名单扩展名 *.png / *.md / *.json / *.log / *.txt；
 *   3. 显式排除 *.zip（含 trace.zip）、*.trace、*.network、*.har、server.log、
 *      *.db / *.sqlite*，以及文件名含 token|secret|credential 的文件；
 *      （敏感词规则采用更严格口径：即便扩展名在白名单内也一律排除。）
 *   4. 每次运行先清空目标目录，避免上一轮残留被再次上传；
 *   5. 不跟随符号链接，避免目录逃逸；
 *   6. 日志只输出数量、相对文件名与排除原因，**绝不输出文件内容**。
 *
 * 用法：
 *   node tools/r0/e2e/scripts/prepare-upload.js
 *   node tools/r0/e2e/scripts/prepare-upload.js --source <dir> --dest <dir>
 *   node tools/r0/e2e/scripts/prepare-upload.js --help
 *
 * 退出码：0 成功 / 1 参数或环境错误。
 */

const fs = require('node:fs');
const path = require('node:path');

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..', '..');
const DEFAULT_SOURCE = path.join(REPO_ROOT, 'artifacts', 'r0-e2e');
const DEFAULT_DEST = path.join(REPO_ROOT, 'artifacts', 'r0-e2e-upload');

/** 唯一允许进入上传目录的扩展名（小写，含点）。 */
const ALLOWED_EXT = new Set(['.png', '.md', '.json', '.log', '.txt']);

/** 一律拒绝的扩展名：压缩归档、网络原始记录与数据库文件。 */
const DENIED_EXT = new Set([
  '.zip', '.trace', '.network', '.har',
  '.db', '.sqlite', '.sqlite3', '.db-wal', '.db-shm',
]);

/** 文件名（basename）命中即拒绝，与扩展名无关。 */
const SENSITIVE_NAME_RE = /(?:token|secret|credential)/i;

/** 明确点名的服务原始日志。 */
const SERVER_LOG_NAME = 'server.log';

const EXIT_OK = 0;
const EXIT_ERROR = 1;

/**
 * 把绝对路径转换为可安全打印的仓库相对路径，不泄露个人绝对路径。
 * @param {string} abs 绝对路径。
 * @return {string} 相对路径；仓库外文件只显示 basename。
 */
function displayPath(abs) {
  const rel = path.relative(REPO_ROOT, abs).split(path.sep).join('/');
  if (rel.startsWith('..')) return `<仓库外:${path.basename(abs)}>`;
  return rel || '.';
}

/** 打印用法说明。 */
function printHelp() {
  process.stdout.write([
    '用法：node tools/r0/e2e/scripts/prepare-upload.js [选项]',
    '',
    '  --source <dir>  源证据目录，默认 artifacts/r0-e2e',
    '  --dest   <dir>  白名单输出目录，默认 artifacts/r0-e2e-upload（运行前清空）',
    '  --help          显示本说明',
    '',
    `白名单：${[...ALLOWED_EXT].join(' ')}`,
    '排除：trace 归档与任意 zip、trace/network/har 原始记录、server.log、',
    '      db/sqlite 数据库文件、文件名含 token|secret|credential 的文件。',
    '',
  ].join('\n'));
}

/**
 * 解析命令行参数。
 * @param {string[]} argv process.argv.slice(2)。
 * @return {{source: string, dest: string, help: boolean}} 解析结果。
 * @throws {Error} 参数非法时抛出。
 */
function parseArgs(argv) {
  let source = DEFAULT_SOURCE;
  let dest = DEFAULT_DEST;
  let help = false;

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--help' || arg === '-h') {
      help = true;
      continue;
    }
    if (arg === '--source' || arg === '--dest') {
      const value = argv[i + 1];
      if (!value || value.startsWith('--')) throw new Error(`参数 ${arg} 缺少取值`);
      const resolved = path.resolve(process.cwd(), value);
      if (arg === '--source') source = resolved;
      else dest = resolved;
      i += 1;
      continue;
    }
    throw new Error(`未知参数：${arg}`);
  }
  return { source, dest, help };
}

/**
 * 判定单个文件是否允许进入上传目录。先判黑名单，再判白名单。
 * @param {string} rel 相对路径。
 * @return {{allowed: boolean, reason: string}} 判定结果。
 */
function classify(rel) {
  const base = path.basename(rel);
  const lower = base.toLowerCase();
  const ext = path.extname(lower);

  if (ext === '.zip' && lower.includes('trace')) {
    return { allowed: false, reason: 'Playwright trace 归档（可能含网络请求明文）' };
  }
  if (DENIED_EXT.has(ext)) {
    return { allowed: false, reason: `禁止扩展名 ${ext}` };
  }
  if (lower === SERVER_LOG_NAME) {
    return { allowed: false, reason: 'server.log 服务原始日志' };
  }
  if (SENSITIVE_NAME_RE.test(base)) {
    return { allowed: false, reason: '文件名含 token/secret/credential 敏感词' };
  }
  if (!ALLOWED_EXT.has(ext)) {
    return { allowed: false, reason: `非白名单扩展名 ${ext || '(无扩展名)'}` };
  }
  return { allowed: true, reason: '' };
}

/**
 * 递归收集文件；符号链接一律跳过，不跟随。
 * @param {string} dir 目录绝对路径。
 * @param {{files: string[], links: string[]}} out 输出累加器。
 * @throws {Error} 目录不可读时抛出。
 */
function walk(dir, out) {
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch (err) {
    throw new Error(`无法读取目录 ${displayPath(dir)}：${err.message}`);
  }
  for (const entry of entries) {
    const abs = path.join(dir, entry.name);
    if (entry.isSymbolicLink()) {
      out.links.push(abs);
      continue;
    }
    if (entry.isDirectory()) {
      walk(abs, out);
      continue;
    }
    if (entry.isFile()) out.files.push(abs);
  }
}

/**
 * 校验源/目标目录组合，防止误删或递归复制。
 * @param {string} source 源目录。
 * @param {string} dest 目标目录。
 * @throws {Error} 组合非法时抛出。
 */
function validatePaths(source, dest) {
  if (source === dest) throw new Error('源目录与目标目录不能相同');
  if (dest.startsWith(source + path.sep) || source.startsWith(dest + path.sep)) {
    throw new Error('源目录与目标目录不能互相嵌套');
  }
  if (dest === REPO_ROOT || REPO_ROOT.startsWith(dest + path.sep)) {
    throw new Error('目标目录不能是仓库根或其上级目录');
  }
  if (dest === path.join(REPO_ROOT, 'artifacts')) {
    throw new Error('目标目录不能是 artifacts 根目录（清空操作会删除全部证据）');
  }
}

/**
 * 程序入口。
 * @param {string[]} argv process.argv.slice(2)。
 * @return {number} 退出码。
 */
function main(argv) {
  let options;
  try {
    options = parseArgs(argv);
  } catch (err) {
    process.stderr.write(`[prepare][ERROR] ${err.message}\n`);
    return EXIT_ERROR;
  }
  if (options.help) {
    printHelp();
    return EXIT_OK;
  }

  const { source, dest } = options;
  try {
    validatePaths(source, dest);
  } catch (err) {
    process.stderr.write(`[prepare][ERROR] ${err.message}\n`);
    return EXIT_ERROR;
  }

  let sourceStat;
  try {
    sourceStat = fs.statSync(source);
  } catch (err) {
    process.stderr.write(`[prepare][ERROR] 源目录不存在或不可访问：${displayPath(source)}\n`);
    return EXIT_ERROR;
  }
  if (!sourceStat.isDirectory()) {
    process.stderr.write(`[prepare][ERROR] 源路径不是目录：${displayPath(source)}\n`);
    return EXIT_ERROR;
  }

  const tree = { files: [], links: [] };
  try {
    walk(source, tree);
  } catch (err) {
    process.stderr.write(`[prepare][ERROR] ${err.message}\n`);
    return EXIT_ERROR;
  }

  /** @type {{rel: string, reason: string}[]} */
  const excluded = [];
  /** @type {string[]} */
  const collected = [];

  try {
    // 先清空，保证上传目录只含本轮白名单结果，杜绝上一轮残留。
    fs.rmSync(dest, { recursive: true, force: true });
    fs.mkdirSync(dest, { recursive: true });

    for (const abs of tree.files) {
      const rel = path.relative(source, abs).split(path.sep).join('/');
      const verdict = classify(rel);
      if (!verdict.allowed) {
        excluded.push({ rel, reason: verdict.reason });
        continue;
      }
      const target = path.join(dest, ...rel.split('/'));
      fs.mkdirSync(path.dirname(target), { recursive: true });
      fs.copyFileSync(abs, target);
      collected.push(rel);
    }
  } catch (err) {
    process.stderr.write(`[prepare][ERROR] 生成白名单副本失败：${err.message}\n`);
    return EXIT_ERROR;
  }

  collected.sort();
  excluded.sort((a, b) => a.rel.localeCompare(b.rel));

  const lines = [
    `[prepare] 源目录：${displayPath(source)}`,
    `[prepare] 目标目录：${displayPath(dest)}（运行前已清空）`,
    `[prepare] 白名单：${[...ALLOWED_EXT].join(' ')}`,
    `[prepare] 排除清单（共 ${excluded.length}，仅文件名与原因，不含内容）：`,
  ];
  for (const item of excluded) lines.push(`  - ${item.rel}  [${item.reason}]`);
  if (excluded.length === 0) lines.push('  （无）');
  lines.push(`[prepare] 收集清单（共 ${collected.length}，仅文件名，不含内容）：`);
  for (const rel of collected) lines.push(`  - ${rel}`);
  if (collected.length === 0) lines.push('  （无）');
  lines.push(`[prepare] 符号链接跳过：${tree.links.length}`);
  lines.push(
    `[prepare] 汇总：扫描=${tree.files.length} 收集=${collected.length} ` +
    `排除=${excluded.length} 符号链接跳过=${tree.links.length}`,
  );
  lines.push('[prepare] 结论：OK（输出目录已就绪，仍需经 scan-artifacts.js 扫描后方可上传）');
  process.stdout.write(`${lines.join('\n')}\n`);
  return EXIT_OK;
}

process.exitCode = main(process.argv.slice(2));
