#!/usr/bin/env node
'use strict';
/**
 * R0 包 B · 同数据集种子化：把原型内置演示数据集（tools/r0/prototype-dataset.json）
 * 通过公开 API 原样种进一个**空的隔离账本**，使两侧截图可以逐像素对照。
 *
 * 设计约束：
 *   1. 只调用公开 HTTP API（不直连数据库、不写 SQL）；
 *   2. token 只在进程内存与子进程 env 中，不打印、不落盘；
 *   3. 余额差异如实记录：实现侧账户初始余额恒为 0（期初余额 D-4 未实现），
 *      因此账户余额由交易派生；为让两侧显示同一组数字，脚本把实现侧派生余额
 *      回填进原型侧 localStorage 注入包（见 protoInjection）；
 *   4. 实现侧不存在的字段（如 merchant 商户）不伪造，登记到 fieldDiffs。
 *
 * 分类解析优先级（**与 tools/r0/lib/resolve-category.js 头部注释保持一致，改一处必须同步另一处**）：
 *   1. 精确 ID 命中（id）；
 *   2. 规范化 ID：补 `cat_` 前缀后命中（id-normalized）；
 *   3. 展示名精确命中子类路径「父-子」（name-path，仅名称唯一时）；
 *   4. 同父类下「唯一前缀」匹配（name-prefix，>1 候选即失败关闭）；
 *   5. 展示名本身就是大类名 → 用大类（parent-fallback，不是猜测）；
 *   6. 其余一律失败关闭（抛 CategoryResolutionError），不降级、不择优。
 *   冲突规则：ID 命中父类但展示名明确到其子类 → 子类胜出（expand-child）；
 *            ID 与展示名指向不同叶子 → CATEGORY_CONFLICT；
 *            同一父类下重复名 / 多前缀候选 → NAME_AMBIGUOUS / PREFIX_AMBIGUOUS。
 *
 * 用法：
 *   R0_BASE=http://127.0.0.1:8091 R0_USER=admin R0_PW=*** node tools/r0/seed-from-dataset.js
 * 输出：$R0_RUNTIME_DIR/seed-manifest.json 与 proto-injection.json（默认 /tmp 下，不入库）
 * 退出码：0 成功 / 1 运行错误 / 2 配置或输入错误
 */

const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { buildCategoryIndex, resolveCategory, CategoryResolutionError } = require('./lib/resolve-category');

const REPO_ROOT = path.resolve(__dirname, '..', '..');
const BASE = (process.env.R0_BASE || 'http://127.0.0.1:8091').replace(/\/$/, '');
const USER = process.env.R0_USER || 'admin';
const PW = process.env.R0_PW || '';
const DATASET = path.resolve(process.env.R0_DATASET || path.join(__dirname, 'prototype-dataset.json'));
const OVERRIDES_PATH = path.resolve(process.env.R0_OVERRIDES || path.join(__dirname, 'category-overrides.json'));
const RUNTIME_DIR = path.resolve(process.env.R0_RUNTIME_DIR || path.join(os.tmpdir(), 'fastledger-r0-pairs-runtime'));
const BOOK_NAME = process.env.R0_BOOK_NAME || 'R0 同数据账本';

if (!PW) {
  process.stderr.write('SEED_CONFIG_ERROR 缺少 R0_PW（隔离实例测试账号口令）\n');
  process.exit(2);
}
if (!fs.existsSync(DATASET)) {
  process.stderr.write(`SEED_CONFIG_ERROR 数据集不存在：${DATASET}\n`);
  process.exit(2);
}

const ds = JSON.parse(fs.readFileSync(DATASET, 'utf8'));
const { accounts, categories, transactions, budgets } = ds.data;

// 原型账户分类 → 实现账户分类枚举（模型 §2）
const ACCOUNT_CATEGORY = { savings: 2, cash: 1, ewallet: 4, credit: 10, investment: 5, other: 8 };
// 原型分类 type → 实现 catType（1 收入 / 2 支出）
const CAT_TYPE = { income: 1, expense: 2 };

async function call(p, method, body, token) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers.Authorization = `Bearer ${token}`;
  const r = await fetch(BASE + p, { method, headers, body: body ? JSON.stringify(body) : undefined });
  const text = await r.text();
  let j;
  try { j = JSON.parse(text); } catch {
    throw new Error(`BAD_JSON ${method} ${p} status=${r.status} body=${text.slice(0, 200)}`);
  }
  if (j.code !== 0) throw new Error(`API_FAIL ${method} ${p} status=${r.status} body=${JSON.stringify(j).slice(0, 300)}`);
  return j.data;
}

const uuid = () => crypto.randomUUID();
const toCents = (yuan) => Math.round(Number(yuan) * 100);

(async () => {
  fs.mkdirSync(RUNTIME_DIR, { recursive: true });

  const login = await call('/api/v1/auth/login', 'POST', { username: USER, password: PW });
  const token = login.accessToken;
  if (!token) throw new Error('NO_TOKEN');

  const book = await call('/api/v1/books', 'POST', { name: BOOK_NAME, bookType: 2 }, token);
  const bookId = book.id;

  // ---- 清空建账本时自动播种的默认账户/分类（否则两侧账户数不一致，对照失真） ----
  async function resetBook() {
    const deleted = { accounts: 0, categories: 0 };
    const cats = (await call(`/api/v1/books/${bookId}/categories`, 'GET', null, token)).filter((c) => !c.deleted);
    // 先删小类再删大类：实现侧禁止删除仍有子类的分类
    for (const c of cats.filter((x) => x.parentId)) {
      await call(`/api/v1/books/${bookId}/categories/${c.id}`, 'DELETE', null, token);
      deleted.categories += 1;
    }
    for (const c of cats.filter((x) => !x.parentId)) {
      await call(`/api/v1/books/${bookId}/categories/${c.id}`, 'DELETE', null, token);
      deleted.categories += 1;
    }
    const accs = (await call(`/api/v1/books/${bookId}/accounts`, 'GET', null, token)).filter((a) => !a.deleted);
    for (const a of accs) {
      await call(`/api/v1/books/${bookId}/accounts/${a.id}`, 'DELETE', null, token);
      deleted.accounts += 1;
    }
    return deleted;
  }
  const defaultsRemoved = await resetBook();

  // ---- 账户 ----
  const accountMap = {}; // proto id -> impl id
  const accountNotes = [];
  for (const a of accounts) {
    const category = ACCOUNT_CATEGORY[a.category];
    if (!category) throw new Error(`未知账户分类 ${a.category}（${a.name}）`);
    const created = await call(`/api/v1/books/${bookId}/accounts`, 'POST', {
      name: a.name, category, currency: a.currency || 'CNY',
    }, token);
    accountMap[a.id] = created.id;
    accountNotes.push({ protoId: a.id, name: a.name, protoCategory: a.category, implCategory: category, implAccountId: created.id });
  }

  // ---- 分类：先大类，再小类（实现侧只允许两级） ----
  const categoryMap = {};
  const tops = categories.filter((c) => c.parentId === null);
  const children = categories.filter((c) => c.parentId !== null);
  for (const c of tops) {
    const created = await call(`/api/v1/books/${bookId}/categories`, 'POST', {
      name: c.name, parentId: 0, catType: CAT_TYPE[c.type],
    }, token);
    categoryMap[c.id] = created.id;
  }
  for (const c of children) {
    const parentId = categoryMap[c.parentId];
    if (!parentId) throw new Error(`子分类 ${c.name} 的父亲 ${c.parentId} 未建立`);
    const created = await call(`/api/v1/books/${bookId}/categories`, 'POST', {
      name: c.name, parentId, catType: CAT_TYPE[c.type],
    }, token);
    categoryMap[c.id] = created.id;
  }

  // ---- 交易 ----
  const txMap = {};
  const categoryFallbacks = [];
  const categoryAudit = [];
  const categoryConflicts = [];
  // 解析规则的唯一事实来源：tools/r0/lib/resolve-category.js（顶部注释列全优先级）。
  // 与旧实现的差别：
  //   · ID 命中父类、展示名明确到子类 → 子类胜出（expand-child），父类 ID 不再静默覆盖；
  //   · 重复名称 / 多前缀候选 / ID 与展示名指向不同叶子 → 失败关闭（抛错），不再降级或择优。
  const catIndex = buildCategoryIndex(categories);
  // 显式覆盖：只用于"ID 与展示名互相矛盾"的情形，条目必须带 reason/decidedBy/decidedAt
  const overrideList = fs.existsSync(OVERRIDES_PATH)
    ? (JSON.parse(fs.readFileSync(OVERRIDES_PATH, 'utf8')).overrides || [])
    : [];
  for (const o of overrideList) {
    if (!o.txId || !o.categoryId || !o.reason || !o.decidedBy || !o.decidedAt) {
      throw new CategoryResolutionError('OVERRIDE_INVALID', `覆盖条目缺少必填字段：${JSON.stringify(o)}`, { entry: o });
    }
  }
  const overrides = new Map(overrideList.map((o) => [o.txId, o]));
  const appliedOverrides = [];
  const catById = catIndex.byId;
  const VIA_REASON = {
    "id": "精确 ID 命中",
    "id-normalized": "补 cat_ 前缀后命中",
    "name-path": "按分类名路径「父-子」命中",
    "name-prefix": "按同父类唯一前缀命中",
    "expand-child": "ID 为父类但展示名到子类，展开到子类",
    "parent-fallback": "展示名本身即大类名，用大类",
    "manual-override": "ID 与展示名互相矛盾，按 category-overrides.json 的显式决定",
  };
  function resolveForTx(t) {
    const r = resolveCategory(t, catIndex, overrides);
    const implId = categoryMap[r.protoId] || 0;
    if (!implId) {
      throw new CategoryResolutionError("IMPL_CATEGORY_MISSING", `交易 ${t.id}：原型分类「${r.protoId}」在实现侧未建立`, { txId: t.id, protoId: r.protoId });
    }
    for (const note of r.notes) {
      if (note.kind === "id-vs-name") categoryConflicts.push({ txId: t.id, ...note });
      if (note.kind === "override") appliedOverrides.push({ txId: t.id, ...note });
    }
    if (r.via !== "id") {
      categoryFallbacks.push({ txId: t.id, protoCategoryId: t.categoryId, used: r.protoId, reason: VIA_REASON[r.via] || r.via });
    }
    return { protoId: r.protoId, implId, via: r.via };
  }

  for (const t of transactions) {
    const accountId = accountMap[t.accountId];
    const resolved = resolveForTx(t);
    const categoryId = resolved.implId;
    if (!accountId) throw new Error(`交易 ${t.id} 的账户 ${t.accountId} 未建立`);
    const res = await call(`/api/v1/books/${bookId}/transactions`, 'POST', {
      clientUuid: uuid(),
      txType: t.type === 'income' ? 1 : 2, // 原型的「转账」在此数据集里是一个支出分类，不是两账户转账
      accountId,
      amount: toCents(t.amount),
      categoryId,
      txTime: Math.floor(Date.parse(t.time) / 1000),
      timezoneOffset: 480,
      comment: t.note || '',
      source: 0,
    }, token);
    const ids = res.transactionIds || [];
    txMap[t.id] = ids[0] || 0;
    const protoCat = catById.get(resolved.protoId);
    const parent = protoCat && protoCat.parentId ? catById.get(protoCat.parentId) : null;
    categoryAudit.push({
      txId: t.id,
      implTxId: txMap[t.id],
      protoCategoryId: t.categoryId,
      protoDisplayName: t.categoryName,
      resolvedProtoCategoryId: resolved.protoId,
      resolvedName: protoCat ? (parent ? `${parent.name}-${protoCat.name}` : protoCat.name) : '(unresolved)',
      implCategoryId: categoryId,
      via: resolved.via,
    });
  }

  // ---- 预算 ----
  const budgetMap = {};
  for (const b of budgets) {
    const isTotal = b.categoryId === 'total';
    const created = await call(`/api/v1/books/${bookId}/budgets`, 'POST', {
      name: b.categoryName,
      budgetType: isTotal ? 2 : 1,
      categoryId: isTotal ? 0 : categoryMap[b.categoryId],
      periodType: 1,
      amount: toCents(b.amount),
      rollover: false,
      enabled: true,
    }, token);
    budgetMap[b.id] = created.id;
  }

  // ---- 读回实现侧账户余额，回填原型注入包，使两侧数字一致 ----
  const implAccounts = await call(`/api/v1/books/${bookId}/accounts`, 'GET', null, token);
  const balanceByImplId = {};
  for (const a of implAccounts) balanceByImplId[a.id] = a.balance;

  const protoAccounts = accounts.map((a) => ({
    ...a,
    balance: (balanceByImplId[accountMap[a.id]] || 0) / 100,
  }));

  const injection = {
    __fastledger_accounts: protoAccounts,
    __fastledger_categories: categories,
    __fastledger_transactions: transactions,
    __fastledger_budgets: budgets,
    __fastledger_loggedIn: 'true',
  };

  const protoInjectionPath = path.join(RUNTIME_DIR, 'proto-injection.json');
  fs.writeFileSync(protoInjectionPath, `${JSON.stringify(injection, null, 2)}\n`, 'utf8');

  const manifest = {
    generatedAt: new Date().toISOString(),
    dataset: {
      path: path.relative(REPO_ROOT, DATASET).replace(/\\/g, '/'),
      bundleSha256: ds.source.bundleSha256,
      counts: ds.counts,
    },
    bookId,
    bookName: BOOK_NAME,
    defaultsRemoved,
    accountMap,
    categoryMap,
    budgetMap,
    firstTxId: txMap[transactions[0].id] || 0,
    txMap,
    categoryFallbacks,
    categoryConflicts,
    categoryOverridesApplied: appliedOverrides,
    categoryAudit,
    balances: { unit: '分', byImplAccountId: balanceByImplId, protoAccountsAfterAlignment: protoAccounts.map((a) => ({ id: a.id, name: a.name, balance: a.balance })) },
    protoInjectionPath,
    fieldDiffs: [
      { field: 'merchant（商户）', proto: '交易有独立商户字段并展示', impl: '无该字段，交易仅 comment（备注）', handling: '不伪造；备注写原型 note，商户标签在对照时不计入' },
      { field: 'account.balance（账户余额）', proto: '静态演示值 100000/5000/1200/800/-3200', impl: '由交易派生，创建时恒为 0（期初余额 D-4 未实现）', handling: '截图前把实现侧派生余额回填进原型注入包，使两侧显示同一组数字' },
      { field: 'category.displayName', proto: '交易自带类目展示名（如「餐饮-早餐」）', impl: '由分类树「父-子」拼装（如「餐饮-三餐」）', handling: '分类 ID 一一映射；展示文案差异如实保留' },
      { field: 'book（账本）', proto: '无账本概念，单库直接展示', impl: '必须有账本，名称固定为「R0 同数据账本」', handling: '记录在案' },
      { field: 'family（成员）', proto: '2 名成员（我/配偶）', impl: '仅 1 名实例用户，多成员需邀请流程', handling: '本轮不伪造成员，成员页标注数据未对齐' },
      { field: 'investments（投资）', proto: '券商账户/持仓/实物资产/负债', impl: '持仓需 instrument 前置且无公开创建 API', handling: '本轮不种子化，投资页仅作空态对照' },
    ],
  };

  const manifestPath = path.join(RUNTIME_DIR, 'seed-manifest.json');
  fs.writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, 'utf8');

  process.stdout.write(`SEED_OK bookId=${bookId} firstTxId=${manifest.firstTxId}\n`);
  process.stdout.write(`SEED_COUNTS accounts=${Object.keys(accountMap).length} categories=${Object.keys(categoryMap).length} txs=${Object.keys(txMap).length} budgets=${Object.keys(budgetMap).length}\n`);
  process.stdout.write(`SEED_RUNTIME_DIR ${RUNTIME_DIR}\n`);
  process.stdout.write(`SEED_MANIFEST ${manifestPath}\n`);
  process.stdout.write(`SEED_PROTO_INJECTION ${protoInjectionPath}\n`);
})().catch((e) => {
  process.stderr.write(`SEED_ERROR ${e && e.stack ? e.stack : String(e)}\n`);
  process.exit(1);
});
