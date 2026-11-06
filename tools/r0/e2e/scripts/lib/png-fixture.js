'use strict';

/**
 * FastLedger R0 E2E —— 合成 PNG 样本构造器（RV-14 专用，全部为运行期生成，无真实凭据）。
 *
 * 生成的是「结构完整的 PNG」：真 PNG 8 字节签名 + IHDR + IDAT + tEXt（可选）+ IEND，
 * 每个 chunk 带正确 CRC32。这样扫描器的 isKnownBinary() 会判定为 knownBinary，
 * 精确复现第九次复核 RV-14 指出的「合法 PNG 魔数跳过噪声复扫」路径。
 *
 * 所有负载均为运行时拼接的 SYNTHETIC/FAKE 合成串，绝不写入仓库、绝不输出真实匹配值。
 */

const zlib = require('node:zlib');

const PNG_MAGIC = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

/** CRC32 查表。 */
const CRC_TABLE = (() => {
  const table = new Int32Array(256);
  for (let n = 0; n < 256; n += 1) {
    let c = n;
    for (let k = 0; k < 8; k += 1) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    table[n] = c;
  }
  return table;
})();

/**
 * @param {Buffer} buf 输入。
 * @return {number} CRC32（无符号）。
 */
function crc32(buf) {
  let c = 0xffffffff;
  for (let i = 0; i < buf.length; i += 1) c = CRC_TABLE[(c ^ buf[i]) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}

/**
 * 组装一个 PNG chunk。
 * @param {string} type 4 字符类型。
 * @param {Buffer} data 数据。
 * @return {Buffer}
 */
function chunk(type, data) {
  const typeBuf = Buffer.from(type, 'latin1');
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length, 0);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(Buffer.concat([typeBuf, data])), 0);
  return Buffer.concat([len, typeBuf, data, crc]);
}

/**
 * 生成一个结构完整的 1x1 RGBA PNG，可选把任意「文本负载」放进标准 tEXt chunk。
 * @param {{text?: Buffer|string, highEntropyTail?: Buffer}} [opts]
 * @return {Buffer} PNG 字节。
 */
function makePng(opts = {}) {
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(1, 0);  // width
  ihdr.writeUInt32BE(1, 4);  // height
  ihdr[8] = 8;               // bit depth
  ihdr[9] = 6;               // color type RGBA
  ihdr[10] = 0;              // compression
  ihdr[11] = 0;              // filter
  ihdr[12] = 0;              // interlace
  const raw = Buffer.from([0x00, 0x22, 0x44, 0x66, 0x88]); // filter byte + RGBA
  const idat = zlib.deflateSync(raw);

  const parts = [PNG_MAGIC, chunk('IHDR', ihdr), chunk('IDAT', idat)];
  if (opts.text !== undefined && opts.text !== null) {
    const payload = Buffer.isBuffer(opts.text) ? opts.text : Buffer.from(String(opts.text), 'latin1');
    // 标准 tEXt chunk：关键字 "Comment\0" + 文本（latin1 字节可含 NUL / 高位非法字节）
    parts.push(chunk('tEXt', Buffer.concat([Buffer.from('Comment\0', 'latin1'), payload])));
  }
  parts.push(chunk('IEND', Buffer.alloc(0)));
  if (opts.highEntropyTail) parts.push(opts.highEntropyTail);
  return Buffer.concat(parts);
}

/**
 * 生成确定性的高熵二进制填充（模拟正常 PNG 压缩数据，不含任何可读凭据）。
 * @param {number} size 字节数。
 * @param {number} [seed] 种子。
 * @return {Buffer}
 */
function highEntropyBytes(size, seed = 0x5eed) {
  const out = Buffer.alloc(size);
  let x = seed >>> 0;
  for (let i = 0; i < size; i += 1) {
    x = (Math.imul(x, 1664525) + 1013904223) >>> 0;
    out[i] = (x >>> 16) & 0xff;
  }
  return out;
}

module.exports = { PNG_MAGIC, crc32, chunk, makePng, highEntropyBytes };
