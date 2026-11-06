'use strict';

/**
 * FastLedger R0 E2E —— 最终上传目录的「单一权威根清单」生成与递归字节校验（RV-13）。
 *
 * 设计口径（相对第八轮按 basename 全局排除清单的做法，本轮彻底重设计）：
 *
 *  1. 唯一权威清单 = 目标目录根下的 `SHA256SUMS.txt`（根清单）。
 *     它递归覆盖最终上传目录中的「每一个实际文件」（含任意层级的子目录文件）。
 *  2. 源目录中任何层级的 `SHA256SUMS.txt` 都是「上一轮生成的派生产物」，
 *     不是证据本体：prepare-upload.js 在白名单收敛阶段就把它们全部排除，
 *     绝不原样复制进目标目录 —— 从根上杜绝「指向已被过滤文件的失效引用」。
 *     替代校验方式 = 根清单对全部实际文件（含原嵌套清单所描述的文件）逐一给出 SHA256。
 *  3. 职责清晰、无重复、无循环：只有一份清单；嵌套清单无任何职责；
 *     根清单不列出自身（不能哈希自身），是唯一例外，在输出中显式声明。
 *  4. 校验是「递归实际字节校验」：重新遍历目标目录、重新读取每个文件字节、
 *     重新计算 SHA256，与清单逐条比对；不是只校验清单格式或记录条数。
 *     任何 missing / mismatch / uncovered / staleRef / nestedManifest / selfRef
 *     / malformed / symlink 都会导致校验失败（失败关闭）。
 *
 * 本模块被 prepare-upload.js、verify-upload-manifest.js、test-upload-manifest.js 共用，
 * 保证「生成」与「独立校验」使用完全一致的口径。
 */

const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');

/** 唯一权威清单文件名（大小写不敏感判定）。 */
const MANIFEST_NAME = 'SHA256SUMS.txt';

/** 与 prepare-upload.js 一致的「被排除载体」判定，用于识别失效引用。 */
const EXCLUDED_EXT_RE = /\.(?:zip|trace|network|har|db|sqlite|sqlite3|db-wal|db-shm)$/i;
const SENSITIVE_NAME_RE = /(?:token|secret|credential)/i;

/**
 * 相对路径是否名为清单（任意层级）。
 * @param {string} rel POSIX 相对路径。
 * @return {boolean}
 */
function isManifestName(rel) {
  return path.posix.basename(rel).toLowerCase() === MANIFEST_NAME.toLowerCase();
}

/**
 * 清单条目是否指向「本应被白名单排除」的载体（失效引用）。
 * @param {string} rel POSIX 相对路径。
 * @return {boolean}
 */
function isExcludedRef(rel) {
  const base = path.posix.basename(rel);
  if (EXCLUDED_EXT_RE.test(base)) return true;
  if (base.toLowerCase() === 'server.log') return true;
  if (SENSITIVE_NAME_RE.test(base)) return true;
  if (isManifestName(rel)) return true;
  return false;
}

/**
 * 计算文件实际字节的 SHA256。
 * @param {string} abs 绝对路径。
 * @return {string} 64 位十六进制摘要。
 */
function sha256File(abs) {
  return crypto.createHash('sha256').update(fs.readFileSync(abs)).digest('hex');
}

/**
 * 递归遍历目录，返回全部普通文件与符号链接（POSIX 相对路径，已排序）。
 * 不跟随符号链接。
 * @param {string} root 目录绝对路径。
 * @return {{files: string[], symlinks: string[]}}
 */
function walkFiles(root) {
  const files = [];
  const symlinks = [];
  function rec(dir, prefix) {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
      const rel = prefix ? `${prefix}/${entry.name}` : entry.name;
      const abs = path.join(dir, entry.name);
      if (entry.isSymbolicLink()) {
        symlinks.push(rel);
      } else if (entry.isDirectory()) {
        rec(abs, rel);
      } else if (entry.isFile()) {
        files.push(rel);
      }
    }
  }
  rec(root, '');
  files.sort();
  symlinks.sort();
  return { files, symlinks };
}

/**
 * 把 POSIX 相对路径安全地拼到目标目录下（禁止越界）。
 * @param {string} dest 目标目录。
 * @param {string} rel POSIX 相对路径。
 * @return {string} 绝对路径。
 */
function resolveUnder(dest, rel) {
  const abs = path.join(dest, ...rel.split('/'));
  if (path.relative(dest, abs).startsWith('..')) throw new Error(`清单条目越界：${rel}`);
  return abs;
}

/**
 * 生成唯一权威根清单：覆盖目标目录下每一个实际文件（根清单自身除外）。
 * 若目标目录里仍残留嵌套清单，直接抛错（失败关闭），不静默吞掉。
 * @param {string} dest 最终上传目录。
 * @return {{targets: string[], lines: string[], manifestRel: string}}
 */
function buildRootManifest(dest) {
  const { files } = walkFiles(dest);
  const nested = files.filter((rel) => rel !== MANIFEST_NAME && isManifestName(rel));
  if (nested.length) {
    throw new Error(`目标目录残留嵌套清单（应由根清单统一重建）：${nested.join(', ')}`);
  }
  const targets = files.filter((rel) => rel !== MANIFEST_NAME);
  const lines = targets.map((rel) => `${sha256File(resolveUnder(dest, rel))}  ${rel}`);
  fs.writeFileSync(path.join(dest, MANIFEST_NAME), lines.length ? `${lines.join('\n')}\n` : '', 'utf8');
  return { targets, lines, manifestRel: MANIFEST_NAME };
}

/**
 * 对最终目录递归执行「实际字节校验」。
 *
 * @param {string} dest 最终上传目录。
 * @return {{
 *   ok: boolean, manifestExists: boolean, entries: number, files: number,
 *   verified: number, missing: string[], mismatch: string[], uncovered: string[],
 *   staleRefs: string[], nestedManifests: string[], malformed: string[],
 *   selfRef: boolean, symlinks: string[], manifestRel: string
 * }}
 */
function verifyUploadTree(dest) {
  const result = {
    ok: false,
    manifestExists: false,
    entries: 0,
    files: 0,
    verified: 0,
    missing: [],
    mismatch: [],
    uncovered: [],
    staleRefs: [],
    nestedManifests: [],
    malformed: [],
    selfRef: false,
    symlinks: [],
    manifestRel: MANIFEST_NAME,
  };

  const manifestAbs = path.join(dest, MANIFEST_NAME);
  result.manifestExists = fs.existsSync(manifestAbs);
  if (!result.manifestExists) return result;

  const entryMap = new Map();
  const rawLines = fs.readFileSync(manifestAbs, 'utf8').split('\n').filter((l) => l.length > 0);
  for (const line of rawLines) {
    const m = line.match(/^([0-9a-f]{64})  (.+)$/);
    if (!m) {
      result.malformed.push(line);
      continue;
    }
    const [, sum, rel] = m;
    if (entryMap.has(rel)) result.malformed.push(`重复条目 ${rel}`);
    entryMap.set(rel, sum);
    if (rel === MANIFEST_NAME) result.selfRef = true;
    else if (isManifestName(rel)) result.nestedManifests.push(rel);
    if (isExcludedRef(rel)) result.staleRefs.push(rel);
  }
  result.entries = entryMap.size;

  // 逐条比对清单 → 实际字节（重新读取文件、重新计算摘要）。
  for (const [rel, declared] of entryMap) {
    const abs = resolveUnder(dest, rel);
    if (!fs.existsSync(abs) || !fs.statSync(abs).isFile()) {
      result.missing.push(rel);
      continue;
    }
    const actual = sha256File(abs);
    if (actual !== declared) result.mismatch.push(rel);
    else result.verified += 1;
  }

  // 递归遍历目录 → 每个实际文件都必须被清单覆盖（根清单自身是唯一例外）。
  const tree = walkFiles(dest);
  result.symlinks = tree.symlinks;
  const covered = tree.files.filter((rel) => rel !== MANIFEST_NAME);
  result.files = covered.length;
  for (const rel of covered) {
    if (!entryMap.has(rel)) result.uncovered.push(rel);
    if (isManifestName(rel)) result.nestedManifests.push(rel);
  }

  result.ok = result.manifestExists
    && result.missing.length === 0
    && result.mismatch.length === 0
    && result.uncovered.length === 0
    && result.staleRefs.length === 0
    && result.nestedManifests.length === 0
    && result.malformed.length === 0
    && !result.selfRef
    && result.symlinks.length === 0;
  return result;
}

/**
 * 生成供日志/证据使用的多行人类可读结果（不含任何文件内容）。
 * @param {ReturnType<typeof verifyUploadTree>} r 校验结果。
 * @return {string[]} 行数组。
 */
function formatVerification(r) {
  return [
    `根清单：${r.manifestRel}（唯一权威清单，递归覆盖全目录；根清单自身为唯一例外，不哈希自身）`,
    `清单条目数=${r.entries} 实际文件数=${r.files} 实际字节校验通过=${r.verified}`,
    `missing=${r.missing.length} mismatch=${r.mismatch.length} uncovered=${r.uncovered.length}`,
    `staleRefs=${r.staleRefs.length} nestedManifests=${r.nestedManifests.length} ` +
      `malformed=${r.malformed.length} selfRef=${r.selfRef} symlinks=${r.symlinks.length}`,
    ...r.missing.map((x) => `  [MISSING] ${x}`),
    ...r.mismatch.map((x) => `  [MISMATCH] ${x}`),
    ...r.uncovered.map((x) => `  [UNCOVERED] ${x}`),
    ...r.staleRefs.map((x) => `  [STALE_REF] ${x}`),
    ...r.nestedManifests.map((x) => `  [NESTED_MANIFEST] ${x}`),
    ...r.malformed.map((x) => `  [MALFORMED] ${x}`),
    ...r.symlinks.map((x) => `  [SYMLINK] ${x}`),
    `结论：${r.ok ? 'OK（0 missing / 0 mismatch / 全部文件可验证）' : 'FAILED（禁止上传）'}`,
  ];
}

module.exports = {
  MANIFEST_NAME,
  isManifestName,
  isExcludedRef,
  sha256File,
  walkFiles,
  resolveUnder,
  buildRootManifest,
  verifyUploadTree,
  formatVerification,
};
