'use strict';

/**
 * FastLedger R0 全仓凭证安全扫描器（RV-05，无第三方依赖）。
 *
 * 用法：
 *   node tools/r0/scan-credentials.js                     # Markdown 报告输出到 stdout
 *   node tools/r0/scan-credentials.js --json              # JSON 报告
 *   node tools/r0/scan-credentials.js --strict            # 发现真实凭证候选时退出码 2
 *   node tools/r0/scan-credentials.js --out <path>        # 同时写入报告文件
 *
 * 扫描：Authorization / Bearer / JWT 三段式 / Cookie / Set-Cookie / 常见密码与密钥字段、
 *       *.env 与 token|secret|credential 文件名、个人绝对路径。
 * 输出：仅文件路径、命中类型、数量、脱敏上下文与判定；**绝不输出敏感值本身**。
 * 退出码：0 扫描完成 / 1 参数错误 / 2 --strict 下发现真实凭证候选。
 */

const fs = require('node:fs');
const path = require('node:path');

const REPO_ROOT = path.resolve(__dirname, '..', '..');
const SKIP_DIRS = new Set(['.git', 'node_modules', '.dart_tool', '.tools', 'build', '.idea', '.vscode']);
const BINARY_EXT = new Set([
  '.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp', '.ico', '.pdf', '.zip', '.gz', '.7z',
  '.exe', '.dll', '.so', '.dylib', '.db', '.sqlite', '.sqlite3', '.woff', '.woff2', '.ttf',
  '.dill', '.a', '.o', '.class', '.jar', '.apk', '.pem', '.p12', '.der',
]);
const MAX_BYTES = 8 * 1024 * 1024;

// 明显的合成测试样本 / 占位符 / 环境变量引用，不算真实凭证。
const SYNTHETIC = /FAKE|DUMMY|EXAMPLE|SYNTHETIC|PLACEHOLDER|REDACTED|<[^>]+>|\$\{|\$[A-Z_]{3,}|process\.env|os\.environ|ENV\[|your[-_ ]?(token|password|secret)/i;

// cls=credential：自动可判定的凭证字面量，参与 --strict 判定。
// cls=review：代码/文档中的赋值表达式，无法自动区分示例与真实值，全部列出供人工复核，
//             但不作为 strict 自动失败依据（避免把变量名、命令替换误报为泄露）。
const PATTERNS = [
  { type: 'JWT_LITERAL', re: /eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}/g, group: 0, cls: 'credential' },
  { type: 'BEARER_TOKEN', re: /\bBearer\s+([A-Za-z0-9._~+/=-]{12,})/gi, group: 1, cls: 'credential' },
  { type: 'AUTH_HEADER', re: /(["']?authorization["']?\s*[:=]\s*["']?)([^\s"',}]{8,})/gi, group: 2, cls: 'credential' },
  { type: 'COOKIE_HEADER', re: /(["']?(?:set-)?cookie["']?\s*[:=]\s*["']?)([^\r\n"']{8,})/gi, group: 2, cls: 'credential' },
  {
    type: 'JSON_SECRET_FIELD',
    re: /"(?:password|passwd|pwd|secret|client_secret|api_key|apikey|access_token|refresh_token|token)"\s*:\s*"([^"]{4,})"/gi,
    group: 1, cls: 'credential',
  },
  {
    type: 'ASSIGNED_SECRET_LITERAL',
    re: /((?:password|passwd|pwd|secret|api[_-]?key|access[_-]?token|EBK_JWT_SECRET)\s*[:=]\s*["'])([^"']{4,})["']/gi,
    group: 2, cls: 'review',
  },
];

const TEST_FIXTURE = /(?:^|\/)[^/]*_test\.go$|test-redaction\.js$|ci\.yml$|\.example$/i;

/** 头值是否为「看起来像真实令牌」的形态；`Bearer <token>` 之类说明文字返回 false。 */
function looksLikeRealSecret(v) {
  if (/eyJ[A-Za-z0-9_-]{8,}\./.test(v)) return true;
  const t = String(v).replace(/^Bearer\s+/i, '').replace(/^["'`]+|["'`]+$/g, '');
  return /^[A-Za-z0-9._~+/=-]{20,}$/.test(t);
}

const FILENAME_PAT = /(?:^|[\\/])(?:\.env(?:\.|$)|[^\\/]*(?:token|secret|credential)[^\\/]*\.(?:txt|json|env|pem|key|log))$/i;
const PERSONAL_PATH = /[A-Za-z]:[/\\]Users[/\\][^/\s"'<>]+|\/c\/Users\/[^/\s"'<>]+|\/home\/[^/\s"'<>]+\/WorkBuddy/gi;

function walk(dir, out) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (SKIP_DIRS.has(entry.name)) continue;
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(p, out);
    else out.push(p);
  }
}

function redactLine(line) {
  let out = line;
  for (const p of PATTERNS) {
    // 使用独立副本，避免与主扫描循环的 exec() 共享 lastIndex。
    const re = new RegExp(p.re.source, p.re.flags);
    out = out.replace(re, (m, ...args) => {
      // replace 回调：args 的最后两项是 offset 与完整字符串，前面依次是捕获组。
      const captures = args.slice(0, -2);
      const value = p.group === 0 ? m : captures[p.group - 1];
      if (value === undefined) return m;
      return m.replace(value, `[REDACTED:LEN=${String(value).length}]`);
    });
  }
  const absRe = new RegExp(PERSONAL_PATH.source, PERSONAL_PATH.flags);
  return out.replace(absRe, '[PERSONAL_PATH]').slice(0, 160);
}

function main() {
  const argv = process.argv.slice(2);
  const asJson = argv.includes('--json');
  const strict = argv.includes('--strict');
  const outIdx = argv.indexOf('--out');
  const outFile = outIdx >= 0 ? argv[outIdx + 1] : null;
  if (outIdx >= 0 && !outFile) {
    process.stderr.write('参数错误：--out 缺少路径\n');
    process.exit(1);
  }

  // 报告文件本身不参与扫描（无论是否指定 --out），避免自引用导致命中数膨胀。
  const DEFAULT_REPORT = 'docs/r0-screenshots/security-scan.md';
  const outRel = outFile
    ? (path.isAbsolute(outFile) ? null : outFile.replace(/\\/g, '/'))
    : DEFAULT_REPORT;

  const files = [];
  walk(REPO_ROOT, files);
  let scanned = 0;
  let binarySkipped = 0;
  const findings = [];

  for (const abs of files) {
    const rel = path.relative(REPO_ROOT, abs).split(path.sep).join('/');
    if (outRel && rel === outRel) continue;
    const ext = path.extname(abs).toLowerCase();
    if (BINARY_EXT.has(ext)) { binarySkipped += 1; continue; }
    let stat;
    try { stat = fs.statSync(abs); } catch (_) { continue; }
    if (stat.size > MAX_BYTES) { binarySkipped += 1; continue; }
    let text;
    try { text = fs.readFileSync(abs, 'utf8'); } catch (_) { binarySkipped += 1; continue; }
    scanned += 1;

    if (FILENAME_PAT.test(rel)) {
      findings.push({ file: rel, line: 0, type: 'SENSITIVE_FILENAME', kind: 'portability', context: '[文件名匹配 token|secret|credential|.env 规则]' });
    }

    const lines = text.split(/\r?\n/);
    lines.forEach((line, i) => {
      for (const p of PATTERNS) {
        p.re.lastIndex = 0;
        let m;
        while ((m = p.re.exec(line)) !== null) {
          const value = m[p.group];
          if (value === undefined) continue;
          let kind;
          const descriptiveHeader = (p.type === 'AUTH_HEADER' || p.type === 'COOKIE_HEADER')
            && !looksLikeRealSecret(value);
          if (SYNTHETIC.test(value) || descriptiveHeader) kind = 'synthetic';
          else if (p.cls === 'review') kind = 'review';
          else if (TEST_FIXTURE.test(rel)) kind = 'test-fixture';
          else kind = 'credential-candidate';
          findings.push({
            file: rel,
            line: i + 1,
            type: p.type,
            kind,
            context: redactLine(line).trim(),
          });
          if (p.re.lastIndex === m.index) p.re.lastIndex += 1;
        }
      }
      PERSONAL_PATH.lastIndex = 0;
      let pm;
      while ((pm = PERSONAL_PATH.exec(line)) !== null) {
        findings.push({
          file: rel, line: i + 1, type: 'PERSONAL_ABS_PATH', kind: 'portability',
          context: redactLine(line).trim(),
        });
        if (PERSONAL_PATH.lastIndex === pm.index) PERSONAL_PATH.lastIndex += 1;
      }
    });
  }

  const real = findings.filter((f) => f.kind === 'credential-candidate');
  const synthetic = findings.filter((f) => f.kind === 'synthetic');
  const review = findings.filter((f) => f.kind === 'review');
  const fixture = findings.filter((f) => f.kind === 'test-fixture');
  const portability = findings.filter((f) => f.kind === 'portability');

  const byFile = {};
  for (const f of findings) {
    const k = `${f.file}|${f.type}|${f.kind}`;
    byFile[k] = byFile[k] || { file: f.file, type: f.type, kind: f.kind, count: 0, context: f.context };
    byFile[k].count += 1;
  }
  const rows = Object.values(byFile).sort((a, b) => a.file.localeCompare(b.file) || a.type.localeCompare(b.type));

  const report = {
    generatedAt: new Date().toISOString(),
    repoRoot: '<repo-root>（由脚本位置推导，不记录绝对路径）',
    scannedTextFiles: scanned,
    binaryOrOversizeSkipped: binarySkipped,
    strict,
    realCredentialCandidates: real.length,
    syntheticOrPlaceholderHits: synthetic.length,
    manualReviewHits: review.length,
    testFixtureHits: fixture.length,
    portabilityHits: portability.length,
    rows,
  };

  let output;
  if (asJson) {
    output = JSON.stringify(report, null, 2) + '\n';
  } else {
    const lines = [
      '# FastLedger R0 凭证安全扫描报告',
      '',
      `- 生成时间：${report.generatedAt}`,
      `- 仓库根：${report.repoRoot}`,
      `- 扫描文本文件：${scanned}；跳过二进制/超大文件：${binarySkipped}`,
      `- \`--strict\`：${strict ? '启用' : '未启用'}`,
      `- **真实凭证候选命中：${real.length}**（参与 --strict）；合成/占位：${synthetic.length}；人工复核：${review.length}；测试夹具：${fixture.length}；可移植性：${portability.length}`,
      '',
      '> 本报告只记录文件路径、命中类型、数量与脱敏上下文，不记录任何敏感值。',
      '> `--strict` 仅对自动可判定的凭证字面量（JWT / Bearer / 头值 / JSON 密码字段）判定；',
      '> 代码赋值表达式与测试夹具仍全部列出，供人工/Owner 复核，不自动判为泄露。',
      '',
      '| 文件 | 命中类型 | 数量 | 判定 | 脱敏上下文 |',
      '|---|---|---:|---|---|',
    ];
    for (const r of rows) {
      const ctx = r.context.replace(/\|/g, '\\|') || '—';
      lines.push(`| \`${r.file}\` | ${r.type} | ${r.count} | ${r.kind} | ${ctx} |`);
    }
    if (rows.length === 0) lines.push('| — | — | 0 | — | 无命中 |');
    output = lines.join('\n') + '\n';
  }

  process.stdout.write(output);
  if (outFile) {
    const target = path.isAbsolute(outFile) ? outFile : path.join(REPO_ROOT, outFile);
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.writeFileSync(target, output, 'utf8');
    process.stderr.write(`报告已写入：${outFile}\n`);
  }
  if (strict && real.length > 0) process.exitCode = 2;
}

main();
