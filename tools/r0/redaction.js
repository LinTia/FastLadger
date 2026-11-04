'use strict';

/**
 * FastLedger R0 运行时证据统一脱敏模块（RV-05 / RV-06）。
 *
 * 用途：CDP 事件在写入任何文件之前必须经过 scrub()。覆盖：
 *   - 请求头 / 响应头 / headersText 中的 Authorization、Proxy-Authorization、
 *     Cookie、Set-Cookie、X-Api-Key 等键值；
 *   - 任意字符串中的 JWT 三段式字面量与 `Bearer <token>`；
 *   - URL 查询参数中可能出现的 token / api_key / password 等凭证；
 *   - JSON 请求体（postData）与表单串中的 password / token 等字段。
 *
 * 设计原则：只替换「值」，保留事件结构（事件名、URL 路径、方法、状态码、
 * 时间戳、requestId 等），以便脱敏后的日志仍能用于 RV-01 证据链核对。
 * 本模块不读取环境变量、不打印任何值、无第三方依赖。
 */

const REDACTED = '[REDACTED]';

// 规范化后精确匹配的敏感键名（去掉大小写、空格、下划线、连字符）。
// 用「精确集合」而非子串匹配，避免把 tokenLength / tokenInjection 这类
// 描述性字段误抹掉，导致日志结构信息丢失。
const SENSITIVE_KEYS = new Set([
  'authorization', 'proxyauthorization',
  'cookie', 'setcookie',
  'xapikey', 'apikey',
  'accesstoken', 'refreshtoken', 'idtoken', 'token',
  'clientsecret', 'secret',
  'password', 'passwd', 'pwd',
  'credential', 'credentials',
]);

function normKey(k) {
  return String(k).toLowerCase().replace(/[\s_-]/g, '');
}

function isSensitiveKey(k) {
  return SENSITIVE_KEYS.has(normKey(k));
}

function redactString(s) {
  if (typeof s !== 'string') return s;
  let out = s;

  // 1) 原始多行请求/响应头文本（CDP headersText，形如 "Authorization: Bearer xxx\r\n"）。
  out = out.replace(
    /^([ \t]*[A-Za-z0-9-]*(?:authorization|cookie|set-cookie|api-key|x-api-key)[A-Za-z0-9-]*[ \t]*:[ \t]*).+$/gim,
    (_m, prefix) => prefix + REDACTED,
  );

  // 2) Bearer <任意非空 token>，不限于 JWT（覆盖 PAT / opaque token）。
  out = out.replace(
    /\b(Bearer\s+)(?!\[REDACTED\])[A-Za-z0-9._~+/=-]+/gi,
    (_m, prefix) => prefix + REDACTED,
  );

  // 3) JWT 三段式字面量（即便没有 Bearer 前缀，例如在 URL / localStorage / 日志文本里）。
  out = out.replace(
    /eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}/g,
    REDACTED,
  );

  // 4) URL 查询参数中的凭证（?token= / &access_token= / &api_key= / &password= ...）。
  out = out.replace(
    /([?&](?:token|access_token|refresh_token|id_token|auth|api_key|apikey|key|secret|password|passwd|pwd|sig|signature)=)[^&#\s"'<>]+/gi,
    (_m, prefix) => prefix + REDACTED,
  );

  // 5) JSON 字符串 / 请求体中的敏感字段值。
  out = out.replace(
    /("(?:password|passwd|pwd|secret|client_secret|access_token|refresh_token|id_token|token|api_key|apikey)"\s*:\s*")[^"]*(")/gi,
    (_m, a, b) => a + REDACTED + b,
  );

  // 6) 表单串 / 纯文本中的 key=value 敏感项。
  out = out.replace(
    /((?:^|[&?;\s])(?:password|passwd|pwd|secret|token|api_key|apikey)=)[^&\s"'<>]+/gi,
    (_m, prefix) => prefix + REDACTED,
  );

  return out;
}

/** 递归脱敏任意 CDP params / 事件对象；保留键名与结构。 */
function scrub(value, depth = 0) {
  if (depth > 40) return REDACTED;
  if (Array.isArray(value)) return value.map((v) => scrub(v, depth + 1));
  if (value && typeof value === 'object') {
    const out = {};
    for (const [k, v] of Object.entries(value)) {
      out[k] = isSensitiveKey(k) ? REDACTED : scrub(v, depth + 1);
    }
    return out;
  }
  if (typeof value === 'string') return redactString(value);
  return value;
}

/** 测试辅助：返回仍出现在 text 中的原始样本（合法时为 []）。 */
function findLeakedSamples(text, samples) {
  return samples.filter((s) => typeof s === 'string' && s.length > 0 && text.includes(s));
}

module.exports = { REDACTED, isSensitiveKey, redactString, scrub, findLeakedSamples };
