'use strict';

/**
 * RV-05 / RV-06 安全脱敏自动化测试（node:test，无第三方依赖）。
 *
 * 全部使用「合成假凭证」，不读取、不使用任何真实令牌或口令。
 * 运行：node --test tools/r0/test-redaction.js
 * 退出码：0 = 全部断言通过；1 = 有断言失败。
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const { scrub, redactString, findLeakedSamples, REDACTED } = require('./redaction.js');

// ---------------------------------------------------------------------------
// 合成假凭证（字符串里显式带 FAKE，便于人眼确认不是真实值）
// ---------------------------------------------------------------------------
const FAKE_JWT = 'eyJFAKE_HEADER_TEST.eyJFAKE_PAYLOAD_TEST.FAKE_SIGNATURE_TEST';
const FAKE_OPAQUE_TOKEN = 'FAKE_OPAQUE_TOKEN_0123456789';
const FAKE_COOKIE = 'session=FAKE_SESSION_COOKIE_VALUE; csrf=FAKE_CSRF_VALUE';
const FAKE_PASSWORD = 'FAKE_PASSWORD_ABCDEF_012345';
const FAKE_QUERY_TOKEN = 'FAKE_QUERY_TOKEN_9876543210';
const FAKE_API_KEY = 'FAKE_API_KEY_XYZ_24680';

const ALL_SAMPLES = [
  FAKE_JWT,
  FAKE_OPAQUE_TOKEN,
  FAKE_COOKIE,
  FAKE_PASSWORD,
  FAKE_QUERY_TOKEN,
  FAKE_API_KEY,
];

/** 一组覆盖各类 CDP 事件形态的合成事件。 */
function buildSyntheticEvents() {
  return [
    {
      event: 'Network.requestWillBeSent',
      params: {
        requestId: 'interception-job-9.0',
        timestamp: 31488.11,
        type: 'XHR',
        request: {
          url: `http://127.0.0.1:8080/api/v1/books/1/transactions?page=1&count=60&access_token=${FAKE_QUERY_TOKEN}`,
          method: 'GET',
          headers: {
            Authorization: `Bearer ${FAKE_JWT}`,
            Cookie: FAKE_COOKIE,
            'X-Api-Key': FAKE_API_KEY,
            Accept: 'application/json',
          },
          headersText: `GET /api/v1/books/1/transactions HTTP/1.1\r\nAuthorization: Bearer ${FAKE_JWT}\r\nCookie: ${FAKE_COOKIE}\r\n`,
          postData: JSON.stringify({ username: 'fake_user', password: FAKE_PASSWORD }),
        },
      },
    },
    {
      event: 'Fetch.requestPaused',
      params: {
        requestId: 'interception-job-9.0',
        request: {
          url: `http://127.0.0.1:8080/api/v1/books/1/transactions?token=${FAKE_QUERY_TOKEN}`,
          method: 'GET',
          headers: { Authorization: `Bearer ${FAKE_OPAQUE_TOKEN}`, 'Set-Cookie': FAKE_COOKIE },
        },
      },
    },
    {
      event: 'Network.responseReceived',
      params: {
        requestId: 'interception-job-9.0',
        status: 200,
        response: {
          headers: { 'Set-Cookie': FAKE_COOKIE, 'Content-Type': 'application/json' },
          headersText: `HTTP/1.1 200 OK\r\nSet-Cookie: ${FAKE_COOKIE}\r\n`,
          url: `http://127.0.0.1:8080/api/v1/books/1/transactions?api_key=${FAKE_API_KEY}`,
        },
      },
    },
    {
      event: 'Runtime.consoleAPICalled',
      params: {
        type: 'log',
        args: [
          { type: 'string', value: `token=${FAKE_QUERY_TOKEN} password=${FAKE_PASSWORD}` },
          { type: 'string', value: FAKE_JWT },
        ],
      },
    },
    {
      event: 'Log.entryAdded',
      params: {
        entry: {
          source: 'network',
          level: 'error',
          text: `Failed to load resource: Bearer ${FAKE_JWT}`,
          url: `http://127.0.0.1:8080/api/v1/books/1/transactions?secret=${FAKE_API_KEY}`,
          networkRequestId: 'interception-job-9.0',
        },
      },
    },
  ];
}

test('合成凭证经 scrub 后全部不可见于序列化输出', () => {
  const cleaned = buildSyntheticEvents().map((e) => ({ event: e.event, params: scrub(e.params) }));
  const json = JSON.stringify(cleaned, null, 2);
  const leaked = findLeakedSamples(json, ALL_SAMPLES);
  assert.deepEqual(leaked, [], `输出中仍存在合成凭证片段：${leaked.length} 项`);
  assert.ok(json.includes(REDACTED), '输出应含脱敏标记');
});

test('scrub 保留事件结构（URL 路径 / 方法 / 状态码 / requestId / 时间戳）', () => {
  const [req, paused, resp] = buildSyntheticEvents().map((e) => scrub(e.params));
  assert.equal(req.request.method, 'GET');
  assert.equal(req.requestId, 'interception-job-9.0');
  assert.equal(req.timestamp, 31488.11);
  assert.equal(new URL(req.request.url).pathname, '/api/v1/books/1/transactions');
  assert.equal(paused.request.method, 'GET');
  assert.equal(resp.status, 200);
  assert.equal(resp.response.headers['Content-Type'], 'application/json');
  assert.equal(req.request.headers.Accept, 'application/json');
  assert.equal(req.request.headers.Authorization, REDACTED);
  assert.equal(resp.response.headers['Set-Cookie'], REDACTED);
});

test('redactString 覆盖 headersText 多行、Bearer 非 JWT、URL query、JSON 请求体', () => {
  const cases = [
    `Authorization: Bearer ${FAKE_JWT}\r\nCookie: ${FAKE_COOKIE}\r\nAccept: */*`,
    `Bearer ${FAKE_OPAQUE_TOKEN}`,
    `http://host/path?access_token=${FAKE_QUERY_TOKEN}&page=1`,
    `{"username":"fake_user","password":"${FAKE_PASSWORD}"}`,
    `password=${FAKE_PASSWORD}&remember=1`,
  ];
  for (const raw of cases) {
    const out = redactString(raw);
    assert.deepEqual(findLeakedSamples(out, ALL_SAMPLES), [], `未脱敏干净：${raw.slice(0, 40)}...`);
  }
  const header = redactString(`Authorization: Bearer ${FAKE_JWT}\r\nAccept: */*`);
  assert.ok(header.startsWith('Authorization: ' + REDACTED));
  assert.ok(header.includes('Accept: */*'), '非敏感头必须保留');
});

test('scrub 不误伤描述性字段与不含凭证的普通文本', () => {
  const params = {
    tokenLength: 171,
    tokenInjection: 'Page.addScriptToEvaluateOnNewDocument: localStorage.setItem("fl_token", <login accessToken>)',
    note: 'GET /api/v1/books/1/transactions?page=1&count=60',
    statusText: 'OK',
  };
  const out = scrub(params);
  assert.equal(out.tokenLength, 171);
  assert.equal(out.tokenInjection, params.tokenInjection);
  assert.equal(out.note, params.note);
  assert.equal(out.statusText, 'OK');
});

test('scrub 幂等且处理数组 / 嵌套 / 空值', () => {
  const ev = buildSyntheticEvents();
  const once = JSON.stringify(scrub(ev));
  const twice = JSON.stringify(scrub(scrub(ev)));
  assert.equal(once, twice);
  assert.deepEqual(findLeakedSamples(once, ALL_SAMPLES), []);
  assert.equal(scrub(null), null);
  assert.equal(scrub(42), 42);
  assert.equal(scrub(false), false);
});

test('每个合成事件单独序列化后都不含原始假凭证', () => {
  for (const e of buildSyntheticEvents()) {
    const json = JSON.stringify(scrub(e));
    assert.deepEqual(findLeakedSamples(json, ALL_SAMPLES), [], `事件 ${e.event} 脱敏失败`);
  }
});
