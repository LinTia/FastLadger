#!/usr/bin/env node
'use strict';
/**
 * 包 B · 分类映射回归审计（每次改动解析规则后必须跑）。
 *
 * 起一个临时实例 → 用 tools/r0/seed-from-dataset.js 把原型数据集种进去 →
 * 读 seed-manifest.json 统计 via 分布 → 与基线比对。
 *
 * 基线（第六次整改：显式冲突规则生效后）：
 *   id 17 / expand-child 4 / manual-override 1 / 其它 0，共 22 笔
 *
 * 与第五次整改的差异（落点不变，只有"命中路径"的更精确归类 + 1 笔改判）：
 *   · t007/t009/t010/t017 之前记为 name-path/name-prefix，现记为 expand-child——
 *     因为它们的 categoryId 本就是父类，规则"父类 ID + 子类展示名 → 子类"先行命中；
 *     最终落点与第五次完全一致（shopping_clothes / travel_hotel / travel_ticket / shopping_digital）。
 *   · t021 之前被"精确 ID"静默判为 meal（三餐），但其展示名是「餐饮-聚餐」= dinner；
 *     新规则判为冲突并失败关闭，由 category-overrides.json 显式改判为 dinner（落点变化）。
 *
 * 退出码：0 = 与基线一致；1 = 出现漂移或解析失败
 */
const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { startServer } = require('../lib/server');

const REPO = path.resolve(__dirname, '..', '..', '..', '..');
const BASELINE = { id: 17, 'expand-child': 4, 'manual-override': 1, 'name-path': 0, 'name-prefix': 0, 'parent-fallback': 0, 'id-normalized': 0 };
const TOTAL = 22;

(async () => {
  const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-audit-'));
  const runDir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-audit-rt-'));
  const server = await startServer({ dataDir });
  try {
    server.createUser('audit', 'R0audit-local-2026', { owner: true });
    const out = execFileSync(process.execPath, [path.join(REPO, 'tools', 'r0', 'seed-from-dataset.js')], {
      env: {
        ...process.env,
        R0_BASE: server.base,
        R0_USER: 'audit',
        R0_PW: 'R0audit-local-2026',
        R0_RUNTIME_DIR: runDir,
      },
      encoding: 'utf8',
    });
    process.stdout.write(out.split(/\r?\n/).slice(-4).join('\n') + '\n');

    const manifest = JSON.parse(fs.readFileSync(path.join(runDir, 'seed-manifest.json'), 'utf8'));
    const counts = {};
    for (const row of manifest.categoryAudit) counts[row.via] = (counts[row.via] || 0) + 1;

    const rows = manifest.categoryAudit.map((r) => `${r.txId}\t${r.protoCategoryId}\t${r.protoDisplayName}\t->\t${r.resolvedProtoCategoryId}\t${r.resolvedName}\tvia=${r.via}`);
    process.stdout.write(rows.join('\n') + '\n');
    process.stdout.write(`\nvia 分布：${JSON.stringify(counts)}（共 ${manifest.categoryAudit.length} 笔）\n`);
    process.stdout.write(`冲突记录：${JSON.stringify(manifest.categoryConflicts || [])}\n`);

    const drift = Object.entries(BASELINE).filter(([k, v]) => (counts[k] || 0) !== v);
    const totalOk = manifest.categoryAudit.length === TOTAL;
    if (drift.length || !totalOk) {
      process.stderr.write(`AUDIT_DRIFT 分类映射与基线不一致：${JSON.stringify(drift)} total=${manifest.categoryAudit.length}\n`);
      process.exitCode = 1;
    } else {
      process.stdout.write('AUDIT_OK 与基线一致（id 17 / expand-child 4 / manual-override 1 / 父类兜底 0）\n');
    }
  } catch (e) {
    process.stderr.write(`AUDIT_ERROR ${e && e.message ? e.message : e}\n`);
    process.exitCode = 1;
  } finally {
    await server.stop();
    try { fs.rmSync(dataDir, { recursive: true, force: true }); } catch (_) { /* ignore */ }
    try { fs.rmSync(runDir, { recursive: true, force: true }); } catch (_) { /* ignore */ }
  }
})();
