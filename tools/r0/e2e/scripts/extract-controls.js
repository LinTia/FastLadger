#!/usr/bin/env node
'use strict';
/**
 * 从 R0 主报告 §3.1 解析 228 条逐控件清单 → 机器可读 controls.json。
 *
 * 只做"读文档 + 结构化"，不改任何判定口径：
 *   · 每行 13 个字段原样保留（id/page/control/code/pre/input/action/success/fail/data/target/evidence/status）；
 *   · group 取最近的 `#### 3.1.x` 标题；
 *   · 文档 SHA256 一并记录，便于核对"清单版本与执行版本是否同一份"。
 *
 * 用法：node tools/r0/e2e/scripts/extract-controls.js [--check]
 *   默认写 tools/r0/e2e/fixtures/controls.json
 *   --check 只校验行数（应为 228）并打印分组统计，不写文件
 */

const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');

const REPO = path.resolve(__dirname, '..', '..', '..', '..');
const REPORT = path.join(REPO, 'docs', 'FastLedger-R0-基准冻结与差异盘点.md');
const OUT = path.join(REPO, 'tools', 'r0', 'e2e', 'fixtures', 'controls.json');
const EXPECT = 228;

function cells(line) {
  // | a | b | ... | → 去掉首尾空段
  const parts = line.split('|');
  parts.shift();
  parts.pop();
  return parts.map((s) => s.trim());
}

function main() {
  const check = process.argv.includes('--check');
  const raw = fs.readFileSync(REPORT, 'utf8');
  const sha256 = crypto.createHash('sha256').update(raw).digest('hex');
  const lines = raw.split(/\r?\n/);

  const controls = [];
  const groupOrder = [];
  let group = '(未分组)';

  for (const line of lines) {
    const h = line.match(/^####\s+(3\.1\.\d+)\s*(.*)$/);
    if (h) {
      group = `${h[1]} ${h[2]}`.trim();
      if (!groupOrder.includes(group)) groupOrder.push(group);
      continue;
    }
    if (!line.startsWith('| I-')) continue;
    const c = cells(line);
    if (c.length < 13) {
      process.stderr.write(`WARN 字段不足 13 个，跳过：${line.slice(0, 80)}\n`);
      continue;
    }
    const [id, page, control, code, pre, input, action, success, fail, data, target, evidence, status] = c;
    controls.push({ id, group, page, control, code, pre, input, action, success, fail, data, target, evidence, status });
  }

  const byGroup = {};
  for (const c of controls) byGroup[c.group] = (byGroup[c.group] || 0) + 1;

  process.stdout.write(`controls=${controls.length} groups=${groupOrder.length} reportSha256=${sha256.slice(0, 16)}…\n`);
  for (const g of groupOrder) process.stdout.write(`  ${g}: ${byGroup[g] || 0}\n`);

  if (controls.length !== EXPECT) {
    process.stderr.write(`ERROR 期望 ${EXPECT} 条，实际 ${controls.length} 条（文档结构可能已变）\n`);
    process.exit(1);
  }
  if (check) return;

  const payload = {
    source: 'docs/FastLedger-R0-基准冻结与差异盘点.md §3.1',
    sourceSha256: sha256,
    extractedAt: new Date().toISOString(),
    count: controls.length,
    groups: groupOrder,
    controls,
  };
  fs.mkdirSync(path.dirname(OUT), { recursive: true });
  fs.writeFileSync(OUT, JSON.stringify(payload, null, 2) + '\n');
  process.stdout.write(`WROTE ${path.relative(REPO, OUT)}\n`);
}

main();
