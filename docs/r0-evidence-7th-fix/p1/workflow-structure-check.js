'use strict';

/**
 * P1 交付自证：r0-e2e.yml 结构检查（可复现，零第三方依赖）。
 *
 * 运行：node docs/r0-evidence-7th-fix/p1/workflow-structure-check.js
 * 输出：逐条 PASS/FAIL；全部通过时退出 0，否则退出 1。
 *
 * 检查重点：上传 step 是否真正依赖扫描 step 的 outcome、白名单 path 是否生效、
 * 以及 A / D / C / 汇总退出码四个既有 step 是否与 HEAD 逐字一致（禁止改动语义）。
 */

const fs = require('node:fs');
const path = require('node:path');
const { execSync } = require('node:child_process');

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..');
const WF_PATH = path.join(REPO_ROOT, '.github', 'workflows', 'r0-e2e.yml');
const CI_PATH = path.join(REPO_ROOT, '.github', 'workflows', 'ci.yml');
const WORKFLOW_DIR = path.join(REPO_ROOT, '.github', 'workflows');

const wf = fs.readFileSync(WF_PATH, 'utf8');
const ci = fs.readFileSync(CI_PATH, 'utf8');
const lines = wf.split('\n').map((line) => line.replace(/\r$/, ''));

/** @type {string[]} */
const out = [];
let allOk = true;

/**
 * 记录一条检查结果。
 * @param {string} label 检查项名称。
 * @param {boolean} cond 是否通过。
 * @param {string} detail 补充信息。
 */
function check(label, cond, detail) {
  if (!cond) allOk = false;
  out.push(`${cond ? '[PASS]' : '[FAIL]'} ${label}${detail ? ` :: ${detail}` : ''}`);
}

/**
 * 查找包含指定文本的首个行号（1-based）。
 * @param {string} needle 待查找文本。
 * @return {number} 行号，未找到返回 -1。
 */
function lineNo(needle) {
  const index = lines.findIndex((line) => line.includes(needle));
  return index < 0 ? -1 : index + 1;
}

const iPrepare = lineNo('id: artifact_prepare');
const iScan = lineNo('id: artifact_scan');
const iUpload = lineNo('uses: actions/upload-artifact@v4');
const iSummary = lineNo('name: 汇总退出码');

check(
  'step 顺序：汇总 < 准备 < 扫描 < 上传',
  iSummary > 0 && iSummary < iPrepare && iPrepare < iScan && iScan < iUpload,
  `行号 汇总=${iSummary} 准备=${iPrepare} 扫描=${iScan} 上传=${iUpload}`,
);

check(
  '扫描 step 的 if 依赖 prepare 的 outcome',
  wf.includes("if: ${{ always() && steps.artifact_prepare.outcome == 'success' }}"),
);
check(
  '上传 step 的 if 依赖 scan 的 outcome',
  wf.includes("if: ${{ always() && steps.artifact_scan.outcome == 'success' }}"),
);

// 上传 step 的 uses 行上一行就是它的 if 行；必须不是裸 always()。
const uploadIfLine = (lines[iUpload - 2] || '').trim();
check(
  '上传 step 的 if 行不是裸 always()',
  uploadIfLine.startsWith('if:') && !uploadIfLine.includes('if: always()'),
  uploadIfLine,
);

const iPath = lineNo('path: |');
const pathValue = (lines[iPath] || '').trim();
check('上传 path 指向白名单目录', pathValue === 'artifacts/r0-e2e-upload/**', pathValue);
check('if-no-files-found 改为 error', /if-no-files-found: error/.test(wf));
check(
  'workflow 全文无 trace 字样',
  !/trace/i.test(wf),
  `trace 出现次数=${(wf.match(/trace/gi) || []).length}`,
);
check(
  'C 报告型 continue-on-error: true 保留且仅一处',
  (wf.match(/continue-on-error: true/g) || []).length === 1,
);

// 与 HEAD 逐字比对 A / D / C / 汇总退出码 四个既有 step 的完整文本段。
const head = execSync('git show HEAD:.github/workflows/r0-e2e.yml', { encoding: 'utf8' });
/**
 * 取出「# A 是门禁」到汇总 step 结束之间的文本。
 * @param {string} text workflow 全文。
 * @return {string} 门禁语义段落。
 */
function gateSegment(text) {
  const start = text.indexOf('      # A 是门禁');
  const end = text.indexOf('          } >> "$GITHUB_STEP_SUMMARY"');
  return text.slice(start, end);
}
const headSegment = gateSegment(head);
const nowSegment = gateSegment(wf);
check(
  'A / D / C / 汇总退出码 四个既有 step 与 HEAD 逐字一致',
  headSegment === nowSegment,
  `字节数 HEAD=${headSegment.length} 当前=${nowSegment.length}`,
);

check('ci.yml 无 upload-artifact 步骤', !ci.includes('upload-artifact'));
check(
  '.github/workflows 下仅 ci.yml 与 r0-e2e.yml 两个文件',
  fs.readdirSync(WORKFLOW_DIR).sort().join(',') === 'ci.yml,r0-e2e.yml',
);
check(
  '全仓 workflow 中 upload-artifact 仅 1 处',
  (wf.match(/uses: actions\/upload-artifact@/g) || []).length === 1,
);

out.push('');
out.push(`结构检查总结论：${allOk ? 'PASS' : 'FAIL'}`);
process.stdout.write(`${out.join('\n')}\n`);
process.exitCode = allOk ? 0 : 1;
