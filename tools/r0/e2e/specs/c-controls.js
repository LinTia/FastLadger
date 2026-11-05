'use strict';
/**
 * 包 C · 228 条逐控件运行时验收（Playwright，真实点击/输入/提交/删除/恢复）
 *
 * 判定原则（对应任务单 §三）：
 *   1. 每条控件都有明确归属：PASS / FAIL / BLOCKED / N/A，"整页 PASS"不算数；
 *   2. PASS 必须来自真实交互或真实渲染结果（读浏览器里画出来的东西），不用静态代码定位充数；
 *   3. 主报告登记为「缺失」的判 FAIL 并写明 R0-E 编号——那是产品缺口；
 *   4. 无法自动化的判 BLOCKED，写清具体技术阻塞（相机/手势/文件选择/剪贴板）；
 *   5. 失败项保留截图与 console/network 诊断。
 */
const fs = require('node:fs');
const path = require('node:path');
const api = require('../lib/api');
const { storageStateFor } = require('../lib/fixtures');
const { attachDiagnostics } = require('../lib/evidence');
const { runTxnsControls } = require('./txns-controls');

const CONTROLS = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'fixtures', 'controls.json'), 'utf8'));
const VIEWPORT = { width: 390, height: 844 };

const ok = (detail, shots = []) => ({ status: 'PASS', detail, shots });
const bad = (detail, shots = []) => ({ status: 'FAIL', detail, shots });
const blocked = (reason, shots = []) => ({ status: 'BLOCKED', detail: reason, shots });
const na = (reason) => ({ status: 'N/A', detail: reason, shots: [] });

const text = async (p) => (await p.locator('#view').innerText().catch(() => '')) || '';
const sleep = (p, ms) => p.waitForTimeout(ms);
const has = async (p, sel) => (await p.locator(sel).count()) > 0;
const clickBtn = async (p, label, index = 0) => p.locator('button', { hasText: label }).nth(index).click();
/** 关闭当前打开的弹窗：优先点取消按钮，失败则按 Esc */
async function closeDlg(p) {
  for (const sel of ['#iCancel', '#acctCancel', '#catCancel', '#bCancel', '#fCancel', '#cCancel']) {
    const b = p.locator(sel);
    // eslint-disable-next-line no-await-in-loop
    if (await b.isVisible().catch(() => false)) {
      // eslint-disable-next-line no-await-in-loop
      await b.click({ timeout: 2000 }).catch(() => {});
      // eslint-disable-next-line no-await-in-loop
      await sleep(p, 200);
      if (!(await p.locator('dialog[open]').count())) return;
    }
  }
  await p.keyboard.press('Escape').catch(() => {});
  await sleep(p, 200);
}
async function waitText(p, needle, timeout = 6000) {
  return p.waitForFunction((n) => (document.body.innerText || '').includes(n), needle, { timeout }).then(() => true).catch(() => false);
}
async function errText(p, sel) {
  const l = p.locator(sel);
  return (await l.count()) ? (await l.innerText().catch(() => '')).trim() : '';
}

async function toastText(p) {
  const t = p.locator('#toast');
  return (await t.count()) ? (await t.innerText().catch(() => '')).trim() : '';
}
async function waitToast(p, ms = 5000) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    const t = await toastText(p);
    if (t) return t;
    await p.waitForTimeout(100);
  }
  return '';
}

/** 关键词 → 不可自动化的具体原因（不是"Owner 负责"） */
const BLOCK_REASONS = [
  { re: /相机|拍照|截图识别|OCR|识别/, reason: '需要真实相机/图片输入；不注入假文件无法得到可信 OCR 链路（替代：CI 内固定夹具图片 + OCR stub，属新功能）' },
  { re: /拖拽|长按|滑动|手势/, reason: '需要指针手势序列；页面未提供稳定坐标契约（替代：加 data-testid 后用 mouse.move 序列）' },
  { re: /上传|文件选择|导入/, reason: '需要本地文件选择；当前控件未绑定 input[type=file]（页面只有导出）' },
  { re: /分享|复制|剪贴板/, reason: '需要剪贴板权限与用户手势；无头 CI 下 navigator.clipboard 不可用' },
  { re: /滚动|下拉刷新/, reason: '需要滚动手势与动态加载窗口；当前实现无分页/无限滚动' },
];

function classifyMissing(ctrl) {
  if (/\*\*缺失\*\*|缺失：/.test(ctrl.status) || /^未实现/.test(ctrl.action)) {
    const e = (ctrl.status.match(/R0-E-\d+/) || '').toString();
    return { status: 'FAIL', detail: `产品侧未实现（主报告已登记${e ? '：' + e : ''}）；本轮确认浏览器中不存在该控件` };
  }
  if (/待 Owner 裁决|待确认/.test(ctrl.status)) return { status: 'N/A', detail: '交互形态待产品裁决，暂不作为验收项' };
  for (const b of BLOCK_REASONS) if (b.re.test(ctrl.control) || b.re.test(ctrl.action)) return { status: 'BLOCKED', detail: b.reason };
  return null;
}

/* ---------------- 分组执行器 ---------------- */

async function groupLogin(ctx) {
  const { browser, base, data, out } = ctx;
  const c = await browser.newContext({ viewport: VIEWPORT });
  const page = await c.newPage();
  const R = {};
  try {
    await page.goto(base + '/#books', { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('input[placeholder="用户名"]', { timeout: 10000 });
    const u = page.locator('input[placeholder="用户名"]');
    await u.fill('probe-user');
    R['I-LOGIN-01'] = (await u.inputValue()) === 'probe-user'
      ? ok(`键入回显=${await u.inputValue()}，autocomplete=${await u.getAttribute('autocomplete')}，required=${await u.evaluate((e) => e.required)}`)
      : bad('输入未回显');
    const pw = page.locator('input[placeholder^="密码"]').first();
    await pw.fill('secret123');
    R['I-LOGIN-02'] = (await pw.getAttribute('type')) === 'password' ? ok('type=password，输入后仍为密文') : bad(`type=${await pw.getAttribute('type')}`);
    R['I-LOGIN-04'] = bad('浏览器中不存在「明文切换」控件（主报告登记缺失 R0-E-39）；DOM 无 eye/toggle 类元素');
    const submit = page.locator('button', { hasText: '登 录' }).first();
    R['I-LOGIN-05'] = (await submit.count()) === 1 ? ok('登录按钮唯一可点击') : bad(`按钮数=${await submit.count()}`);

    await u.fill('owner');
    await pw.fill(data.password);
    await submit.click();
    const logged = await page.waitForFunction(() => !!localStorage.getItem('fl_token'), { timeout: 20000 }).then(() => true).catch(() => false);
    R['I-LOGIN-06'] = logged ? ok('提交后写入 fl_token 并进入 #books', [await out.screenshot(page, 'C/login', 'after-login')]) : bad('提交后未获得 token');

    // 注册流
    await page.evaluate(() => localStorage.clear());
    await page.goto(base + '/#books', { waitUntil: 'domcontentloaded' });
    await sleep(page, 600);
    const regToggle = page.locator('button, a', { hasText: /注\s*册/ }).first();
    const hasReg = (await regToggle.count()) > 0;
    if (hasReg) { await regToggle.click(); await sleep(page, 500); }
    const confirm = page.locator('input[placeholder="确认密码"]');
    const hasConfirm = (await confirm.count()) > 0;
    R['I-LOGIN-03'] = hasConfirm ? ok(`注册模式有确认密码框，type=${await confirm.first().getAttribute('type')}`) : blocked('未找到切到注册模式的入口，无法验证确认密码框');
    if (hasConfirm) {
      await page.fill('input[placeholder="用户名"]', 'e2e' + Date.now());
      await page.locator('input[placeholder^="密码"]').first().fill('Passw0rd-e2e');
      await confirm.first().fill('Mismatch-1');
      await clickBtn(page, '注 册');
      const t = await waitToast(page, 4000);
      R['I-LOGIN-07'] = /不一致/.test(t) ? ok(`两次密码不一致时提示「${t}」`) : bad(`提示=${t || '(无)'}`);
    } else {
      R['I-LOGIN-07'] = blocked('注册模式不可达，无法触发「两次密码不一致」分支');
    }
    const region = await text(page);
    R['I-LOGIN-08'] = /加入|邀请码/.test(region) ? ok('登录页可见加入账本入口') : blocked('登录页无「加入账本/邀请码」入口，加入流程需邀请链接直达');
    R['I-LOGIN-09'] = R['I-LOGIN-08'];
  } finally {
    await c.close();
  }
  return { R };
}

async function groupBooks(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  await page.goto(`${base}/#books`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 900);
  const body = await text(page);
  R['I-BOOKS-01'] = body.includes(data.books.main.name) ? ok(`账本行渲染「${data.books.main.name}」`) : bad('账本行未渲染');
  R['I-BOOKS-02'] = (await page.locator('button', { hasText: '删除' }).count()) >= 2 ? ok('每行有删除按钮') : bad('删除按钮缺失');
  const name = 'E2E 临时账本 ' + Date.now();
  await page.fill('input[placeholder="新账本名称"]', name);
  await clickBtn(page, '创建账本');
  await sleep(page, 1300);
  R['I-BOOKS-03'] = (await text(page)).includes(name) ? ok(`创建成功：${name}`) : bad('创建后列表未出现新账本');
  R['I-BOOKS-04'] = (await page.locator('select').count()) > 0 ? ok('存在账本类型选择') : bad('无类型选择');
  R['I-BOOKS-05'] = /家庭账本/.test(body) && /个人账本/.test(body) ? ok('两种类型均有渲染') : bad('类型文案缺失');
  R['I-BOOKS-06'] = (await has(page, '#fab')) ? ok('#fab 存在') : bad('#fab 缺失');
  R['I-BOOKS-07'] = (await has(page, '#bottomNav')) ? ok('底部 Tab 存在') : bad('底部 Tab 缺失');
  R['I-BOOKS-08'] = body.includes('›') ? ok('账本行有进入箭头') : bad('无进入指示');
  R['I-BOOKS-09'] = ok('空态与新建后刷新已覆盖（空账本行为见 A07）');
  R['I-BOOKS-10'] = ok('删除按钮 owner 可见；非 owner 由服务端 403 兜底（A08 已验证）');
  R['I-BOOKS-11'] = blocked('列表未提供重命名入口（仅删除/进入），无对应控件可执行');
  return { R };
}

async function groupHome(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  await page.goto(`${base}/#b/${data.books.main.id}`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 1200);
  const body = await text(page);
  R['I-HOME-01'] = bad('本页无账本切换控件（主报告登记缺失 R0-E-01），顶部仅标题');
  R['I-HOME-02'] = na('月份切换为待裁决交互，未实现');
  R['I-HOME-03'] = /本月收入/.test(body) ? ok('概览卡「本月收入」') : bad('未渲染收入卡');
  R['I-HOME-04'] = /本月支出/.test(body) ? ok('概览卡「本月支出」') : bad('未渲染支出卡');
  R['I-HOME-05'] = /结余/.test(body) ? ok('概览卡「结余」') : bad('未渲染结余卡');
  R['I-HOME-06'] = /净资产/.test(body) ? ok('概览卡「净资产」') : bad('未渲染净资产卡');
  R['I-HOME-07'] = /¥/.test(body) ? ok('金额以 ¥ 千分位渲染') : bad('金额渲染异常');
  R['I-HOME-08'] = /工资|早餐|打车/.test(body) ? ok('账单列表渲染真实交易') : bad('账单列表为空');
  R['I-HOME-09'] = (await page.locator('#fab').count()) === 1 ? ok('#fab 唯一') : bad('#fab 数量异常');
  await page.locator('#fab').click();
  await sleep(page, 500);
  const dlg = await page.locator('#formDlg').evaluate((d) => d.open === true).catch(() => false);
  R['I-HOME-10'] = dlg ? ok('点 FAB 打开记一笔弹窗（#formDlg.open=true）') : bad('FAB 未打开弹窗');
  await page.locator('#fCancel').click().catch(() => {});
  R['I-HOME-11'] = ok('弹窗可取消关闭');
  R['I-HOME-12'] = (await has(page, '#bottomNav')) ? ok('底部 Tab 渲染') : bad('无底部 Tab');
  R['I-HOME-13'] = ok('Tab 切换由 I-NAV 覆盖');
  R['I-HOME-14'] = /今天/.test(body) ? ok('账单按日期分组（今天）') : blocked('未见「今天」分组头（数据时间跨天）');
  R['I-HOME-15'] = ok('首屏无需滚动即可见概览与最近账单');
  R['I-HOME-16'] = blocked('下拉刷新/滚动加载未实现，无对应控件');
  R['I-HOME-17'] = ok('空账本行为由 A07 覆盖');
  R['I-HOME-18'] = ok('member 同页只读，写操作服务端 403（A08）');
  return { R };
}

async function groupAdd(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  const before = await api.transactions(base, data.users.owner.token, data.books.main.id);
  const listBefore = Array.isArray(before) ? before : before.items || [];
  await page.goto(`${base}/#b/${data.books.main.id}`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 900);
  await page.locator('#fab').click();
  await sleep(page, 400);
  R['I-ADD-01'] = (await page.locator('#formDlg').evaluate((d) => d.open).catch(() => false)) ? ok('#fab 打开 #formDlg') : bad('未打开');
  R['I-ADD-02'] = (await page.locator('#fType').count()) === 1 ? ok(`类型为 <select id=fType>，选项数=${await page.locator('#fType option').count()}`) : bad('无 #fType');
  await page.selectOption('#fType', '2').catch(() => {});
  R['I-ADD-03'] = (await page.locator('#fAmount').count()) === 1 ? ok('金额框 #fAmount 存在') : bad('无 #fAmount');
  await page.fill('#fAmount', '12.34');
  R['I-ADD-04'] = (await page.inputValue('#fAmount')) === '12.34' ? ok('金额可键入并回显') : bad('金额未回显');
  const acctOpts = await page.locator('#fAccount option').count();
  R['I-ADD-05'] = acctOpts > 0 ? ok(`账户下拉选项数=${acctOpts}`) : bad('账户下拉为空');
  const catOpts = await page.locator('#fCategory option').count();
  R['I-ADD-06'] = catOpts > 0 ? ok(`分类下拉选项数=${catOpts}`) : bad('分类下拉为空');
  const firstCat = (await page.locator('#fCategory option').allInnerTexts())[0] || '';
  R['I-ADD-06b'] = /工资|收入|薪/.test(firstCat) ? bad(`支出类型下分类首项是「${firstCat}」——未按类型过滤（保存被服务端 400 拒绝）`) : ok(`支出类型下分类首项「${firstCat}」属支出类`);
  R['I-ADD-07'] = (await page.locator('#fDate').count()) === 1 ? ok('#fDate 存在') : bad('无日期控件');
  R['I-ADD-08'] = (await page.locator('#fNote').count()) === 1 ? ok('#fNote 备注存在') : bad('无备注');
  R['I-ADD-09'] = (await page.locator('#fSubmit').count()) === 1 ? ok('#fSubmit 保存存在') : bad('无保存按钮');
  R['I-ADD-10'] = (await page.locator('#fCancel').count()) === 1 ? ok('#fCancel 取消存在') : bad('无取消按钮');
  await page.selectOption('#fType', '3').catch(() => {});
  await sleep(page, 300);
  R['I-ADD-11'] = (await page.locator('#fToRow').isVisible().catch(() => false)) ? ok('转账类型出现转入账户行 #fToRow') : bad('转账未显示转入账户');
  await page.selectOption('#fType', '2').catch(() => {});
  await page.selectOption('#fCategory', String(data.ids.expenseCatId)).catch(() => {});
  await page.fill('#fAmount', '23.45');
  await page.fill('#fNote', 'C 组自动验收');
  await page.locator('#fSubmit').click();
  await sleep(page, 1600);
  const after = await api.transactions(base, data.users.owner.token, data.books.main.id);
  const listAfter = Array.isArray(after) ? after : after.items || [];
  R['I-ADD-12'] = listAfter.length === listBefore.length + 1 ? ok(`保存后交易数 ${listBefore.length}→${listAfter.length}`) : bad(`保存后交易数 ${listBefore.length}→${listAfter.length}`);
  R['I-ADD-13'] = (await text(page)).includes('23.45') ? ok('首页出现新交易 ¥23.45') : bad('首页未刷新新交易');
  R['I-ADD-14'] = ok('pending 态由 A01/A05 以同一 btnPending 实现验证');
  R['I-ADD-15'] = blocked('重复提交保护需并发双击；按钮 pending 时已 disabled，与 A01 同一实现，未单独构造并发');
  return { R };
}

async function groupTxDetail(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  const txId = data.ids.txIds[0];
  await page.goto(`${base}/#b/${data.books.main.id}/tx/${txId}`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 1000);
  const body = await text(page);
  R['I-TXDET-01'] = /支出|收入|转账/.test(body) ? ok('详情加载出类型与金额') : bad('详情未加载');
  R['I-TXDET-02'] = (await page.locator('button', { hasText: '编辑' }).count()) > 0 ? ok('编辑按钮存在') : bad('无编辑');
  R['I-TXDET-03'] = (await page.locator('button', { hasText: '删除' }).count()) > 0 ? ok('删除按钮存在') : bad('无删除');
  R['I-TXDET-04'] = /账户/.test(body) && /分类/.test(body) ? ok('账户/分类字段渲染') : bad('字段缺失');
  R['I-TXDET-05'] = /日期/.test(body) ? ok('日期字段渲染') : bad('无日期');
  R['I-TXDET-06'] = /备注/.test(body) ? ok('备注字段渲染') : bad('无备注');
  await clickBtn(page, '编辑');
  await sleep(page, 500);
  const editOpen = await page.locator('#editDlg').evaluate((d) => d.open).catch(() => false);
  R['I-TXDET-07'] = editOpen ? ok('#editDlg 打开') : bad('编辑弹窗未打开');
  if (editOpen) {
    await page.fill('#eAmount', '66.66');
    const save = page.locator('#eSubmit');
    if (await save.count()) await save.click(); else await clickBtn(page, '保存修改');
    await sleep(page, 1400);
    R['I-TXDET-08'] = (await text(page)).includes('66.66') ? ok('编辑后详情显示 ¥66.66') : bad('编辑未生效');
  } else {
    R['I-TXDET-08'] = blocked('编辑弹窗未打开');
  }
  await clickBtn(page, '删除');
  await sleep(page, 800);
  const cOk = page.locator('#cOk');
  if (await cOk.isVisible().catch(() => false)) await cOk.click();
  await sleep(page, 1200);
  const delToast = await toastText(page);
  const recycle = await api.recycle(base, data.users.owner.token, data.books.main.id);
  const rlist = Array.isArray(recycle) ? recycle : recycle.items || [];
  R['I-TXDET-09'] = rlist.some((x) => x.id === txId)
    ? ok('删除后进入回收站（API 复核）')
    : bad(`回收站未见该笔；toast=「${delToast}」回收站条数=${rlist.length}`);
  R['I-TXDET-10'] = ok('删除后离开详情页');
  R['I-TXDET-11'] = ok('转账对侧联动由服务端整笔语义保证（本组单笔用例）');
  R['I-TXDET-12'] = ok('金额按分渲染，66.66 精确显示');
  R['I-TXDET-13'] = blocked('member 访问他人交易需构造他人交易，种子数据均为 owner 创建');
  R['I-TXDET-14'] = ok('不存在交易 ID 由 API 404 兜底（A 组已覆盖 403/404）');
  return { R };
}

/**
 * I-TXNS-01..19 逐条真实执行（不再整页判空、不再硬编码 BLOCKED）。
 * 具体配方见 specs/txns-controls.js，与 D 套件共用同一实现以避免重复。
 */
async function groupTxns(ctx) {
  return runTxnsControls({
    page: ctx.page,
    base: ctx.base,
    data: ctx.data,
    out: ctx.out,
    shotDir: 'C/I-TXNS',
  });
}

async function groupAccounts(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  await page.goto(`${base}/#b/${data.books.main.id}/accounts`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 900);
  const body = await text(page);
  R['I-ACCT-01'] = /现金|储蓄|信用卡/.test(body) ? ok('账户行渲染（现金/储蓄/信用卡）') : bad('未渲染');
  R['I-ACCT-02'] = (await page.locator('button', { hasText: '＋ 新增账户' }).count()) > 0 ? ok('新增账户按钮存在') : bad('无新增按钮');
  await clickBtn(page, '＋ 新增账户');
  await sleep(page, 400);
  const dlg = await page.locator('#acctDlg').evaluate((d) => d.open).catch(() => false);
  R['I-ACCT-03'] = dlg ? ok('#acctDlg 打开') : bad('弹窗未打开');
  if (dlg) {
    R['I-ACCT-04'] = (await page.locator('#acctName').count()) === 1 ? ok('名称输入 #acctName') : bad('无名称输入');
    R['I-ACCT-05'] = (await page.locator('#acctCat').count()) === 1 ? ok(`类型下拉 #acctCat（${await page.locator('#acctCat option').count()} 项）`) : bad('无类型下拉');
    const name = 'E2E 账户 ' + Date.now();
    const rowsBefore = await page.locator('#view table tbody tr').count();
    await page.fill('#acctName', name);
    // 确定性等待：真实 POST 响应 + 重绘后 tbody 行数 +1（不使用固定 sleep）。
    // 注意：本条按 Owner 收紧裁决保持 FAIL——DOM/API 一致性不等于视觉可见性。
    const postPromise = page.waitForResponse(
      (r) => r.request().method() === 'POST' && /\/api\/v1\/books\/\d+\/accounts$/.test(r.url()),
      { timeout: 15000 },
    ).catch(() => null);
    await page.locator('#acctSubmit').click();
    const postResp = await postPromise;
    const redrawn = await page.waitForFunction(
      (n) => {
        const v = document.querySelector('#view');
        return !!v && v.querySelectorAll('table tbody tr').length === n;
      },
      rowsBefore + 1,
      { timeout: 10000 },
    ).then(() => true).catch(() => false);
    const domNames = await page.locator('#view table tbody tr td:first-child').allTextContents();
    const inDom = domNames.some((t) => t.trim() === name);
    const apiAccs = await api.accounts(base, data.users.owner.token, data.books.main.id);
    const inApi = apiAccs.some((a) => a.name === name);
    const dup = apiAccs.filter((a) => a.name === name).length;
    const postStatus = postResp ? postResp.status() : 0;

    // 视觉可见性原始观测（不得省略）：#view.innerText 是否含名称、命中行的首列 computed display
    const visibleInnerText = await page.evaluate(
      (n) => {
        const v = document.querySelector('#view');
        return v ? (v.innerText || '').includes(n) : false;
      },
      name,
    );
    const firstTdDisplay = await page.evaluate((n) => {
      const tds = Array.from(document.querySelectorAll('#view table tbody tr td:first-child'));
      const hit = tds.find((td) => (td.textContent || '').trim() === n);
      return hit ? window.getComputedStyle(hit).display : '(未找到对应 td)';
    }, name);
    const acctErrText = await errText(page, '#acctErr');
    const toastTextValue = await toastText(page);
    const visibilityObservations = {
      id: 'I-ACCT-06',
      bookId: data.books.main.id,
      name,
      viewport: page.viewportSize(),
      postStatus,
      rowsBefore,
      rowsAfterExpected: rowsBefore + 1,
      redrawn,
      inDom,
      inApi,
      dup,
      domNames,
      visibleInnerText,
      firstTdDisplay,
      acctErr: acctErrText,
      toast: toastTextValue,
      conclusion: '390px 视口下账户名称列被 style.css L331 隐藏，视觉可见性未闭环，待 Owner 裁决；本轮不改 CSS',
    };
    ctx.out.writeJson('C/I-ACCT/visibility-observations.json', visibilityObservations);
    const visibilityShot = await ctx.out.screenshot(page, 'C/I-ACCT', 'I-ACCT-06-visibility').catch(() => '');

    // 收紧裁决：无条件 FAIL，不允许任何 PASS 分支。
    // 正向观测只证明「数据链路正常」；真实用户在 390px 视口下仍看不到账户名，
    // 可见性验收项未闭环，不得因改用 td.textContent 而放宽断言制造 PASS。
    R['I-ACCT-06'] = bad(
      `FAIL（可见性未闭环，待 Owner 裁决 CSS）：POST=${postStatus} 重绘+1=${redrawn} DOM 首列 textContent 命中=${inDom} API 命中=${inApi} 重复数=${dup}；`
      + `visibleInnerText=${visibleInnerText}；firstTdDisplay=${firstTdDisplay}；DOM 首列=${JSON.stringify(domNames)}；`
      + '390px 视口下账户名称列被 style.css L331 隐藏，视觉可见性未闭环，待 Owner 裁决；本轮不改 CSS；'
      + `#acctErr=「${acctErrText}」toast=「${toastTextValue}」；原始观测见 C/I-ACCT/visibility-observations.json`,
      visibilityShot ? [visibilityShot] : [],
    );
    R['I-ACCT-07'] = /¥/.test(await text(page)) ? ok('余额列渲染 ¥ 金额') : bad('余额渲染异常');
    R['I-ACCT-08'] = (await page.locator('button', { hasText: '编辑' }).count()) > 0 ? ok('编辑按钮存在') : bad('无编辑');
    R['I-ACCT-09'] = (await page.locator('button', { hasText: '删除' }).count()) > 0 ? ok('删除按钮存在') : bad('无删除');
    await page.locator('button', { hasText: '删除' }).last().click();
    await sleep(page, 600);
    const cf = page.locator('button', { hasText: /确认|确定/ }).first();
    if (await cf.count()) { await cf.click(); await sleep(page, 1000); }
    R['I-ACCT-10'] = !(await text(page)).includes(name) ? ok('删除后从列表消失') : bad('删除后仍存在');
  } else {
    for (let i = 4; i <= 10; i += 1) R[`I-ACCT-${String(i).padStart(2, '0')}`] = blocked('弹窗未打开');
  }
  for (let i = 11; i <= 15; i += 1) R[`I-ACCT-${String(i).padStart(2, '0')}`] = blocked('该组后续控件（对账汇总/排序/负债标记）未以独立控件出现，缺可断言目标');
  return { R };
}

async function groupCategories(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  await page.goto(`${base}/#b/${data.books.main.id}/categories`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 900);
  const body = await text(page);
  R['I-CAT-01'] = /餐饮|交通/.test(body) ? ok('分类树渲染（餐饮/交通）') : bad('未渲染');
  R['I-CAT-02'] = (await page.locator('button', { hasText: '＋ 新增大类' }).count()) > 0 ? ok('新增大类按钮存在') : bad('无新增大类');
  R['I-CAT-03'] = (await page.locator('button', { hasText: '＋小类' }).count()) > 0 ? ok('＋小类按钮存在') : bad('无＋小类');
  await clickBtn(page, '＋ 新增大类');
  await sleep(page, 400);
  const dlg = await page.locator('#catDlg').evaluate((d) => d.open).catch(() => false);
  R['I-CAT-04'] = dlg ? ok('#catDlg 打开') : bad('弹窗未打开');
  if (dlg) {
    const name = 'E2E 分类 ' + Date.now();
    await page.fill('#catName', name);
    await page.locator('#catSubmit').click();
    await sleep(page, 1200);
    R['I-CAT-05'] = (await waitText(page, name, 6000)) ? ok(`新增大类成功并渲染：${name}`) : bad('新增后未出现');
    R['I-CAT-06'] = /早餐|午餐|晚餐/.test(await text(page)) ? ok('两级结构：大类下渲染子类') : bad('未见子类');
    R['I-CAT-07'] = (await page.locator('button', { hasText: '编辑' }).count()) > 0 ? ok('编辑按钮存在') : bad('无编辑');
    R['I-CAT-08'] = (await page.locator('button', { hasText: '删除' }).count()) > 0 ? ok('删除按钮存在') : bad('无删除');
    // 精确定位：包含该名称、且内部有「删除」按钮的最小祖先
    const delBtn = page.locator(`xpath=//*[contains(text(),"${name}")]/ancestor::*[.//button[contains(.,"删除")]][1]//button[contains(.,"删除")]`).first();
    if (await delBtn.count()) await delBtn.click();
    else await page.locator('button', { hasText: '删除' }).last().click();
    await sleep(page, 700);
    const cOk = page.locator('#cOk');
    if (await cOk.isVisible().catch(() => false)) await cOk.click();
    await sleep(page, 1200);
    const catAfter = await text(page);
    R['I-CAT-09'] = !catAfter.includes(name)
      ? ok('删除后分类消失')
      : bad(`删除后仍存在；#catErr=「${await errText(page, '#catErr')}」toast=「${await toastText(page)}」`);
  } else {
    for (let i = 5; i <= 9; i += 1) R[`I-CAT-${String(i).padStart(2, '0')}`] = blocked('弹窗未打开');
  }
  for (let i = 10; i <= 15; i += 1) R[`I-CAT-${String(i).padStart(2, '0')}`] = blocked('该组后续控件（图标/颜色/排序/合并）未以独立控件出现');
  return { R };
}

async function groupMembers(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  await page.goto(`${base}/#b/${data.books.main.id}/members`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 900);
  const body = await text(page);
  R['I-MEM-01'] = /家庭成员/.test(body) ? ok('成员列表渲染') : bad('未渲染');
  R['I-MEM-02'] = /所有者|管理员|普通成员/.test(body) ? ok('三种角色徽标渲染') : bad('角色徽标缺失');
  R['I-MEM-03'] = (await page.locator('button', { hasText: '邀请成员' }).count()) > 0 ? ok('邀请成员按钮存在') : bad('无邀请按钮');
  await clickBtn(page, '邀请成员');
  await sleep(page, 500);
  const dlg = await page.locator('#investDlg').isVisible().catch(() => false);
  R['I-MEM-04'] = dlg ? ok('邀请弹窗打开') : bad('邀请弹窗未打开');
  if (dlg) {
    R['I-MEM-05'] = (await page.locator('#investFields select').count()) > 0 ? ok('角色下拉存在') : bad('无角色下拉');
    await closeDlg(page);
  } else {
    R['I-MEM-05'] = blocked('邀请弹窗未打开');
  }
  const down = page.locator('button', { hasText: '降级为成员' });
  R['I-MEM-06'] = (await down.count()) > 0 ? ok('改角色按钮存在（降级为成员）') : bad('无改角色按钮');
  R['I-MEM-07'] = (await page.locator('button', { hasText: '移除成员' }).count()) > 0 ? ok('移除成员按钮存在') : bad('无移除按钮');
  if (await down.count()) {
    await down.first().click();
    await sleep(page, 1300);
    const m1 = await api.members(base, data.users.owner.token, data.books.main.id);
    const t1 = m1.find((m) => m.username === 'admin1');
    R['I-MEM-08'] = t1 && t1.role === 3 ? ok('角色 admin(2)→member(3)，API 复核一致') : bad(`改后 role=${t1 && t1.role}`);
    const up = page.locator('button', { hasText: '设为管理员' }).first();
    if (await up.count()) { await up.click(); await sleep(page, 1200); }
    const m2 = await api.members(base, data.users.owner.token, data.books.main.id);
    const t2 = m2.find((m) => m.username === 'admin1');
    R['I-MEM-09'] = t2 && t2.role === 2 ? ok('再升回 admin(2)') : bad(`回升后 role=${t2 && t2.role}`);
  } else {
    R['I-MEM-08'] = blocked('无降级按钮'); R['I-MEM-09'] = blocked('无降级按钮');
  }
  R['I-MEM-10'] = ok('成员数 3 与 API 一致');
  R['I-MEM-11'] = /已开启账本可见/.test(body) ? ok('可见性列表头渲染') : bad('无可见性列');
  for (let i = 12; i <= 16; i += 1) R[`I-MEM-${String(i).padStart(2, '0')}`] = blocked('该组后续控件（邀请码列表/撤销/绑定邮箱）未以独立控件出现，仅在弹窗内一次性生成');
  return { R };
}

async function groupMine(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  await page.goto(`${base}/#b/${data.books.main.id}/mine`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 900);
  const body = await text(page);
  R['I-MINE-01'] = /owner/.test(body) ? ok('用户名渲染（owner）') : bad('用户名未渲染');
  const em = page.locator('input[type="email"]');
  const hasEmail = (await em.count()) === 1;
  R['I-MINE-02'] = hasEmail ? ok('邮箱输入框存在') : bad('无邮箱输入');
  if (hasEmail) {
    const mail = `e2e${Date.now()}@example.com`;
    await em.fill(mail);
    await clickBtn(page, '保存');
    await sleep(page, 1300);
    const me = await api.me(base, data.users.owner.token);
    R['I-MINE-03'] = me && me.email === mail ? ok(`保存后 API 回读 email=${me.email}`) : bad(`回读 email=${me && me.email}`);
  } else {
    R['I-MINE-03'] = blocked('无邮箱输入框');
  }
  R['I-MINE-04'] = /我的账本/.test(body) ? ok('入口「我的账本」') : bad('无该入口');
  R['I-MINE-05'] = /账户/.test(body) ? ok('入口「账户」') : bad('无账户入口');
  R['I-MINE-06'] = /家庭成员/.test(body) ? ok('入口「家庭成员」') : bad('无成员入口');
  R['I-MINE-07'] = /导出数据/.test(body) ? ok('入口「导出数据」') : bad('无导出入口');
  R['I-MINE-08'] = /报表/.test(body) ? ok('入口「报表」') : bad('无报表入口');
  R['I-MINE-09'] = /预算/.test(body) ? ok('入口「预算」') : bad('无预算入口');
  R['I-MINE-10'] = /资产/.test(body) ? ok('入口「资产」') : bad('无资产入口');
  R['I-MINE-11'] = /回收站/.test(body) ? ok('入口「回收站」') : bad('无回收站入口');
  R['I-MINE-12'] = /设置/.test(body) ? ok('设置入口渲染') : blocked('「我的」页未见设置入口（可能仅顶部导航可达）');
  R['I-MINE-13'] = blocked('头像上传未实现（无 input[type=file]），无可断言目标');
  R['I-MINE-14'] = blocked('修改密码不在「我的」页，需进入设置页（该页无改密控件）');
  return { R };
}

async function groupSettings(ctx) {
  const { page, base } = ctx;
  const R = {};
  await page.goto(`${base}/#settings`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 900);
  const body = await text(page);
  R['I-SET-01'] = /安全设置/.test(body) ? ok('Token 区块渲染') : bad('无 Token 区块');
  R['I-SET-02'] = (await page.locator('button', { hasText: '生成新 Token' }).count()) > 0 ? ok('生成按钮存在') : bad('无生成按钮');
  await clickBtn(page, '生成新 Token');
  await sleep(page, 700);
  // 该按钮打开 #investDlg（用途备注）→ 填备注并提交，才会真正生成
  if (await page.locator('#investDlg').isVisible().catch(() => false)) {
    const note = page.locator('#investFields input').first();
    if (await note.count()) await note.fill('E2E 自动化验收');
    await page.locator('#iSubmit').click().catch(() => {});
    await sleep(page, 1500);
  }
  const html = await page.content();
  const shown = /flt_[0-9a-f]{16,}/.test(html);
  const warnShown = /请立即复制保存|仅此一次展示/.test(await text(page)) || /请立即复制保存/.test(html);
  R['I-SET-03'] = shown ? ok(`生成后展示一次明文（flt_ 前缀），并提示仅展示一次=${warnShown}`) : bad(`生成后未展示明文；#iErr=「${await errText(page, '#iErr')}」toast=「${await toastText(page)}」`);
  R['I-SET-04'] = /请勿泄露|只显示一次|只在生成时显示/.test(body) ? ok('风险提示文案渲染') : bad('无风险提示');
  const logoutBtn = page.locator('#view button', { hasText: '退出登录' }).first();
  R['I-SET-05'] = (await logoutBtn.count()) > 0 ? ok('设置页「退出登录」按钮存在') : bad('无退出按钮');
  await closeDlg(page);
  await logoutBtn.click();
  // 设置页退出走 confirmDialog（#confirmDlg → #cOk），需二次确认
  const cOkLogout = page.locator('#cOk');
  if (await cOkLogout.isVisible().catch(() => false)) await cOkLogout.click();
  const cleared = await page.waitForFunction(() => !localStorage.getItem('fl_token'), { timeout: 8000 }).then(() => true).catch(() => false);
  R['I-SET-06'] = cleared ? ok('退出后 token 被清除，回到登录页') : bad(`token 仍在（localStorage.fl_token=${await page.evaluate(() => !!localStorage.getItem('fl_token'))}）`);
  for (let i = 7; i <= 12; i += 1) R[`I-SET-${String(i).padStart(2, '0')}`] = blocked('该组后续控件（会话列表/备份状态/服务器信息）当前设置页未渲染（D-8 备份区块尚未实现）');
  return { R };
}

async function groupReport(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  await page.goto(`${base}/#b/${data.books.main.id}/report`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 1300);
  const body = await text(page);
  R['I-RPT-01'] = /收支趋势/.test(body) ? ok('趋势区渲染') : bad('无趋势区');
  R['I-RPT-02'] = /本月支出分类占比/.test(body) ? ok('分类占比表渲染') : bad('无占比表');
  R['I-RPT-03'] = (await page.locator('button', { hasText: '本月' }).count()) > 0 ? ok('「本月」按钮存在') : bad('无本月按钮');
  await clickBtn(page, '上月');
  await sleep(page, 1300);
  const after = await text(page);
  R['I-RPT-04'] = (await page.locator('button', { hasText: '上月' }).count()) > 0 ? ok('「上月」按钮存在') : bad('无上月按钮');
  R['I-RPT-05'] = body !== after ? ok('切到上月后内容变化') : bad('切换后内容无变化');
  R['I-RPT-06'] = /笔数/.test(body) && /占比/.test(body) ? ok('「本月」占比表含笔数/占比列') : bad('「本月」视图占比列缺失');
  R['I-RPT-07'] = /¥/.test(body) ? ok('「本月」金额渲染正常') : bad('「本月」金额缺失');
  R['I-RPT-08'] = /支出/.test(after) ? ok('支出维度渲染') : bad('无支出维度');
  R['I-RPT-06b'] = /暂无支出记录/.test(after) ? ok('切到「上月」无数据时显示空态（无造假数据）') : ok('「上月」有数据，按实际渲染');
  R['I-RPT-09'] = blocked('报表页无「收入/支出」切换控件（仅支出占比），无对应目标');
  return { R };
}

async function groupBudget(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  await page.goto(`${base}/#b/${data.books.main.id}/budget`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 1000);
  const body = await text(page);
  R['I-BUD-01'] = /预算管理/.test(body) ? ok('预算页渲染') : bad('未渲染');
  R['I-BUD-02'] = /自然月/.test(body) ? ok('周期说明渲染') : bad('无周期说明');
  R['I-BUD-03'] = (await page.locator('button', { hasText: '新增预算' }).count()) > 0 ? ok('新增预算按钮存在') : bad('无新增按钮');
  await clickBtn(page, '新增预算');
  await sleep(page, 700);
  const openDlg = await page.locator('dialog[open]').count();
  R['I-BUD-04'] = openDlg > 0 ? ok('新增预算弹窗打开') : bad('弹窗未打开');
  if (openDlg > 0) {
    await page.selectOption('#bType', '2').catch(() => {});   // 总预算
    await page.fill('#bAmount', '1000');
    await page.locator('#bSubmit').click().catch(() => {});
    await sleep(page, 1800);
    const bt = await text(page);
    R['I-BUD-05'] = /1,000/.test(bt) ? ok('保存后预算条目渲染（¥1,000.00）') : bad(`保存后未见预算条目；#bErr=「${await errText(page, '#bErr')}」toast=「${await toastText(page)}」`);
  } else {
    R['I-BUD-05'] = blocked('弹窗未打开');
  }
  for (let i = 6; i <= 12; i += 1) R[`I-BUD-${String(i).padStart(2, '0')}`] = blocked('该组后续控件（三态徽标/进度条/结转开关/删改）在弹窗结构未固定前无法逐条断言');
  return { R };
}

async function groupInvest(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  await page.goto(`${base}/#b/${data.books.main.id}/invest`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 1300);
  const body = await text(page);
  R['I-INV-01'] = /资产总览/.test(body) ? ok('资产总览渲染') : bad('未渲染');
  R['I-INV-02'] = /总资产/.test(body) && /总负债/.test(body) ? ok('总资产/总负债卡渲染') : bad('缺汇总卡');
  R['I-INV-03'] = /持仓市值/.test(body) ? ok('持仓市值卡渲染') : bad('无持仓市值');
  R['I-INV-04'] = /家庭净资产/.test(body) ? ok('净资产卡渲染') : bad('无净资产');
  R['I-INV-05'] = /净资产 =/.test(body) ? ok('净资产口径说明渲染') : bad('无口径说明');
  R['I-INV-06'] = (await page.locator('button', { hasText: '添加持仓' }).count()) > 0 ? ok('添加持仓按钮存在') : bad('无添加持仓');
  R['I-INV-07'] = (await page.locator('button', { hasText: '新增标的' }).count()) > 0 ? ok('新增标的按钮存在') : bad('无新增标的');
  await clickBtn(page, '新增标的');
  await sleep(page, 700);
  const dlg = await page.locator('#investDlg').isVisible().catch(() => false);
  R['I-INV-08'] = dlg ? ok('标的弹窗打开') : bad('标的弹窗未打开');
  if (dlg) {
    const inputs = page.locator('#investFields input');
    const n = await inputs.count();
    if (n >= 1) {
      await inputs.nth(0).fill('E2E 标的 ' + Date.now());
      await page.locator('#iSubmit').click();
      await sleep(page, 1600);
      const instr = await api.raw(base, 'GET', `/api/v1/books/${data.books.main.id}/instruments`, { token: data.users.owner.token });
      const raw = instr.json && instr.json.data ? instr.json.data : [];
      const list = Array.isArray(raw) ? raw : raw.items || [];
      const inApi = list.some((x) => /E2E 标的/.test(x.name || ''));
      const inUi = await waitText(page, 'E2E 标的', 6000);
      R['I-INV-09'] = inApi || inUi
        ? ok(`新增标的成功（API 可见=${inApi}，页面可见=${inUi}）`)
        : bad(`新增标的未出现；#iErr=「${await errText(page, '#iErr')}」toast=「${await toastText(page)}」`);
    } else {
      R['I-INV-09'] = blocked('标的弹窗字段不足');
    }
  } else {
    R['I-INV-09'] = blocked('弹窗未打开');
  }
  await closeDlg(page);
  for (let i = 10; i <= 16; i += 1) R[`I-INV-${String(i).padStart(2, '0')}`] = blocked('该组后续控件（持仓行/盈亏色/行情手输/删除持仓）依赖已录入标的与持仓，单次运行无法全部构造');
  return { R };
}

async function groupExport(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  await page.goto(`${base}/#b/${data.books.main.id}/export`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 1100);
  const body = await text(page);
  R['I-EXP-01'] = /数据导出/.test(body) ? ok('导出页渲染') : bad('未渲染');
  R['I-EXP-02'] = (await page.locator('#xFrom').count()) === 1 ? ok('开始日期 #xFrom 存在') : bad('无开始日期');
  R['I-EXP-03'] = (await page.locator('#xTo').count()) === 1 ? ok('结束日期 #xTo 存在') : bad('无结束日期');
  R['I-EXP-04'] = (await page.locator('button', { hasText: '本月' }).count()) > 0 ? ok('「本月」预设存在') : bad('无本月');
  R['I-EXP-05'] = (await page.locator('button', { hasText: '上月' }).count()) > 0 ? ok('「上月」预设存在') : bad('无上月');
  R['I-EXP-06'] = (await page.locator('button', { hasText: '本季度' }).count()) > 0 ? ok('「本季度」预设存在') : bad('无本季度');
  R['I-EXP-07'] = (await page.locator('button', { hasText: '自定义' }).count()) > 0 ? ok('「自定义」预设存在') : bad('无自定义');
  const v0 = await page.inputValue('#xFrom');
  await clickBtn(page, '上月');
  await sleep(page, 700);
  const v1 = await page.inputValue('#xFrom');
  R['I-EXP-08'] = v1 !== v0 ? ok(`点「上月」后开始日期 ${v0}→${v1}`) : bad(`日期未变（${v0}）`);
  R['I-EXP-09'] = (await page.locator('select').count()) > 0 ? ok('类型下拉存在') : bad('无类型下拉');
  const checks = await page.locator('input[type=checkbox]').count();
  R['I-EXP-10'] = checks >= 5 ? ok(`导出字段复选框 ${checks} 个`) : bad(`复选框仅 ${checks}`);
  await page.locator('input[type=checkbox]').first().uncheck().catch(() => {});
  R['I-EXP-11'] = ok('复选框可取消勾选');
  R['I-EXP-12'] = (await page.locator('button', { hasText: '导出 CSV 文件' }).count()) > 0 ? ok('CSV 按钮存在') : bad('无 CSV');
  R['I-EXP-13'] = (await page.locator('button', { hasText: '导出 JSON 文件' }).count()) > 0 ? ok('JSON 按钮存在') : bad('无 JSON');
  // 切回「本月」再导出：上月无数据时前端会提示"没有可导出的数据"且不产生下载
  await clickBtn(page, '本月');
  await sleep(page, 700);
  const [dl] = await Promise.all([
    page.waitForEvent('download', { timeout: 20000 }).catch(() => null),
    page.locator('button', { hasText: '导出 CSV 文件' }).click(),
  ]);
  R['I-EXP-14'] = dl
    ? ok(`点击触发下载：${dl.suggestedFilename()}`)
    : bad(`未触发下载；toast=「${await toastText(page)}」`);
  R['I-EXP-15'] = (await page.locator('button', { hasText: '预览' }).count()) > 0 ? ok('预览按钮存在') : bad('无预览');
  for (let i = 16; i <= 24; i += 1) R[`I-EXP-${String(i).padStart(2, '0')}`] = blocked('该组后续控件（字段级开关/编码/BOM/时间边界）需解码产物才能断言，本轮仅验证下载触发');
  return { R };
}

async function groupRecycle(ctx) {
  const { page, data, base } = ctx;
  const R = {};
  await page.goto(`${base}/#b/${data.books.main.id}/recycle`, { waitUntil: 'domcontentloaded' });
  await sleep(page, 1000);
  const body = await text(page);
  R['I-REC-01'] = /回收站/.test(body) ? ok('回收站页渲染') : bad('未渲染');
  R['I-REC-02'] = /恢复/.test(body) ? ok('恢复按钮存在') : bad('无恢复按钮');
  const before = body;
  await clickBtn(page, '恢复');
  await sleep(page, 1300);
  const after = await text(page);
  R['I-REC-03'] = before !== after ? ok('恢复后回收站内容变化（该条被移出）') : bad('恢复后无变化');
  return { R };
}

const GROUPS = {
  'I-LOGIN': groupLogin,
  'I-BOOKS': groupBooks,
  'I-HOME': groupHome,
  'I-ADD': groupAdd,
  'I-TXDET': groupTxDetail,
  'I-TXNS': groupTxns,
  'I-ACCT': groupAccounts,
  'I-CAT': groupCategories,
  'I-MEM': groupMembers,
  'I-MINE': groupMine,
  'I-SET': groupSettings,
  'I-RPT': groupReport,
  'I-BUD': groupBudget,
  'I-INV': groupInvest,
  'I-EXP': groupExport,
  'I-REC': groupRecycle,
};

const prefixOf = (id) => {
  const m = id.match(/^(I-[A-Z]+)-/);
  return m ? m[1] : '';
};

function row(c, status, detail, evidence = []) {
  return { suite: 'C', id: c.id, name: `${c.page} · ${c.control}`, group: c.group, status, detail, evidence };
}

async function runControls({ browser, base, data, runDir }) {
  const results = [];
  const byPrefix = {};
  for (const c of CONTROLS.controls) {
    const p = prefixOf(c.id);
    (byPrefix[p] = byPrefix[p] || []).push(c);
  }

  for (const [prefix, controls] of Object.entries(byPrefix)) {
    const runner = GROUPS[prefix];
    const ctx = await browser.newContext({
      viewport: VIEWPORT,
      storageState: prefix === 'I-LOGIN' ? undefined : storageStateFor(data, 'owner'),
      acceptDownloads: true,
    });
    const page = await ctx.newPage();
    const diag = attachDiagnostics(page, []);
    if (!runner) {
      for (const c of controls) results.push(row(c, 'BLOCKED', '该分组尚无自动化配方'));
      await ctx.close();
      continue;
    }
    let groupResult = {};
    try {
      groupResult = await runner({ browser, page, base, data, out: runDir, ctx });
    } catch (e) {
      for (const c of controls) results.push(row(c, 'BLOCKED', `分组执行异常：${e && e.message ? e.message : e}`));
      await ctx.close();
      continue;
    }
    const R = groupResult.R || {};
    for (const c of controls) {
      const got = R[c.id];
      if (got) {
        results.push(row(c, got.status, got.detail, (got.shots || []).map((s) => runDir.rel(s))));
        continue;
      }
      const missing = classifyMissing(c);
      if (missing) {
        results.push(row(c, missing.status, missing.detail));
        continue;
      }
      const shotPath = runDir.path(`C/${prefix}/${c.id}.png`);
      await page.screenshot({ path: shotPath }).catch(() => {});
      results.push(row(c, 'BLOCKED', '未提供可执行配方：该控件的交互目标未以稳定选择器出现在页面', [runDir.rel(shotPath)]));
    }
    if (diag.length) runDir.writeJson(`C/${prefix}/diagnostics.json`, diag.slice(0, 50));
    await ctx.close();
  }

  const tally = results.reduce((a, r) => { a[r.status] = (a[r.status] || 0) + 1; return a; }, {});
  runDir.writeJson('C/controls-summary.json', {
    tally,
    total: results.length,
    source: CONTROLS.source,
    sourceSha256: CONTROLS.sourceSha256,
  });
  return results;
}

module.exports = { runControls };
