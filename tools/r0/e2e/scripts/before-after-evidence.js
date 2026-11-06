'use strict';
// Before/after evidence harness: runs BOTH the pre-fix (git 659c0c3) scanner and the
// current (fixed) scanner against identical synthetic K1-K3 / RV-09 samples, recording exit codes.
// Output is used as evidence in the P1 remediation report. No real credentials are used.
// 659c0c3 是第三方独立复核的对照基线（PR #3 最新提交），代表修复前状态。
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const REPO = path.resolve(__dirname, '..', '..', '..', '..');
const NEW_SCANNER = path.join(REPO, 'tools/r0/e2e/scripts/scan-artifacts.js');
const OLD_SCANNER = path.join(os.tmpdir(), 'old-scan-artifacts.js');
const BASELINE = '659c0c3';

// Extract the pre-fix scanner from the audit baseline commit.
const extract = spawnSync('git', ['show', `${BASELINE}:tools/r0/e2e/scripts/scan-artifacts.js`], { cwd: REPO, encoding: 'utf8' });
if (extract.status !== 0) {
  console.error(`无法从 ${BASELINE} 提取旧扫描器：`, extract.stderr);
  process.exit(2);
}
fs.writeFileSync(OLD_SCANNER, extract.stdout);

function encodeUtf16BE(s) {
  const out = Buffer.alloc(s.length * 2);
  for (let i = 0; i < s.length; i += 1) {
    const c = s.charCodeAt(i);
    out[i * 2] = (c >> 8) & 0xff;
    out[i * 2 + 1] = c & 0xff;
  }
  return out;
}

function run(scanner, dir) {
  const r = spawnSync(process.execPath, [scanner, dir], { encoding: 'utf8' });
  if (r.status !== 2 && r.status !== 0) {
    // 仅当异常时打印 stderr，便于定位（正常 CLEAN/BLOCKED 不打扰输出）。
    process.stderr.write(`[DEBUG ${path.basename(scanner)}] rc=${r.status} stderr=${r.stderr || ''}\n`);
  }
  return r.status;
}

function makeSample(build) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ba-evidence-'));
  build(dir);
  return dir;
}

const synthetic = (tag) => ['SYNTHETIC', 'FAKE', tag, '000000000000'].join('');
const jwt = () => {
  const h = 'eyJ' + 'SYNTHETICFAKEHEADER';
  const b = 'SYNTHETICFAKEPAYLOAD0001';
  const s = 'SYNTHETICFAKESIGNATURE0002';
  return [h, b, s].join('.');
};

const samples = [
  ['K1-01 UTF-16LE Bearer', (d) => fs.writeFileSync(path.join(d, 'a.txt'), Buffer.from('Bearer ' + synthetic('UTF16LE'), 'utf16le'))],
  ['K1-02 UTF-16BE auth', (d) => fs.writeFileSync(path.join(d, 'a.txt'), encodeUtf16BE('Authorization: Bearer ' + synthetic('UTF16BE')))],
  ['K1-03 NUL insertion', (d) => fs.writeFileSync(path.join(d, 'a.txt'), 'Bearer ' + String.fromCharCode(0) + synthetic('NULINJ'), 'latin1')],
  ['K1-04 undecodable', (d) => {
    const buf = Buffer.alloc(64);
    for (let i = 0; i < buf.length; i += 1) buf[i] = (i * 37 + 11) % 256;
    fs.writeFileSync(path.join(d, 'a.bin'), buf);
  }],
  ['K2-01 short token', (d) => fs.writeFileSync(path.join(d, 'a.txt'), 'token: ab12\n', 'utf8')],
  ['K2-02 short api key', (d) => fs.writeFileSync(path.join(d, 'a.txt'), 'api_key: xy78\n', 'utf8')],
  ['K2-03 short password', (d) => fs.writeFileSync(path.join(d, 'a.txt'), 'password: pw9\n', 'utf8')],
  ['K2-04 business fields', (d) => fs.writeFileSync(path.join(d, 'a.txt'), JSON.stringify({ unit: 'kg', qty: 2, note: 'milk', code: 'A1' }) + '\n', 'utf8')],
  ['K3-01 x-api-key', (d) => fs.writeFileSync(path.join(d, 'a.txt'), 'x-api-key: ' + synthetic('XAPIKEY') + '\n', 'utf8')],
  ['K3-02 apiKey/API_KEY', (d) => fs.writeFileSync(path.join(d, 'a.txt'), 'apiKey: ' + synthetic('APIKEY') + '\nAPI_KEY=' + synthetic('APIKEY2') + '\n', 'utf8')],
  ['K3-03 Base64 cred', (d) => fs.writeFileSync(path.join(d, 'a.txt'), 'data=' + Buffer.from('Authorization: Bearer ' + jwt()).toString('base64') + '\n', 'utf8')],
  ['K3-04 JSON nested', (d) => fs.writeFileSync(path.join(d, 'a.txt'), JSON.stringify({ config: { auth: { token: 'SYNTHETIC-NESTED-TOKEN-99887766' } } }) + '\n', 'utf8')],
  ['K3-05 variants', (d) => fs.writeFileSync(path.join(d, 'a.txt'), 'X_Api_Key: ' + synthetic('VARKEY') + '\nclient-secret=' + synthetic('VARSECRET') + '\n', 'utf8')],
  // RV-09：非法 UTF-8 字节打断关键词 / token 值（旧宽松解码会判可靠而漏报；修复后严格解码 → BLOCK）。
  ['RV-09-01 illegal UTF-8 in Bearer', (d) => {
    const body = 'Authoriz: Bea' + String.fromCharCode(0xFF) + 'rer ' + synthetic('RV09A') + '\n' + 'x'.repeat(2000) + '\n';
    fs.writeFileSync(path.join(d, 'a.txt'), body, 'latin1');
  }],
  ['RV-09-02 illegal UTF-8 in token', (d) => {
    // 非法字节位于 token 值「前 12 字符之内」打断：旧宽松解码会判关键词/值为可靠而漏报（CLEAN）；
    // 修复后严格 UTF-8 失败 → 无法可靠解码 → BLOCK（噪声复扫亦能恢复连续串）。
    const body = 'Authorization: Bearer abc' + String.fromCharCode(0xFE) + 'def000000000000\n' + 'y'.repeat(2000) + '\n';
    fs.writeFileSync(path.join(d, 'a.txt'), body, 'latin1');
  }],
];

console.log('sample | old(659c0c3) rc | new(fixed) rc | old verdict | new verdict');
console.log('---');
let changed = 0;
for (const [name, build] of samples) {
  const dir = makeSample(build);
  const oldRc = run(OLD_SCANNER, dir);
  const newRc = run(NEW_SCANNER, dir);
  const oldV = oldRc === 0 ? 'CLEAN(bypass)' : (oldRc === 2 ? 'BLOCKED' : 'ERR');
  const newV = newRc === 0 ? 'CLEAN' : (newRc === 2 ? 'BLOCKED' : 'ERR');
  const fixed = oldRc === 0 && newRc === 2;
  if (fixed) changed += 1;
  console.log(`${name} | ${oldRc} | ${newRc} | ${oldV} | ${newV}${fixed ? '  << fixed' : ''}`);
  fs.rmSync(dir, { recursive: true, force: true });
}
console.log('---');
console.log(`新扫描器封死的可复现绕过数：${changed} / ${samples.length}`);
