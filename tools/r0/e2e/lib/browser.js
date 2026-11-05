'use strict';
/**
 * 浏览器启动：本地用已安装的 Edge（channel=msedge，不下载额外浏览器）；
 * CI 用 playwright 自带的 chromium（workflow 里 `npx playwright install --with-deps chromium`）。
 */
function loadChromium() {
  try { return { chromium: require('playwright').chromium, from: 'playwright' }; } catch (_) { /* fallthrough */ }
  try { return { chromium: require('playwright-core').chromium, from: 'playwright-core' }; } catch (_) { /* fallthrough */ }
  throw new Error('缺少 playwright / playwright-core：请在 tools/r0/e2e 下 npm install');
}

async function launch() {
  const { chromium, from } = loadChromium();
  const channel = process.env.R0_BROWSER_CHANNEL || '';
  const opts = {
    headless: process.env.R0_HEADED !== '1',
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  };
  if (channel) opts.channel = channel;
  if (process.env.R0_BROWSER_EXECUTABLE) opts.executablePath = process.env.R0_BROWSER_EXECUTABLE;
  const browser = await chromium.launch(opts);
  const version = browser.version();
  return { browser, version, from, channel: channel || '(bundled)' };
}

module.exports = { launch, loadChromium };
