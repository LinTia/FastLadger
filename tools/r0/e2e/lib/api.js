'use strict';
/**
 * 最小 API 客户端：只覆盖 E2E 需要的能力。
 * 所有请求都带超时；错误信息里不含令牌。
 */

async function raw(base, method, p, { token, body, timeout = 15000 } = {}) {
  const headers = { 'content-type': 'application/json' };
  if (token) headers.authorization = `Bearer ${token}`;
  const res = await fetch(`${base}${p}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(timeout),
  });
  const text = await res.text();
  let json = null;
  try { json = text ? JSON.parse(text) : null; } catch (_) { /* 非 JSON（如导出文件流） */ }
  return { status: res.status, json, text, headers: res.headers };
}

function unwrap(r) {
  if (r.json && typeof r.json === 'object' && 'data' in r.json) return r.json.data;
  return r.json;
}

async function ok(base, method, p, opts = {}) {
  const r = await raw(base, method, p, opts);
  if (r.status < 200 || r.status >= 300) {
    const msg = r.json && r.json.msg ? r.json.msg : r.text.slice(0, 200);
    const err = new Error(`${method} ${p} → HTTP ${r.status}：${msg}`);
    err.status = r.status;
    err.payload = r.json;
    throw err;
  }
  return unwrap(r);
}

const api = {
  raw,
  login: (base, username, password) => ok(base, 'POST', '/api/v1/auth/login', { body: { username, password } }),
  register: (base, username, password) => ok(base, 'POST', '/api/v1/auth/register', { body: { username, password } }),
  me: (base, token) => ok(base, 'GET', '/api/v1/users/me', { token }),

  createBook: (base, token, name, bookType = 2) => ok(base, 'POST', '/api/v1/books', { token, body: { name, bookType } }),
  listBooks: (base, token) => ok(base, 'GET', '/api/v1/books', { token }),
  getBook: (base, token, id) => ok(base, 'GET', `/api/v1/books/${id}`, { token }),

  accounts: (base, token, id) => ok(base, 'GET', `/api/v1/books/${id}/accounts`, { token }),
  createAccount: (base, token, id, body) => ok(base, 'POST', `/api/v1/books/${id}/accounts`, { token, body }),

  categories: (base, token, id, tree = true) => ok(base, 'GET', `/api/v1/books/${id}/categories${tree ? '?tree=1' : ''}`, { token }),
  createCategory: (base, token, id, body) => ok(base, 'POST', `/api/v1/books/${id}/categories`, { token, body }),

  transactions: (base, token, id, query = '') => ok(base, 'GET', `/api/v1/books/${id}/transactions${query}`, { token }),
  transaction: (base, token, id, txId) => ok(base, 'GET', `/api/v1/books/${id}/transactions/${txId}`, { token }),
  createTx: (base, token, id, body) => ok(base, 'POST', `/api/v1/books/${id}/transactions`, { token, body }),
  updateTx: (base, token, id, txId, body) => ok(base, 'PUT', `/api/v1/books/${id}/transactions/${txId}`, { token, body }),
  // 删除/恢复需要 clientUuid（幂等回执），缺省会 400
  deleteTx: (base, token, id, txId, clientUuid = require('node:crypto').randomUUID()) =>
    ok(base, 'DELETE', `/api/v1/books/${id}/transactions/${txId}?clientUuid=${encodeURIComponent(clientUuid)}`, { token }),
  restoreTx: (base, token, id, txId, clientUuid = require('node:crypto').randomUUID()) =>
    ok(base, 'POST', `/api/v1/books/${id}/transactions/${txId}/restore?clientUuid=${encodeURIComponent(clientUuid)}`, { token }),
  recycle: (base, token, id) => ok(base, 'GET', `/api/v1/books/${id}/recycle`, { token }),

  members: (base, token, id) => ok(base, 'GET', `/api/v1/books/${id}/members`, { token }),
  invite: (base, token, id, role = 3) => ok(base, 'POST', `/api/v1/books/${id}/members/invite`, { token, body: { role } }),
  // join 是未认证入口：凭邀请码 + 用户名/口令完成注册或绑定
  join: (base, id, { code, username, password }) => ok(base, 'POST', `/api/v1/books/${id}/members/join`, { body: { code, username, password } }),
  changeRole: (base, token, id, uid, role) => ok(base, 'PUT', `/api/v1/books/${id}/members/${uid}`, { token, body: { role } }),
  removeMember: (base, token, id, uid) => ok(base, 'DELETE', `/api/v1/books/${id}/members/${uid}`, { token }),

  pull: (base, token, id, since = 0) => ok(base, 'GET', `/api/v1/books/${id}/sync/pull?since=${since}`, { token }),
  push: (base, token, id, items) => ok(base, 'POST', `/api/v1/books/${id}/sync/push`, { token, body: { items } }),

  reportsMonthly: (base, token, id) => ok(base, 'GET', `/api/v1/books/${id}/reports/monthly`, { token }),
  reportsCategories: (base, token, id) => ok(base, 'GET', `/api/v1/books/${id}/reports/categories`, { token }),
  netWorth: (base, token, id) => ok(base, 'GET', `/api/v1/books/${id}/net-worth`, { token }),
};

module.exports = api;
