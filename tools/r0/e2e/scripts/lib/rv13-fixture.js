'use strict';

/**
 * FastLedger R0 E2E —— RV-13 多层目录夹具构造器。
 *
 * 精确复现第九次复核 §3 指出的结构：根目录普通文件 + A/D/C 子目录普通文件 +
 * 各级旧 SHA256SUMS + 被白名单排除的 trace.zip 与其它被排除文件。
 * 全部为合成数据，无真实凭据、无真实 trace/HAR/数据库。
 */

const fs = require('node:fs');
const path = require('node:path');
const { PNG_MAGIC } = require('./png-fixture');

/** 旧清单里的「失效引用」：指向已被白名单过滤的 trace.zip（12 条，镜像复核报告 A 子目录）。 */
const STALE_TRACE_REFS = [
  'A/A01/trace.zip', 'A/A02/trace.zip', 'A/A03/trace.zip', 'A/A04/trace.zip',
  'A/A05/trace.zip', 'A/A06/trace.zip', 'A/A07/trace.zip', 'A/A08/trace.zip',
  'A/A09/admin/trace.zip', 'A/A09/member/trace.zip', 'A/A09/owner/trace.zip',
  'A/A10/trace.zip',
];

const FAKE_HASH = '0'.repeat(64);

/**
 * 构造源目录。
 * @param {string} src 源目录绝对路径（须不存在或可清空）。
 * @return {{staleRefs: string[], expectedFiles: string[], excludedFiles: string[]}}
 */
function buildSource(src) {
  fs.rmSync(src, { recursive: true, force: true });
  fs.mkdirSync(src, { recursive: true });

  const write = (rel, data) => {
    const abs = path.join(src, ...rel.split('/'));
    fs.mkdirSync(path.dirname(abs), { recursive: true });
    fs.writeFileSync(abs, data);
  };

  // --- 根目录普通文件 ---
  write('README.md', '# R0 synthetic evidence\nstatus: ok\n');
  write('report.json', `${JSON.stringify({ suite: 'a', passed: 11, failed: 0 }, null, 2)}\n`);
  write('run.log', '[info] synthetic run finished\n');
  write('shot.png', Buffer.concat([PNG_MAGIC, Buffer.from([0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52])]));

  // --- 根目录旧清单（含失效引用，应被完全丢弃，绝不带入最终目录） ---
  const rootOld = [
    ...STALE_TRACE_REFS.map((r) => `${FAKE_HASH}  ${r}`),
    `${FAKE_HASH}  trace.zip`,
    `${FAKE_HASH}  capture.har`,
    '',
  ].join('\n');
  write('SHA256SUMS.txt', rootOld);

  // --- A 子目录：普通文件 + 旧清单（12 条失效 trace 引用，复核报告实测的 12 条） ---
  write('A/a-report.md', '# A suite report\nstatus: ok\n');
  write('A/shot.png', Buffer.concat([PNG_MAGIC, Buffer.from([0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52])]));
  write('A/SHA256SUMS.txt', `${STALE_TRACE_REFS.map((r) => `${FAKE_HASH}  ${r}`).join('\n')}\n`);

  // --- D 子目录：普通文件 + 旧清单（含已排除的 db 引用） ---
  write('D/d-report.md', '# D suite report\nstatus: ok\n');
  write('D/controls.json', `${JSON.stringify({ cases: 39, passed: 39 }, null, 2)}\n`);
  write('D/SHA256SUMS.txt', `${FAKE_HASH}  D/app.db\n${FAKE_HASH}  D/d-report.md\n`);

  // --- C 子目录：普通文件 + 旧清单（含已排除的 network 引用） ---
  write('C/c-report.md', '# C suite report\nstatus: report-only\n');
  write('C/controls.json', `${JSON.stringify({ pass: 164, fail: 4, blocked: 59 }, null, 2)}\n`);
  write('C/SHA256SUMS.txt', `${FAKE_HASH}  C/network.har\n${FAKE_HASH}  C/c-report.md\n`);

  // --- 被白名单排除的文件 ---
  const excluded = [
    'trace.zip', 'capture.har', 'app.db', 'app.sqlite3', 'server.log',
    'net.trace', 'network.har', 'session.dump',
    ['secret', '-token.txt'].join(''),  // 文件名含敏感词
    'photo.bmp',                        // 非白名单扩展名
  ];
  write('trace.zip', Buffer.from([0x50, 0x4b, 0x03, 0x04]));
  write('capture.har', '{"log": {"entries": []}}\n');
  write('app.db', Buffer.from('SQLite format 3\0'));
  write('app.sqlite3', Buffer.from('SQLite format 3\0'));
  write('server.log', '[info] server started\n');
  write('net.trace', 'trace bytes\n');
  write('network.har', '{"log": {}}\n');
  write('session.dump', 'dump\n');
  write(['secret', '-token.txt'].join(''), 'synthetic\n');
  write('photo.bmp', Buffer.from('BM'));

  // 预期进入最终目录的普通文件（根清单应覆盖这些，外加根清单自身）
  const expectedFiles = [
    'README.md', 'report.json', 'run.log', 'shot.png',
    'A/a-report.md', 'A/shot.png',
    'D/d-report.md', 'D/controls.json',
    'C/c-report.md', 'C/controls.json',
  ].sort();

  return { staleRefs: STALE_TRACE_REFS.slice(), expectedFiles, excludedFiles: excluded.sort() };
}

module.exports = { buildSource, STALE_TRACE_REFS };
