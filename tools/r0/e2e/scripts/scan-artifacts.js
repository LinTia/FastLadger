#!/usr/bin/env node
'use strict';

/**
 * FastLedger R0 E2E —— 上传前安全扫描器（P1 安全整改，失败关闭）。
 *
 * 用途：在 actions/upload-artifact 之前对待上传目录做最后一道闸门。只要存在
 * 黑名单文件名或疑似凭据文本，立即以退出码 2 失败；workflow 的上传 step 通过
 * `steps.artifact_scan.outcome == 'success'` 依赖本 step，因此扫描不通过时绝不上传。
 *
 * 规则：
 *   A. 文件名/扩展名黑名单：*.zip（含 trace 归档）、*.trace、*.network、*.har、
 *      *.db / *.sqlite*、server.log、文件名含 token|secret|credential；符号链接。
 *   B. 文本内容：JWT 三段式、Bearer 值、鉴权头赋值、Cookie / Set-Cookie 赋值、
 *      EBK_JWT_SECRET 赋值。
 *   C. PNG/JPG 等二进制按扩展名或 NUL 字节判定后跳过内容扫描，但 A 类检查照常执行。
 *
 * 输出：仅「文件路径 + 命中类型 + 命中数量」与汇总计数；**绝不打印匹配到的值**。
 * 退出码：0 干净 / 2 发现疑似凭据或黑名单文件（阻断上传）/ 1 参数或环境错误（同样阻断上传）。
 *
 * 用法：
 *   node tools/r0/e2e/scripts/scan-artifacts.js [待扫描目录]
 *   node tools/r0/e2e/scripts/scan-artifacts.js --dir <dir>
 *   node tools/r0/e2e/scripts/scan-artifacts.js --help
 */

const fs = require('node:fs');
const path = require('node:path');

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..', '..');
const DEFAULT_DIR = path.join(REPO_ROOT, 'artifacts', 'r0-e2e-upload');

const EXIT_CLEAN = 0;
const EXIT_ERROR = 1;
const EXIT_BLOCKED = 2;

/** 单文件文本扫描上限；超过即视为环境错误（失败关闭，不静默放过）。 */
const MAX_TEXT_BYTES = 32 * 1024 * 1024;

/** 命中即阻断的扩展名。 */
const BLACKLIST_EXT = new Set([
  '.zip', '.trace', '.network', '.har',
  '.db', '.sqlite', '.sqlite3', '.db-wal', '.db-shm',
]);

/** 跳过内容扫描的二进制扩展名（文件名黑名单仍然生效）。 */
const BINARY_EXT = new Set([
  '.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp', '.ico', '.pdf',
  '.woff', '.woff2', '.ttf', '.eot', '.mp4', '.webm', '.mp3', '.wav',
  '.gz', '.7z', '.exe', '.dll', '.so', '.dylib', '.class', '.jar',
]);

/** 文件名（basename）命中即阻断。 */
const SENSITIVE_NAME_RE = /(?:token|secret|credential)/i;

/** 明确点名的服务原始日志。 */
const SERVER_LOG_NAME = 'server.log';

/**
 * 文本命中规则。每条正则必须带 g 标志；只统计命中数量，不保留匹配值。
 * @type {{type: string, re: RegExp}[]}
 */
const TEXT_RULES = [
  { type: 'JWT_LITERAL', re: /eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}/g },
  { type: 'BEARER_TOKEN', re: /\bBearer\s+[A-Za-z0-9._~+/=-]{12,}/gi },
  { type: 'AUTH_HEADER', re: /["']?authorization["']?\s*[:=]\s*["']?[^\s"',}]{8,}/gi },
  { type: 'COOKIE_HEADER', re: /["']?(?:set-)?cookie["']?\s*[:=]\s*["']?[^\r\n"']{8,}/gi },
  { type: 'EBK_JWT_SECRET_ASSIGN', re: /EBK_JWT_SECRET\s*[:=]\s*["']?[^\s"',}]{4,}/gi },
];

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
    '用法：node tools/r0/e2e/scripts/scan-artifacts.js [待扫描目录|--dir <dir>] [--help]',
    '',
    `默认目录：artifacts/r0-e2e-upload`,
    '退出码：0 干净 / 2 发现疑似凭据或黑名单文件 / 1 参数或环境错误',
    '输出：仅文件路径、命中类型与命中数量，不输出任何匹配值。',
    '',
  ].join('\n'));
}

/**
 * 解析命令行参数。
 * @param {string[]} argv process.argv.slice(2)。
 * @return {{dir: string, help: boolean}} 解析结果。
 * @throws {Error} 参数非法时抛出。
 */
function parseArgs(argv) {
  if (argv.includes('--help') || argv.includes('-h')) {
    return { dir: DEFAULT_DIR, help: true };
  }
  let dir = DEFAULT_DIR;
  let positionalSeen = false;
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--dir') {
      const value = argv[i + 1];
      if (!value || value.startsWith('--')) throw new Error('参数 --dir 缺少取值');
      dir = path.resolve(process.cwd(), value);
      i += 1;
      continue;
    }
    if (arg.startsWith('--')) throw new Error(`未知参数：${arg}`);
    if (positionalSeen) throw new Error('只能指定一个待扫描目录');
    dir = path.resolve(process.cwd(), arg);
    positionalSeen = true;
  }
  return { dir, help: false };
}

/**
 * 统计某条规则在文本中的命中次数，使用独立正则副本避免 lastIndex 污染。
 * @param {string} text 待扫描文本。
 * @param {RegExp} re 带 g 标志的规则。
 * @return {number} 命中次数。
 */
function countMatches(text, re) {
  const copy = new RegExp(re.source, re.flags);
  let count = 0;
  let match;
  while ((match = copy.exec(text)) !== null) {
    count += 1;
    if (copy.lastIndex === match.index) copy.lastIndex += 1;
  }
  return count;
}

/**
 * 检查文件名是否命中黑名单。
 * @param {string} rel 相对路径。
 * @return {string[]} 命中类型列表。
 */
function fileNameFindings(rel) {
  const base = path.basename(rel);
  const lower = base.toLowerCase();
  const ext = path.extname(lower);
  const types = [];

  if (BLACKLIST_EXT.has(ext)) {
    types.push(ext === '.zip' && lower.includes('trace') ? 'TRACE_ARCHIVE' : 'BLACKLIST_EXT');
  }
  if (lower === SERVER_LOG_NAME) types.push('SERVER_LOG');
  if (SENSITIVE_NAME_RE.test(base)) types.push('SENSITIVE_FILENAME');
  return types;
}

/**
 * 通过 NUL 字节判定二进制内容。
 * @param {Buffer} buf 文件内容。
 * @return {boolean} 是否为二进制。
 */
function looksBinary(buf) {
  const limit = Math.min(buf.length, 4096);
  for (let i = 0; i < limit; i += 1) {
    if (buf[i] === 0) return true;
  }
  return false;
}

/**
 * 递归遍历目录；符号链接单独记录，不跟随。
 * @param {string} dir 目录绝对路径。
 * @param {{files: string[], symlinks: string[]}} out 输出累加器。
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
      out.symlinks.push(abs);
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
 * 程序入口。
 * @param {string[]} argv process.argv.slice(2)。
 * @return {number} 退出码。
 */
function main(argv) {
  let options;
  try {
    options = parseArgs(argv);
  } catch (err) {
    process.stderr.write(`[scan][ERROR] ${err.message}\n`);
    return EXIT_ERROR;
  }
  if (options.help) {
    printHelp();
    return EXIT_CLEAN;
  }

  const targetDir = options.dir;
  let dirStat;
  try {
    dirStat = fs.statSync(targetDir);
  } catch (err) {
    process.stderr.write(`[scan][ERROR] 待扫描目录不存在或不可访问：${displayPath(targetDir)}\n`);
    return EXIT_ERROR;
  }
  if (!dirStat.isDirectory()) {
    process.stderr.write(`[scan][ERROR] 待扫描路径不是目录：${displayPath(targetDir)}\n`);
    return EXIT_ERROR;
  }

  /** @type {Map<string, {file: string, type: string, count: number}>} */
  const findings = new Map();
  /**
   * 累加一条命中，只记录文件名、类型与数量。
   * @param {string} file 相对文件路径。
   * @param {string} type 命中类型。
   * @param {number} count 命中数量。
   */
  function addFinding(file, type, count) {
    const key = `${file}|${type}`;
    const prev = findings.get(key);
    if (prev) prev.count += count;
    else findings.set(key, { file, type, count });
  }

  const tree = { files: [], symlinks: [] };
  let textScanned = 0;
  let binarySkipped = 0;

  try {
    walk(targetDir, tree);
    for (const abs of tree.files) {
      const rel = path.relative(targetDir, abs).split(path.sep).join('/');
      for (const type of fileNameFindings(rel)) addFinding(rel, type, 1);

      const ext = path.extname(abs).toLowerCase();
      if (BINARY_EXT.has(ext)) {
        binarySkipped += 1;
        continue;
      }
      const stat = fs.statSync(abs);
      if (stat.size > MAX_TEXT_BYTES) {
        throw new Error(`文件超过可扫描上限 ${MAX_TEXT_BYTES} 字节：${rel}`);
      }
      const buf = fs.readFileSync(abs);
      if (looksBinary(buf)) {
        binarySkipped += 1;
        continue;
      }
      const text = buf.toString('utf8');
      textScanned += 1;
      for (const rule of TEXT_RULES) {
        const count = countMatches(text, rule.re);
        if (count > 0) addFinding(rel, rule.type, count);
      }
    }
    for (const link of tree.symlinks) {
      const rel = path.relative(targetDir, link).split(path.sep).join('/');
      addFinding(rel, 'SYMLINK_ENTRY', 1);
    }
  } catch (err) {
    process.stderr.write(`[scan][ERROR] 扫描过程异常：${err.message}\n`);
    return EXIT_ERROR;
  }

  const rows = [...findings.values()].sort(
    (a, b) => a.file.localeCompare(b.file) || a.type.localeCompare(b.type),
  );
  const totalHits = rows.reduce((sum, row) => sum + row.count, 0);

  const lines = [
    `[scan] 待上传目录：${displayPath(targetDir)}`,
    `[scan] 文件总数=${tree.files.length} 文本已扫描=${textScanned} ` +
      `二进制跳过内容扫描=${binarySkipped} 符号链接=${tree.symlinks.length}`,
    `[scan] 命中明细（仅文件路径 + 命中类型 + 命中数量，不输出匹配值）：`,
  ];
  for (const row of rows) lines.push(`[HIT] ${row.file} | ${row.type} | ${row.count}`);
  if (rows.length === 0) lines.push('[HIT] （无） | — | 0');
  lines.push(`[scan] 命中文件/类型组合=${rows.length} 命中总数=${totalHits}`);

  if (totalHits > 0) {
    lines.push('[scan] 结论：BLOCKED —— 禁止上传，请先清除上述命中项');
    process.stdout.write(`${lines.join('\n')}\n`);
    return EXIT_BLOCKED;
  }
  if (tree.files.length === 0) {
    lines.push('[scan][WARN] 目录为空；上传 step 的 if-no-files-found=error 仍会使 job 失败');
  }
  lines.push('[scan] 结论：CLEAN —— 允许上传');
  process.stdout.write(`${lines.join('\n')}\n`);
  return EXIT_CLEAN;
}

process.exitCode = main(process.argv.slice(2));
