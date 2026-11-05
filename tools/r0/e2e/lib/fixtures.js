'use strict';
/**
 * 合成测试数据：固定口令的本地用户 + 一个"有数据"账本 + 一个"空数据"账本 + 三种角色。
 * 全部走公开 API / CLI，不直连数据库；不使用任何生产凭证。
 */
const api = require('./api');

const PASSWORD = 'R0e2e-local-2026';

async function bootstrap(server) {
  const { base } = server;

  // owner 用 CLI 建（可带 --owner 实例管理员标记）；admin/member 走"邀请码加入"真实路径
  server.createUser('owner', PASSWORD, { owner: true });
  const ownerTok = (await api.login(base, 'owner', PASSWORD)).accessToken;

  // 账本：main（有数据）/ empty（空数据）
  const main = await api.createBook(base, ownerTok, 'E2E 主账本', 2);
  const empty = await api.createBook(base, ownerTok, 'E2E 空账本', 2);

  // 角色：admin1 → admin(2)，member1 → member(3)，走真实邀请码兑换（未认证入口）
  for (const [uname, role] of [["admin1", 2], ["member1", 3]]) {
    const inv = await api.invite(base, ownerTok, main.id, role);
    await api.join(base, main.id, { code: inv.code, username: uname, password: PASSWORD });
  }
  const adminTok = (await api.login(base, "admin1", PASSWORD)).accessToken;
  const memberTok = (await api.login(base, "member1", PASSWORD)).accessToken;

  // 账户：建账本时已自动播种（现金/储蓄卡/信用卡），直接取用，不再自造
  const accounts = await api.accounts(base, ownerTok, main.id);
  if (accounts.length < 2) throw new Error(`默认账户不足：${accounts.length}`);
  const cash = accounts.find((a) => a.category === 1) || accounts[0];
  const bank = accounts.find((a) => a.category === 2) || accounts[1];
  const credit = accounts.find((a) => a.category === 10) || null;

  // 分类：API 返回 catType（1=收入 2=支出），parentId=0 为大类；取子类做交易分类
  const flatCats = await api.categories(base, ownerTok, main.id, false);
  const leaves = flatCats.filter((c) => c.parentId !== 0);
  const expenseCat = leaves.find((c) => c.catType === 2) || null;
  const incomeCat = leaves.find((c) => c.catType === 1) || null;
  if (!expenseCat) throw new Error(`没有可用的支出子类，实际分类数=${flatCats.length}`);

  const now = Math.floor(Date.now() / 1000);
  const mk = (over = {}) => ({
    clientUuid: require('node:crypto').randomUUID(),
    txType: 2,
    accountId: cash.id,
    amount: 1234,
    categoryId: expenseCat.id,
    txTime: now,
    timezoneOffset: 480,
    comment: 'e2e',
    source: 0,
    ...over,
  });

  const created = [];
  created.push((await api.createTx(base, ownerTok, main.id, mk({ amount: 1234, comment: '早餐' }))).transactionIds[0]);
  created.push((await api.createTx(base, ownerTok, main.id, mk({ amount: 5000, comment: '打车', accountId: bank.id }))).transactionIds[0]);
  if (incomeCat) {
    created.push((await api.createTx(base, ownerTok, main.id, mk({ txType: 1, amount: 800000, categoryId: incomeCat.id, comment: '工资' }))).transactionIds[0]);
  }
  // 一笔删除（进回收站），用于 C 的回收站/恢复断言
  const toDelete = (await api.createTx(base, ownerTok, main.id, mk({ amount: 999, comment: '待删除' }))).transactionIds[0];
  await api.deleteTx(base, ownerTok, main.id, toDelete);

  const accountsAfter = await api.accounts(base, ownerTok, main.id);

  return {
    password: PASSWORD,
    users: {
      owner: { username: 'owner', token: ownerTok },
      admin: { username: 'admin1', token: adminTok },
      member: { username: 'member1', token: memberTok },
    },
    books: {
      main: { id: main.id, name: 'E2E 主账本' },
      empty: { id: empty.id, name: 'E2E 空账本' },
    },
    ids: {
      accounts: accountsAfter,
      cashId: cash.id,
      bankId: bank.id,
      creditId: credit ? credit.id : null,
      expenseCatId: expenseCat.id,
      incomeCatId: incomeCat ? incomeCat.id : null,
      txIds: created,
      deletedTxId: toDelete,
    },
  };
}

/** 让页面以指定角色登录：直接写 localStorage（与真实登录后状态等价），避免重复走登录流程 */
function storageStateFor(ctx, role) {
  const u = ctx.users[role];
  return {
    origins: [{
      origin: ctx.origin,
      localStorage: [
        { name: 'fl_token', value: u.token },
        { name: 'fl_whoami', value: u.username },
      ],
    }],
  };
}

module.exports = { bootstrap, storageStateFor, PASSWORD };
