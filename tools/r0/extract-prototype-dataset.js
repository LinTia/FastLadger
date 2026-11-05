#!/usr/bin/env node
'use strict';
/**
 * 提取原型（prototype/）内置演示数据集，输出可复现 JSON。
 *
 * 为什么需要：R0 的"13 页成对对照"若两边数据不同，截图差异无法归因于实现偏差。
 * 原型把种子数据内联在打包产物里（无 localStorage 初始值），因此必须从 bundle 中
 * 精确提取 `var lt/dt/ft/pt/ht/gt/_t` 这些字面量，作为两侧统一数据集的来源。
 *
 * 用法：
 *   node tools/r0/extract-prototype-dataset.js [--out <path>]
 * 默认输出：tools/r0/prototype-dataset.json
 *
 * 退出码：0 成功；1 提取失败；2 参数错误
 */

const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');

const REPO_ROOT = path.resolve(__dirname, '..', '..');
const PROTO_DIR = path.join(REPO_ROOT, 'prototype');

function fail(msg, code) {
  process.stderr.write(`ERROR: ${msg}\n`);
  process.exit(code);
}

function parseArgv(argv) {
  const out = { out: path.join(__dirname, 'prototype-dataset.json') };
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i] === '--out') {
      out.out = path.resolve(argv[i + 1] || '');
      i += 1;
    } else if (argv[i] === '--help' || argv[i] === '-h') {
      process.stdout.write('usage: node tools/r0/extract-prototype-dataset.js [--out <path>]\n');
      process.exit(0);
    } else {
      fail(`未知参数 ${argv[i]}`, 2);
    }
  }
  return out;
}

/** 从 `start`（指向 '[' 或 '{'）开始做括号配对，跳过字符串与模板字符串。 */
function sliceLiteral(src, start) {
  const open = src[start];
  const close = open === '[' ? ']' : '}';
  let depth = 0;
  let i = start;
  while (i < src.length) {
    const ch = src[i];
    if (ch === '"' || ch === "'" || ch === '`') {
      const quote = ch;
      i += 1;
      while (i < src.length) {
        if (src[i] === '\\') { i += 2; continue; }
        if (src[i] === quote) { i += 1; break; }
        i += 1;
      }
      continue;
    }
    if (ch === open) depth += 1;
    else if (ch === close) {
      depth -= 1;
      if (depth === 0) return src.slice(start, i + 1);
    }
    i += 1;
  }
  throw new Error('括号未闭合，bundle 结构可能已变化');
}

/**
 * 取 `<name>=<literal>` 中的字面量源码。
 * 打包产物里这些常量可能是 `var a=[...],dt=[...],ft=[...]` 的逗号连写，
 * 因此不要求 `var` 前缀；同时用"字面量里必须像数据"做去歧义（同名局部变量很多）。
 */
function extractLiteral(src, name) {
  const re = new RegExp(`(?:^|[\\s,;])(?:var\\s+)?${name}\\s*=\\s*(?=[[{])`, 'g');
  let m;
  while ((m = re.exec(src))) {
    const at = m.index + m[0].length;
    let literal;
    try {
      literal = sliceLiteral(src, at);
    } catch {
      continue;
    }
    // 数据字面量的判据：含 id/name 等字段，且长度合理（避免命中 `{}` 等空壳）
    if (literal.length < 20) continue;
    if (!/(id|name|nickname|label)\s*:/.test(literal)) continue;
    return literal;
  }
  throw new Error(`未找到 ${name} 的数据字面量`);
}

/** 取 `<name>=` 后面的简单常量值（如 `mt=`total``），用于补齐字面量里的外部引用。 */
function resolveSimpleConst(src, name) {
  const patterns = [
    new RegExp(`(?:^|[\\s,;])(?:var\\s+)?${name}\\s*=\\s*\`([^\`]*)\``),
    new RegExp(`(?:^|[\\s,;])(?:var\\s+)?${name}\\s*=\\s*'([^']*)'`),
    new RegExp(`(?:^|[\\s,;])(?:var\\s+)?${name}\\s*=\\s*"([^"]*)"`),
    new RegExp(`(?:^|[\\s,;])(?:var\\s+)?${name}\\s*=\\s*(-?[0-9.]+)\\s*[,;]`),
  ];
  for (const re of patterns) {
    const m = src.match(re);
    if (m) {
      const raw = m[1];
      const asNum = Number(raw);
      return raw !== '' && Number.isFinite(asNum) && String(asNum) === raw ? asNum : raw;
    }
  }
  return undefined;
}

/**
 * 在受控作用域里求值数据字面量。
 * 字面量可能引用同文件的简单常量（例如预算表的 `mt=`total``），
 * 遇到 `X is not defined` 时只允许从 bundle 里取"简单字符串/数字常量"，不执行任何代码。
 */
function evalLiteral(literal, ctx) {
  const scope = { ct: ctx.ct, mt: ctx.mt };
  for (let attempt = 0; attempt < 8; attempt += 1) {
    const keys = Object.keys(scope);
    try {
      // eslint-disable-next-line no-new-func
      return new Function(...keys, `return (${literal});`)(...keys.map((k) => scope[k]));
    } catch (e) {
      const missing = /^(\w+) is not defined$/.exec(e.message);
      if (!missing) throw e;
      const value = resolveSimpleConst(ctx.src, missing[1]);
      if (value === undefined) throw new Error(`${missing[1]} 无法从 bundle 中解析为简单常量`);
      scope[missing[1]] = value;
    }
  }
  throw new Error('依赖补齐次数超过上限');
}

function main() {
  const { out } = parseArgv(process.argv.slice(2));
  if (!fs.existsSync(PROTO_DIR)) fail(`找不到原型目录 ${PROTO_DIR}`, 1);
  const bundles = fs.readdirSync(path.join(PROTO_DIR, 'assets'))
    .filter((f) => /^index-.*\.js$/.test(f));
  if (bundles.length !== 1) fail(`原型 assets 下应只有一个 index-*.js，实际 ${bundles.length} 个`, 1);

  const bundlePath = path.join(PROTO_DIR, 'assets', bundles[0]);
  const src = fs.readFileSync(bundlePath, 'utf8');
  const bundleSha256 = crypto.createHash('sha256').update(src).digest('hex');

  // `ct(daysAgo, hour, minute)` 是 bundle 里的相对日期构造器，提取时必须提供同语义实现
  const ct = (daysAgo, hour = 0, minute = 0) => {
    const d = new Date();
    d.setDate(d.getDate() - daysAgo);
    d.setHours(hour, minute, 0, 0);
    return d.toISOString();
  };

  const names = { accounts: 'dt', categories: 'ft', transactions: 'lt', investments: 'pt', budgets: 'ht', family: 'gt', profile: '_t' };
  const data = {};
  for (const [key, varName] of Object.entries(names)) {
    const literal = extractLiteral(src, varName);
    let value;
    try {
      value = evalLiteral(literal, { ct, mt: 'total', src });
    } catch (e) {
      fail(`解析 ${varName}(${key}) 失败：${e.message}`, 1);
    }
    if (value === undefined || value === null) fail(`${key} 解析为空`, 1);
    data[key] = value;
  }

  const payload = {
    $comment: '由 tools/r0/extract-prototype-dataset.js 从原型 bundle 提取，勿手改；余额单位为元。',
    source: {
      bundle: path.relative(REPO_ROOT, bundlePath).replace(/\\/g, '/'),
      bundleSha256,
      extractedAt: new Date().toISOString(),
      extractor: 'tools/r0/extract-prototype-dataset.js',
    },
    counts: {
      accounts: data.accounts.length,
      categories: data.categories.length,
      transactions: data.transactions.length,
      investments: Array.isArray(data.investments) ? data.investments.length : 0,
      budgets: Array.isArray(data.budgets) ? data.budgets.length : 0,
      family: Array.isArray(data.family) ? data.family.length : 0,
      profile: data.profile && typeof data.profile === 'object' ? Object.keys(data.profile).length : 0,
    },
    data,
  };

  fs.mkdirSync(path.dirname(out), { recursive: true });
  fs.writeFileSync(out, `${JSON.stringify(payload, null, 2)}\n`, 'utf8');

  process.stdout.write(`source=${payload.source.bundle}\n`);
  process.stdout.write(`bundleSha256=${bundleSha256}\n`);
  process.stdout.write(`out=${path.relative(REPO_ROOT, out).replace(/\\/g, '/')}\n`);
  process.stdout.write(`counts=${JSON.stringify(payload.counts)}\n`);
}

main();
