# R0 第四次证据补齐 · 包 B：原型 × 实现同视口成对截图

> 采集日期：2026-11-05　采集主体：WorkBuddy（Owner 授权本地隔离实例）
> 浏览器：Microsoft Edge `156.0.4314.58`（headless=new）　Node：`v22.22.2`　DPR：1
> 视口：`360x800` / `390x844` / `430x932` / `1280x800`
> 业务代码证据基准：`bff6a2b351bf5a039cdf7c762ec2693fb330cd00`（`server/` 自该提交起零改动）
> 采集脚本：`tools/r0/capture-pairs.js`

## 1. 目录与文件

```
pairs/
├── prototype/            52 张：13 页 × 4 视口（原型静态构建）
├── impl/                 44 张：11 路由 × 4 视口（本地隔离实例）
├── manifest-prototype.json
├── manifest-impl.json
├── SHA256SUMS.txt        98 行（96 PNG + 2 manifest）
└── README.md
```

## 2. 采集环境与数据前置

- **原型**：`prototype/` 静态构建快照，由脚本内置静态服务（SPA fallback）提供；演示账号 `me/123456`（README 预填），脚本自动点击「登录」一次后按路径重载。
- **实现**：本地隔离实例（`EBK_DB_PATH` 指向临时目录，一次性随机 `EBK_JWT_SECRET`），schema v8；种子数据：1 个账本「R0 采集账本」、4 个账户（现金/储蓄/零钱/信用卡）、2 笔交易（支出 ¥128.50 餐饮、收入 ¥8,000 工资）。token 仅经环境变量传入采集进程，不落盘、不打印。
- **登录态**：实现侧经 `Page.addScriptToEvaluateOnNewDocument` 注入 `fl_token`；原型侧脚本点击登录按钮。

## 3. 覆盖矩阵（13 页）

| 页面 | 原型路径 | 实现路由 | 原型 | 实现 |
|---|---|---|---|---|
| 登录 | `/login` | 状态页（需清 token） | ✅ 4 视口 | ⏭ 跳过（见 §5） |
| 首页账单 | `/home` | `#b/{book}` | ✅ 4 | ✅ 4 |
| 记一笔（截图/手动） | `/add` | 弹层（无独立路由） | ✅ 4 | ⏭ 跳过（见 §5） |
| 交易详情 | `/transaction` | `#b/{book}/tx/{tx}` | ✅ 4 | ✅ 4 |
| 账户 | `/accounts` | `#b/{book}/accounts` | ✅ 4 | ✅ 4 |
| 两级分类 | `/categories` | `#b/{book}/categories` | ✅ 4 | ✅ 4 |
| 投资资产 | `/invest` | `#b/{book}/invest` | ✅ 4 | ✅ 4 |
| 预算 | `/budget` | `#b/{book}/budget` | ✅ 4 | ✅ 4 |
| 报表 | `/report` | `#b/{book}/report` | ✅ 4 | ✅ 4 |
| 导出 | `/export` | `#b/{book}/export` | ✅ 4 | ✅ 4 |
| 家庭成员 | `/family` | `#b/{book}/members` | ✅ 4 | ✅ 4 |
| 我的 | `/profile` | `#b/{book}/mine` | ✅ 4 | ✅ 4 |
| 我的设置 | `/profile`（二级菜单） | `#settings` | ✅ 4（与“我的”同路径） | ✅ 4 |

- 原型：52 张，**0 错误**，请求路径与落地路径 **0 偏差**（manifest `actualPath`）。
- 实现：44 张，**0 错误**；`login`、`add-transaction` 各 4 视口跳过（8 条 skipped）。

## 4. 复现命令（不含凭证）

```bash
# 原型侧（仓库外临时输出）
R0_SIDES=proto R0_PROTO_DIR="$PWD/prototype" \
R0_OUT_DIR="$(mktemp -d)" R0_EDGE_PATH="<edge-path>" \
node tools/r0/capture-pairs.js

# 实现侧（需先启动本地隔离实例并取得 token/bookId/txId）
R0_SIDES=impl R0_APP_URL="http://127.0.0.1:8091/" \
R0_TOKEN="<accessToken>" R0_BOOK_ID="<bookId>" R0_TX_ID="<txId>" \
R0_OUT_DIR="$(mktemp -d)" R0_EDGE_PATH="<edge-path>" \
node tools/r0/capture-pairs.js
```

写正式证据目录需 `R0_WRITE_EVIDENCE=1` 且 `R0_WRITE_EVIDENCE_CONFIRM=YES`（双重确认），否则默认输出仓库外临时目录。

## 5. 已知限制 / 未覆盖（诚实边界）

1. **非同数据、非同状态**：原型用其内置演示数据，实现用本轮种子数据，两者数值不同。本目录完成的是**同视口、默认（normal）态**的页面级对照，**不是**严格"同数据同状态"对照；严格同数据需 QA 构造统一数据集后重采。
2. **实现侧 `login` 跳过**：实现为状态页（清 token 才显示），而采集脚本注入 token 后会再次登录，故本轮未采实现登录页（原型登录页已采）。
3. **实现侧 `add-transaction` 跳过**：记一笔为弹层（截图识别/手动/转账），无独立路由，需动作级交互采集。
4. **原型 `settings` 与 `mine` 同路径**：原型设置入口在「我的」二级菜单，两页截图相同（SHA256 一致），属已知映射。
5. **仅 normal 态**：未覆盖 loading/pending/empty/error/permission/offline/conflict 等状态；这些仍属包 B/C 后续工作。
6. **未做像素级 diff**：本轮只采集成对截图与 manifest，未生成差异标注图；差异说明见 R0 主报告 §6/§7 与后续 QA 对照表。

## 6. 证据完整性

- `SHA256SUMS.txt` 覆盖 96 张 PNG 与 2 个 manifest；任一张被替换可用 `sha256sum -c SHA256SUMS.txt` 检出。
- manifest 不含 token、Authorization、JWT、Cookie 或个人绝对路径（`protoDir` 已占位为 `<R0_PROTO_DIR>`）。
- 本目录为**新增**，未覆盖 `docs/r0-screenshots/` 既有 43 张与 `runtime/` 历史证据。
