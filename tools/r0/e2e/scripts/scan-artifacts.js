#!/usr/bin/env node
'use strict';

/**
 * FastLedger R0 E2E —— 上传前安全扫描器（P1 安全整改 / Round 3，覆盖 K1–K3，失败关闭）。
 *
 * 用途：在 actions/upload-artifact 之前对待上传目录做最后一道闸门。只要存在
 * 黑名单文件名或疑似凭据文本，立即以退出码 2 失败；workflow 的上传 step 通过
 * steps.artifact_scan.outcome == 'success' 依赖本 step，因此扫描不通过时绝不上传。
 *
 * 本轮（K1–K3 修复 + 第八次复核 RV-07/09/10 整改）相对 Round 2 的关键扩展：
 *   K1 编码绕过：对每个文件尝试严格 UTF-8（fatal）/ UTF-16LE / UTF-16BE 三种解码（含 BOM 识别）；
 *     严格 UTF-8 解码失败且文件无非 UTF-16 BOM 时，文本型文件直接判定为「无法可靠解码 → BLOCK」，
 *     绝不降级为 CLEAN（RV-09：少量非法字节打断关键词也不能借宽松阈值蒙混过关）；
 *     仅当存在明确 UTF-16 BOM 时才按 UTF-16 解码，避免把 UTF-8 文件误当 UTF-16 放行。
 *     无法可靠解码的文本型文件一律 BLOCK。
 *   K2 短值 / 低熵：新增「强敏感键名 + 值」联合判定——凡命中 token / secret / api_key /
 *     password 等强敏感键名，且值非空非明显占位（占位白名单抑制误报、不靠值长度），即判定疑似凭据。
 *     普通业务字段（unit / note / status 等）不属于强敏感键名，不会被无理由误报。
 *   K3 字段名 / Base64：敏感键名规则覆盖 x-api-key / apiKey / api_key / API_KEY 等大小写
 *     与分隔符变体（含 JSON 嵌套）；新增 Base64 解码复扫，覆盖标准 Base64 与 Base64URL、
 *     带 / 无 padding 两种形态，识别 Base64 编码后的凭据。
 *   RV-07 明文 PEM 私钥：明文文本直接识别 PRIVATE KEY / RSA / EC / OPENSSH / ENCRYPTED PRIVATE
 *     KEY 标记，命中一律 BLOCK，绝不输出密钥内容。
 *
 * 输出：仅「文件路径 + 命中类型 + 命中数量 + 编码解码统计」，绝不打印匹配到的值。
 * 退出码：0 干净 / 2 发现疑似凭据、黑名单文件或无法可靠解码（阻断上传）/ 1 参数或环境错误。
 */

const fs = require('node:fs');
const path = require('node:path');
const { TextDecoder } = require('node:util');

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..', '..');
const DEFAULT_DIR = path.join(REPO_ROOT, 'artifacts', 'r0-e2e-upload');

const EXIT_CLEAN = 0;
const EXIT_ERROR = 1;
const EXIT_BLOCKED = 2;

/** 单文件扫描上限；超过即视为环境错误（失败关闭，不静默放过）。 */
const MAX_TEXT_BYTES = 32 * 1024 * 1024;

/** 文本型文件低于此字节数时不触发「无法可靠解码 → BLOCK」，避免误伤极小文件。 */
const MIN_UNDECODABLE_BYTES = 16;

/** 命中即阻断的扩展名。 */
const BLACKLIST_EXT = new Set([
  '.zip', '.trace', '.network', '.har',
  '.db', '.sqlite', '.sqlite3', '.db-wal', '.db-shm',
]);

/** 二进制扩展名：只用于附加魔数校验，绝不再用于跳过内容扫描。 */
const BINARY_EXT = new Set([
  '.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp', '.ico', '.pdf',
  '.woff', '.woff2', '.ttf', '.eot', '.mp4', '.webm', '.mp3', '.wav',
  '.gz', '.7z', '.exe', '.dll', '.so', '.dylib', '.class', '.jar',
]);

/**
 * 已知二进制起始魔数（用于「是否为二进制文件」判定，与扩展名无关）。
 * 命中即视为已知二进制，不进入「无法可靠解码 → BLOCK」分支。
 */
const KNOWN_BINARY_SIGNATURES = [
  Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]), // PNG
  Buffer.from([0xff, 0xd8, 0xff]),                                 // JPEG
  Buffer.from('GIF8', 'latin1'),                                   // GIF
  Buffer.from('%PDF', 'latin1'),                                   // PDF
  Buffer.from('BM', 'latin1'),                                     // BMP
  Buffer.from([0x00, 0x00, 0x01, 0x00]),                           // ICO
  Buffer.from([0x1f, 0x8b]),                                       // GZIP
  Buffer.from('PK\u0003\u0004', 'latin1'),                         // ZIP
];

/** 二进制扩展名对应的起始魔数（附加防线，格式不符记 SUSPICIOUS_BINARY）。 */
const BINARY_SIGNATURES = {
  '.png': [Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])],
  '.jpg': [Buffer.from([0xff, 0xd8, 0xff])],
  '.jpeg': [Buffer.from([0xff, 0xd8, 0xff])],
  '.gif': [Buffer.from('GIF8', 'latin1')],
  '.pdf': [Buffer.from('%PDF', 'latin1')],
  '.bmp': [Buffer.from('BM', 'latin1')],
  '.ico': [Buffer.from([0x00, 0x00, 0x01, 0x00])],
  '.gz': [Buffer.from([0x1f, 0x8b])],
};

const SENSITIVE_NAME_RE = /(?:token|secret|credential)/i;
const SERVER_LOG_NAME = 'server.log';

/** 强敏感键名（K2/K3 联合判定）：命中即按「键值联合」判定，值非空非占位即疑似凭据。 */
const SENSITIVE_KEY_SOURCE = [
  'api[-_]?key', 'apikey', 'x[-_]?api[-_]?key',
  'access[-_]?token', 'refresh[-_]?token', 'auth[-_]?token',
  'token', 'secret', 'client[-_]?secret',
  'password', 'passwd', 'pwd',
  'private[-_]?key', 'session', 'sessionid', 'credential',
].join('|');

/** 明显占位 / 非凭据值，用于降低 K2 误报。 */
const BENIGN_PLACEHOLDER = new Set([
  'none', 'null', 'undefined', 'true', 'false', 'redacted', 'hidden',
  'masked', 'n/a', 'na', 'set', 'enabled', 'disabled', 'anonymous',
  'public', 'empty', 'default', 'missing', 'unknown', 'ok', 'off', 'on',
]);

/** 解码后文本里用于判定 Base64 是否藏有凭据的标记（K3）。 */
const DECODED_CRED_MARKER = new RegExp(
  ['eyJ', 'Bearer', 'Authorization', 'api[-_]?key', 'apiKey',
    'password', 'passwd', 'secret', 'token', 'credential',
    'private[-_]?key', 'BEGIN RSA', 'BEGIN PRIVATE'].join('|'),
  'i',
);

/**
 * 明文 PEM 私钥 / 证书私钥标记（RV-07）。
 * 覆盖 PRIVATE KEY / RSA PRIVATE KEY / EC PRIVATE KEY / OPENSSH PRIVATE KEY /
 * ENCRYPTED PRIVATE KEY（及 DSA 等）。命中一律 BLOCK，绝不输出密钥内容。
 */
const PEM_PRIVATE_KEY_RE = /-----BEGIN (?:RSA |EC |OPENSSH |ENCRYPTED |DSA |[A-Z0-9]* )?PRIVATE KEY-----/g;

/**
 * 判定头值是否具备凭据形态，用于收紧 AUTH_HEADER / COOKIE_HEADER，消除假阳性。
 * 纯字母 + 连字符的自然语言短语（如 role-based-access）不算命中。
 * 只返回布尔，绝不把值写入任何输出。
 */
function credentialLikeValue(raw) {
  const value = String(raw).replace(/^["'`]+|["'`]+$/g, '').trim();
  if (/^eyJ/.test(value) || /^Bearer\b/i.test(value)) return true;
  if (value.length < 16) return false;
  if (/[.=_\+/]/.test(value)) return true;
  if (!/^[A-Za-z0-9+/=_-]+$/.test(value)) return false;
  const hasDigit = /[0-9]/.test(value);
  const hasUpper = /[A-Z]/.test(value);
  const hasLower = /[a-z]/.test(value);
  return hasDigit || (hasUpper && hasLower);
}

/**
 * 强敏感键名 + 值的联合判定（K2/K3）：键名命中 SENSITIVE_KEY_SOURCE，值非空非占位即疑似凭据。
 * 绝不把值写入任何输出。
 */
function sensitiveAssignValue(key, raw) {
  const value = String(raw == null ? '' : raw).trim();
  // K2：短值 / 低熵凭据也必须阻断——只要值非空且非明显占位即疑似凭据。
  // 占位白名单（BENIGN_PLACEHOLDER）才是抑制误报的主防线，而非「值长度 >= 4」。
  if (value.length < 1) return false;
  if (BENIGN_PLACEHOLDER.has(value.toLowerCase())) return false;
  return true;
}

/**
 * 判定 Base64 字符串解码后是否藏有凭据形态（K3 / RV-10）。
 * 同时覆盖标准 Base64 与 Base64URL（URL-safe 使用 - 与 _），并兼容有 / 无 padding。
 * 仅当解码文本命中凭据标记时才判定为凭据，避免把任意 base64（图片/哈希）误报。
 */
function isBase64Credential(b64) {
  // Base64URL 的 - 与 _ 归一化为标准 + 与 /；无 padding 时补足到 4 的倍数。
  let s = b64.replace(/-/g, '+').replace(/_/g, '/');
  while (s.length % 4 !== 0) s += '=';
  let bytes;
  try {
    bytes = Buffer.from(s, 'base64');
  } catch (err) {
    return false;
  }
  if (bytes.length < 8) return false;
  const text1 = bytes.toString('utf8');
  const text2 = bytes.toString('latin1');
  if (DECODED_CRED_MARKER.test(text1) || DECODED_CRED_MARKER.test(text2)) return true;
  return false;
}

/**
 * 统计某条「纯数量」正则（无 group / 无 validator）在文本中的命中次数。
 */
function countOf(text, re) {
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
 * 严格解码 UTF-8（fatal：任何非法字节即失败，不做「少量替换字符继续 CLEAN」的宽松处理）。
 * 严格解码失败意味着文件不是可靠的 UTF-8 文本——安全扫描必须进入「无法可靠解码 → BLOCK」
 * 分支，而绝不能忽略错误继续按 CLEAN 放行（RV-09）。
 */
function decodeUtf8(buf) {
  try {
    const text = new TextDecoder('utf-8', { fatal: true }).decode(buf);
    return { text: text.replace(/^\uFEFF/, ''), reliable: true, ratio: 0 };
  } catch (err) {
    return { text: '', reliable: false, ratio: 1 };
  }
}

/**
 * 解码 UTF-16（LE / BE）并评估可靠性：要求字节数为偶数、U+FFFD 占比 < 2%，
 * 且非 NUL 字符中可打印 ASCII 占比 > 85%（真实 UTF-16 拉丁文本满足）。
 */
function decodeUtf16(buf, le) {
  if (buf.length % 2 !== 0) return { text: '', reliable: false, ratio: 1 };
  let text;
  try {
    // Node Buffer 仅支持 'utf16le'，UTF-16BE 必须用 TextDecoder 显式解码。
    const decoder = new TextDecoder(le ? 'utf-16le' : 'utf-16be');
    text = decoder.decode(buf).replace(/^\uFEFF/, '');
  } catch (err) {
    return { text: '', reliable: false, ratio: 1 };
  }
  let fffd = 0;
  let printable = 0;
  let nonNul = 0;
  for (const ch of text) {
    if (ch === '\uFFFD') {
      fffd += 1;
      continue;
    }
    if (ch === '\u0000') continue;
    nonNul += 1;
    const c = ch.codePointAt(0);
    if ((c >= 0x20 && c <= 0x7e) || (c >= 0xa0 && c <= 0xff) || ch === '\n' || ch === '\t' || ch === '\r') {
      printable += 1;
    }
  }
  const ratio = text.length ? fffd / text.length : 1;
  const reliable = ratio < 0.02 && (nonNul === 0 || printable / nonNul > 0.85);
  return { text, reliable, ratio };
}

/** 判断缓冲区是否以已知二进制魔数开头。 */
function isKnownBinary(buf) {
  return KNOWN_BINARY_SIGNATURES.some(
    (sig) => buf.length >= sig.length && buf.subarray(0, sig.length).equals(sig),
  );
}

/**
 * 对一段已解码文本施加全部文本规则，返回 {类型: 数量}（只计数，不保留值）。
 * @param {string} text 已解码文本。
 * @return {Map<string, number>} 命中计数。
 */
function scanText(text) {
  const out = new Map();
  const add = (type, n) => {
    if (n > 0) out.set(type, (out.get(type) || 0) + n);
  };

  // JWT 三段式
  add('JWT_LITERAL', countOf(text, /eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}/g));
  // Bearer 值
  add('BEARER_TOKEN', countOf(text, /\bBearer\s+[A-Za-z0-9._~+/=-]{12,}/gi));
  // 鉴权头（值形态判定，保留 case-12 业务字段假阳性约束）
  for (const m of text.matchAll(/["']?authorization["']?\s*[:=]\s*["']?([^\s"',}]{8,})/gi)) {
    if (credentialLikeValue(m[1])) add('AUTH_HEADER', 1);
  }
  // Cookie / Set-Cookie（值形态判定）
  for (const m of text.matchAll(/["']?(?:set-)?cookie["']?\s*[:=]\s*["']?([^\r\n"']{8,})/gi)) {
    if (credentialLikeValue(m[1])) add('COOKIE_HEADER', 1);
  }
  // 强敏感键名 + 值（K2/K3 联合判定；覆盖 JSON 嵌套与大小写 / 分隔符变体）。
  // 键名两侧允许可选引号（"token" / 'token'），值两侧同样允许可选引号，
  // 以覆盖标准 JSON / 带引号配置中嵌套凭据。
  const assignRe = new RegExp(
    '(^|[\\s"\',{])["\']?(' + SENSITIVE_KEY_SOURCE + ')["\']?\\s*[:=]\\s*["\']?([^"\',"}\\s]{1,256})',
    'gi',
  );
  for (const m of text.matchAll(assignRe)) {
    if (sensitiveAssignValue(m[2], m[3])) add('SENSITIVE_ASSIGN', 1);
  }
  // EBK_JWT_SECRET 赋值
  add('EBK_JWT_SECRET_ASSIGN', countOf(text, /EBK_JWT_SECRET\s*[:=]\s*["']?[^\s"',}]{4,}/gi));
  // PEM 明文私钥 / 证书私钥标记（RV-07）：命中一律 BLOCK，绝不输出密钥内容。
  add('PEM_PRIVATE_KEY', countOf(text, PEM_PRIVATE_KEY_RE));
  // Base64 / Base64URL 编码凭据（K3 / RV-10）：字符集覆盖标准 + 与 URL-safe - _，
  // 带 / 无 padding 均可；解码后命中凭据标记才判定。
  for (const m of text.matchAll(/[A-Za-z0-9+/_-]{16,}={0,2}/g)) {
    if (isBase64Credential(m[0])) add('BASE64_CREDENTIAL', 1);
  }
  return out;
}

module.exports = {
  scanText,
  decodeUtf8,
  decodeUtf16,
  isKnownBinary,
  isBase64Credential,
  credentialLikeValue,
  sensitiveAssignValue,
  SENSITIVE_KEY_SOURCE,
  KNOWN_BINARY_SIGNATURES,
  EXIT_CLEAN,
  EXIT_ERROR,
  EXIT_BLOCKED,
};
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
 * 检查二进制文件起始魔数是否匹配已知格式。
 */
function binarySignatureMismatch(ext, buf) {
  const signatures = BINARY_SIGNATURES[ext];
  if (!signatures) return false;
  return !signatures.some(
    (sig) => buf.length >= sig.length && buf.subarray(0, sig.length).equals(sig),
  );
}

/**
 * 递归遍历目录；符号链接单独记录，不跟随。
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
 * 把绝对路径转换为可安全打印的仓库相对路径，不泄露个人绝对路径。
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
    '默认目录：artifacts/r0-e2e-upload',
    '退出码：0 干净 / 2 发现疑似凭据、黑名单文件或无法可靠解码 / 1 参数或环境错误',
    '扫描口径：全部文件以 latin1 逐字节映射后，再对 UTF-8/UTF-16LE/UTF-16BE 可靠解码结果',
    '          施加 5+2 条文本规则（含强敏感键名联合判定与 Base64 解码复扫）；',
    '          文本型文件凡无法可靠解码即 BLOCK，绝不降级为 CLEAN。',
    '输出：仅文件路径、命中类型与命中数量，不输出任何匹配值。',
    '',
  ].join('\n'));
}

/**
 * 解析命令行参数。
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
 * 程序入口。
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

  const findings = new Map();
  function addFinding(file, type, count) {
    const key = `${file}|${type}`;
    const prev = findings.get(key);
    if (prev) prev.count += count;
    else findings.set(key, { file, type, count });
  }

  const tree = { files: [], symlinks: [] };
  let contentScanned = 0;
  let binaryAlsoScanned = 0;
  let utf8Reliable = 0;
  let utf16Reliable = 0;
  let undecodableFiles = 0;
  let suspiciousBinary = 0;

  try {
    walk(targetDir, tree);
    for (const abs of tree.files) {
      const rel = path.relative(targetDir, abs).split(path.sep).join('/');
      for (const type of fileNameFindings(rel)) addFinding(rel, type, 1);

      const ext = path.extname(abs).toLowerCase();
      const stat = fs.statSync(abs);
      if (stat.size > MAX_TEXT_BYTES) {
        throw new Error(`文件超过可扫描上限 ${MAX_TEXT_BYTES} 字节：${rel}`);
      }
      const buf = fs.readFileSync(abs);

      const knownBinary = isKnownBinary(buf);
      // 仅当文件起始存在明确 UTF-16 BOM 时，才把 UTF-16 解码结果视为「可靠文本」。
      // 没有 BOM 却「能解出可读 UTF-16」的，多为 UTF-8 损坏后被错误当成 UTF-16（RV-09 反例），
      // 必须按「无法可靠解码」处理，绝不能放行为 CLEAN。
      const hasUtf16Bom = buf.length >= 2 &&
        ((buf[0] === 0xFF && buf[1] === 0xFE) || (buf[0] === 0xFE && buf[1] === 0xFF));
      if (BINARY_EXT.has(ext) && binarySignatureMismatch(ext, buf)) {
        addFinding(rel, 'SUSPICIOUS_BINARY', 1);
        suspiciousBinary += 1;
      }

      // 候选解码：latin1 始终扫描（覆盖纯 ASCII 内嵌与二进制内文本）；
      // UTF-8/UTF-16LE/UTF-16BE 仅采纳「可靠」解码结果，避免乱码误报 / 漏报。
      const candidates = [];
      candidates.push({ label: 'latin1', text: buf.toString('latin1') });
      const u8 = decodeUtf8(buf);
      if (u8.reliable) {
        candidates.push({ label: 'utf8', text: u8.text });
        utf8Reliable += 1;
      }
      const u16le = decodeUtf16(buf, true);
      const u16be = decodeUtf16(buf, false);
      if (u16le.reliable) {
        candidates.push({ label: 'utf16le', text: u16le.text });
        utf16Reliable += 1;
      }
      if (u16be.reliable) {
        candidates.push({ label: 'utf16be', text: u16be.text });
        utf16Reliable += 1;
      }

      for (const c of candidates) {
        const res = scanText(c.text);
        for (const [type, n] of res) addFinding(rel, type, n);
      }

      // K1-03 / RV-09 / RV-11：噪声字节复扫。把 latin1 视图里所有「非可打印 ASCII
      // 与制表/换行/回车」的字节（NUL、控制符、DEL、高位噪声字节）一并剔除后再扫一次，
      // 以捕获被插入噪声打断的关键词 / token 值（含 NUL 位于 token 值内部、关键词内部等形态）。
      // 仅对「非已知二进制」施行，避免压缩二进制被挤压缩连产生偶发字符串造成误 BLOCK
      // （已知二进制已有魔数 + 内容双校验）。此复扫不扩大关键词正则，仅做噪声归一化。
      if (!knownBinary) {
        const squeezed = buf.toString('latin1').replace(/[^\x20-\x7e\t\r\n]/g, '');
        const squeezedRes = scanText(squeezed);
        for (const [type, n] of squeezedRes) addFinding(rel, type, n);
      }

      contentScanned += 1;
      if (BINARY_EXT.has(ext)) binaryAlsoScanned += 1;

      // K1-04：文本型文件（非已知二进制）却无法可靠解码 → 阻断，绝不 CLEAN。
      // 严格 UTF-8 失败后，仅当存在明确 UTF-16 BOM 时才认可 UTF-16 为「可靠文本」
      // （RV-09：否则 UTF-8 损坏文件可能被错误当成 UTF-16 而漏报）。
      const reliableText = u8.reliable || (hasUtf16Bom && (u16le.reliable || u16be.reliable));
      if (!knownBinary && buf.length >= MIN_UNDECODABLE_BYTES && !reliableText) {
        addFinding(rel, 'UNDECODABLE', 1);
        undecodableFiles += 1;
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
    `[scan] 文件总数=${tree.files.length} 内容已全量扫描=${contentScanned} ` +
      `其中二进制文件=${binaryAlsoScanned} 跳过内容扫描=0 符号链接=${tree.symlinks.length}`,
    `[scan] 编码解码：utf8可靠=${utf8Reliable} utf16可靠=${utf16Reliable} ` +
      `无法可靠解码(已阻断)=${undecodableFiles} 可疑二进制(魔数不符)=${suspiciousBinary}`,
    '[scan] 扫描口径：latin1 全量 + UTF-8/UTF-16LE/UTF-16BE 可靠解码复扫 + ' +
      '强敏感键名联合判定 + Base64 解码复扫；任何无法可靠解码的文本型文件一律 BLOCK',
    '[scan] 命中明细（仅文件路径 + 命中类型 + 命中数量，不输出匹配值）：',
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

