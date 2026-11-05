#!/usr/bin/env node
'use strict';
/**
 * R0 自动化验收入口。
 *
 *   node run.js --suite a       # 包 A：11 项状态/权限/同步场景（门禁型）
 *   node run.js --suite c       # 包 C：228 条逐控件运行时验收（报告型）
 *   node run.js --suite d       # 包 D：第七次 4 项缺陷回归（含 I-TXNS-01..19 逐条重跑，门禁型）
 *   node run.js --suite all     # = A + D + C（全部适用套件），按 a → d → c 顺序执行
 *
 * 套件与分工（P2 整改，与 .github/workflows/r0-e2e.yml 对齐）：
 *   · A —— 11 项状态/权限/同步场景，门禁型；
 *   · C —— 228 条逐控件运行时验收，报告型（PASS/FAIL 分布本身就是结论，允许既有 FAIL）；
 *   · D —— 第七次 4 项已确认业务缺陷的回归，门禁型；
 *   · all —— 包含 A / D / C 三个套件。顺序取 a → d → c：让 D 在 C 的报告型写操作
 *           （编辑/删除首笔交易、改角色、退出登录、生成 Token 等）之前执行，避免 C 污染
 *           D 的回归数据；C 作为报告型放最后。B 不在 all 内（见下）。
 *   · B —— 「分类解析合成测试 + 22 笔分类映射回归审计」**不在本运行器 suite 体系内**，
 *           由 .github/workflows/r0-e2e.yml 的独立 step 执行：
 *             node --test tools/r0/test-resolve-category.js
 *             node tools/r0/e2e/scripts/reseed-audit.js   （env R0_SERVER_BIN=<server/fastledger>）
 *           因此 --suite all 不含 B，也不代表 B 无需运行 / 已通过。
 *
 * 环境变量：
 *   R0_SERVER_BIN     服务二进制路径（默认 server/fastledger[.exe]）
 *   R0_BROWSER_CHANNEL 本地用 msedge；CI 不设（用自带 chromium）
 *   R0_ARTIFACT_DIR   证据输出根目录（默认 <repo>/artifacts/r0-e2e）
 *   R0_DATA_DIR       复用固定实例目录（默认临时目录，运行结束即丢弃）
 *
 * 退出码：0 = 无 FAIL 且 A 无 BLOCKED；1 = 有 FAIL 或 A 有 BLOCKED；2 = 环境错误
 *   注意：C 为报告型且有既有 FAIL，故 `--suite all` 通常 exit 1，属预期，不代表 D 失败；
 *   CI 中 C 单步 continue-on-error 的绿色也不等于 228 条全部通过。
 */
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { startServer } = require('./lib/server');
const { launch } = require('./lib/browser');
const { RunDir, envInfo } = require('./lib/evidence');
const { bootstrap } = require('./lib/fixtures');
const aScenarios = require('./specs/a-scenarios');
const { runControls } = require('./specs/c-controls');
const { runDefects } = require('./specs/d-defects');

function parseArgs(argv) {
  const out = { suite: 'all' };
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i] === '--suite') out.suite = argv[++i];
    else if (argv[i] === '--keep-data') out.keepData = true;
  }
  return out;
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const runDir = new RunDir(args.suite);
  const dataDir = process.env.R0_DATA_DIR || fs.mkdtempSync(path.join(os.tmpdir(), 'fl-e2e-'));
  const report = {
    suite: args.suite,
    startedAt: new Date().toISOString(),
    env: null,
    server: null,
    results: [],
    summary: {},
  };

  let server;
  let browserHandle;
  try {
    server = await startServer({ dataDir });
    report.server = {
      base: server.base,
      port: server.port,
      dataDir: server.dataDir,
      health: server.health,
      binary: server.bin,
    };
    process.stdout.write(`SERVER up ${server.base} schema=${server.health && server.health.schemaVersion}\n`);

    const data = await bootstrap(server);
    data.origin = new URL(server.base).origin;
    runDir.writeJson('fixtures.json', {
      users: Object.fromEntries(Object.entries(data.users).map(([k, v]) => [k, { username: v.username }])),
      books: data.books,
      ids: {
        accounts: data.ids.accounts.map((a) => ({ id: a.id, name: a.name, category: a.category, balance: a.balance })),
        expenseCatId: data.ids.expenseCatId,
        txIds: data.ids.txIds,
        deletedTxId: data.ids.deletedTxId,
      },
    });

    browserHandle = await launch();
    report.env = envInfo({
      browser: { version: browserHandle.version, engine: browserHandle.from, channel: browserHandle.channel },
      server: { binary: path.basename(server.bin), health: server.health },
    });
    process.stdout.write(`BROWSER ${browserHandle.from} channel=${browserHandle.channel} version=${browserHandle.version}\n`);

    // P2：--suite all 必须包含全部适用套件 A / D / C（原先只有 A + C，漏掉 D）。
    // 顺序 a → d → c：D 是门禁型回归，放在报告型 C 之前，避免 C 的写操作污染 D 的数据；
    // B（分类解析合成测试 + 22 笔审计）由 .github/workflows/r0-e2e.yml 独立 step 执行，
    // 不在本运行器 suite 体系内，故 all 不含 B。不得为了让 all 变绿而放宽 C 断言或跳过 C。
    const suites = args.suite === 'all' ? ['a', 'd', 'c'] : [args.suite];
    for (const suite of suites) {
      if (suite === 'a') {
        for (const sc of aScenarios) {
          const started = Date.now();
          let r;
          try {
            r = await sc.run({ browser: browserHandle.browser, base: server.base, data, runDir });
          } catch (e) {
            r = { status: 'FAIL', detail: `执行异常：${e && e.message ? e.message : e}`, evidence: [] };
          }
          r.evidence = (r.evidence || []).map((p) => runDir.rel(p));
          const row = { suite: 'A', id: sc.id, name: sc.name, status: r.status, detail: r.detail, evidence: r.evidence, ms: Date.now() - started };
          report.results.push(row);
          process.stdout.write(`${row.status.padEnd(7)} ${row.id} ${row.name} (${row.ms}ms)\n        ${row.detail}\n`);
        }
        // 与 C / D 对称输出套件汇总，便于 `--suite all` 日志直接看出 A 也已执行并有汇总。
        const aTally = report.results
          .filter((r) => r.suite === 'A')
          .reduce((acc, r) => { acc[r.status] = (acc[r.status] || 0) + 1; return acc; }, {});
        process.stdout.write(`A 汇总：${JSON.stringify(aTally)}\n`);
      } else if (suite === 'c') {
        const rows = await runControls({ browser: browserHandle.browser, base: server.base, data, runDir });
        for (const r of rows) {
          report.results.push(r);
        }
        const tally = rows.reduce((acc, r) => { acc[r.status] = (acc[r.status] || 0) + 1; return acc; }, {});
        process.stdout.write(`C 汇总：${JSON.stringify(tally)}\n`);
      } else if (suite === 'd') {
        const rows = await runDefects({ browser: browserHandle.browser, base: server.base, data, runDir });
        for (const r of rows) {
          report.results.push(r);
        }
        const tally = rows.reduce((acc, r) => { acc[r.status] = (acc[r.status] || 0) + 1; return acc; }, {});
        process.stdout.write(`D 汇总：${JSON.stringify(tally)}\n`);
      } else {
        throw new Error(`未知 suite：${suite}`);
      }
    }

    report.finishedAt = new Date().toISOString();
    report.summary = report.results.reduce((acc, r) => {
      const k = `${r.suite}:${r.status}`;
      acc[k] = (acc[k] || 0) + 1;
      return acc;
    }, {});
    report.summary.total = report.results.length;

    runDir.writeJson('report.json', report);
    runDir.writeText('report.md', renderMarkdown(report));
    const sums = runDir.checksums();
    process.stdout.write(`\nSUMMARY ${JSON.stringify(report.summary)}\nARTIFACTS ${runDir.dir}\nCHECKSUMS ${sums.count} files\n`);

    const fails = report.results.filter((r) => r.status === 'FAIL');
    const aBlocked = report.results.filter((r) => r.suite === 'A' && r.status === 'BLOCKED');
    if (fails.length || aBlocked.length) {
      process.stdout.write(`\n失败明细：\n${fails.concat(aBlocked).map((r) => `  ${r.id} ${r.status} ${r.detail}`).join('\n')}\n`);
      process.exitCode = 1;
    }
  } catch (e) {
    process.stderr.write(`E2E_ENV_ERROR ${e && e.stack ? e.stack : e}\n`);
    process.exitCode = 2;
  } finally {
    if (browserHandle) await browserHandle.browser.close().catch(() => {});
    if (server) await server.stop().catch(() => {});
    if (!args.keepData) {
      try { fs.rmSync(dataDir, { recursive: true, force: true }); } catch (_) { /* ignore */ }
    }
  }
}

function renderMarkdown(report) {
  const lines = [];
  lines.push(`# R0 自动化验收报告（suite=${report.suite}）`);
  lines.push('');
  lines.push(`- 开始：${report.startedAt}`);
  lines.push(`- 结束：${report.finishedAt}`);
  lines.push(`- commit：\`${report.env ? report.env.gitSha : 'unknown'}\``);
  lines.push(`- 浏览器：${report.env ? `${report.env.browser.engine} ${report.env.browser.version}（${report.env.browser.channel}）` : '-'}`);
  lines.push(`- 实例：${report.server ? report.server.base : '-'}（schema ${report.server && report.server.health ? report.server.health.schemaVersion : '-'}）`);
  lines.push('');
  lines.push('## 汇总');
  lines.push('');
  lines.push('| 套件 | 状态 | 数量 |');
  lines.push('|---|---|---|');
  for (const [k, v] of Object.entries(report.summary)) {
    if (k === 'total') continue;
    const [suite, status] = k.split(':');
    lines.push(`| ${suite} | ${status} | ${v} |`);
  }
  lines.push('');
  lines.push('## 明细');
  lines.push('');
  lines.push('| 套件 | ID | 名称 | 状态 | 说明 |');
  lines.push('|---|---|---|---|---|');
  for (const r of report.results) {
    lines.push(`| ${r.suite} | ${r.id} | ${r.name} | **${r.status}** | ${String(r.detail || '').replace(/\|/g, '\\|')} |`);
  }
  lines.push('');
  return lines.join('\n');
}

main();
