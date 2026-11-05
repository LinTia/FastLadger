# R0 包 B · 同数据集成对截图（原型 × 实现）

本目录是 R0「13 页原型还原」的**同一数据集**成对截图证据：两侧使用**同一份账户/分类/交易/预算数据**、同一浏览器、同一视口，逐页成对。

> 判定边界先说清：本目录证明的是「**同一份数据下，两侧页面结构与关键数值可逐页对照**」。
> 它**不**证明「逐控件运行时行为合格」，也不等于 R0 通过（R0 仍 BLOCK）。

## 1. 采集口径

| 项 | 值 |
|---|---|
| 浏览器 | Microsoft Edge `Edg/156.0.4314.58`（headless + CDP），Node `v22.22.2`，DPR 1 |
| 业务代码基准 | `bff6a2b351bf5a039cdf7c762ec2693fb330cd00`（`server/` 自该提交起零改动） |
| 视口 | `360×800` / `390×844` / `430×932` / `1280×800`（4 档） |
| 页面 | 13 页（见 §3），每页每视口各 1 张 |
| 张数 | 原型 **52** + 实现 **52** = **104** PNG |
| 状态 | `default`（44）、`logged-out`（登录页 4×2）、`dialog-open`（实现侧记一笔弹层 4）；**loading/empty/error/permission/offline/sync-conflict 未覆盖** |
| 角色 | 实现侧 `role=owner`（`manifest-impl.json` → `roleSource: api:/users/me + /books/:id/members`）；**仅采集了 owner 一个角色**，admin/member/只读未采集 |
| 错误 | 两侧 `errors = 0`、`skipped = 0` |
| 采集时间 | 2026-11-05（`manifest-*.json` 逐张记录 `capturedAt`） |
| 轮次 | **第五次整改重采**（分类映射修复后）；上一轮产物保留为 `SHA256SUMS-4th-round.txt` |

## 2. 同一数据集（关键）

数据集不是"手编一份差不多的"，而是**从原型打包产物里提取原生种子数据**，再用公开 API 原样种进实现侧空账本：

```
prototype/assets/index-*.js
   └─(tools/r0/extract-prototype-dataset.js)→ tools/r0/prototype-dataset.json
        └─(tools/r0/seed-from-dataset.js)→ 实现侧隔离实例（HTTP API）
             └─ 回读实现侧派生余额 → proto-injection.json → 注入原型 localStorage
```

| 数据 | 数量 | 说明 |
|---|---|---|
| accounts | 5 | 招商银行储蓄卡 / 现金 / 微信零钱 / 支付宝余额 / 招行信用卡 |
| categories | 37 | 12 个大类 + 25 个小类（含收入/支出/转账） |
| transactions | 22 | 金额、时间、账户、分类与原型完全一致 |
| budgets | 5 | 总预算 + 餐饮/交通/旅游/购物 |

建账本时实现侧会自动播种默认账户/分类，脚本先**清空默认播种**（`seed-manifest.json` → `defaultsRemoved`）再灌原型数据集，否则两侧账户数不一致。

**余额对齐**（`seed-manifest.json` → `balances`）：实现侧初始余额恒为 0、余额由交易派生，因此截图前把实现侧派生余额回填进原型注入包，使两侧显示同一组数字：

| 账户 | 两侧显示 |
|---|---|
| 招商银行储蓄卡 | ¥28,100.00 |
| 现金 | ¥0.00 |
| 微信零钱 | −¥716.30 |
| 支付宝余额 | −¥1,125.00 |
| 招行信用卡 | ¥3,868.00 |

原型原先的静态演示余额（100000 / 5000 / 1200 / 800 / −3200）已被**替换**为上述派生值——这是"同数据"的必要一步，也顺带暴露了 D-4 期初余额未实现的后果（余额只能靠交易堆出来）。

### 2.1 分类映射修复（第五次整改）

第三方复核指出 `seed-from-dataset.js` 的分类兜底顺序有问题：原先「补 `cat_` 前缀」排在名称匹配之前，导致 4 笔交易被映射到**父类**而不是已存在的子类。已修复解析优先级为：

```
① 原型分类 ID 精确命中
② 交易展示名「父-子」按分类树路径精确命中
③ 同父类下按展示名"唯一前缀"匹配（原型数据瑕疵兜底，例：「购物-数码」→「数码家电」）
④ 规范化 ID（补 cat_ 前缀）
⑤ 最后才按父类降级，且必须登记
```

| 交易 | 修复前 | 修复后 | 命中方式 |
|---|---|---|---|
| t007（购物-服饰 ¥580） | `cat_shopping`（父类「购物」） | `shopping_clothes`（「购物-服饰」） | name-path |
| t009（旅游-住宿 ¥2,480） | `cat_travel`（父类「旅游」） | `travel_hotel`（「旅游-住宿」） | name-path |
| t010（旅游-门票 ¥680） | `cat_travel`（父类「旅游」） | `travel_ticket`（「旅游-门票」） | name-path |
| t017（购物-数码 ¥680） | `cat_shopping`（父类「购物」） | `shopping_digital`（「购物-数码家电」） | name-prefix（唯一） |

22 笔映射统计：`id` 18 笔 / `name-path` 3 笔 / `name-prefix` 1 笔 / **父类降级 0 笔**。逐笔审计表见 `docs/r0-evidence-5th/seed-mapping-after.json`，修复前见 `seed-mapping-before.json`。

**对统计聚合的影响**：报表「本月支出分类占比」首行由「购物 ¥580」变为「服饰 ¥580」（其余行不变），即该笔支出从父类聚合下沉到子类。这说明修复前后的报表数字**不能混用**；本目录全部截图与 `seed-manifest.json` 均为修复后重采版本，旧版校验和存于 `SHA256SUMS-4th-round.txt`。

## 3. 页面对照状态

| 页面 | 原型 | 实现 | 数据 | 备注 |
|---|---|---|---|---|
| login | `/login` | `/`（未登录态） | — | 两侧均为未登录状态页 |
| home | `/home` | `#b/1` | 一致 | 见 §4 差异 1/2 |
| add-transaction | `/add`（整页） | 弹层 `#formDlg` | — | 形态不同：整页 vs `320×552` 弹层 |
| transaction-detail | `/transaction` | `#b/1/tx/1` | 一致 | |
| accounts | `/accounts` | `#b/1/accounts` | 一致 | 见 §4 差异 3 |
| categories | `/categories` | `#b/1/categories` | 一致 | |
| invest | `/invest` | `#b/1/invest` | **未对齐** | 实现侧持仓需 instrument 前置，无公开创建 API |
| budget | `/budget` | `#b/1/budget` | 一致（5 条预算） | |
| report | `/report` | `#b/1/report` | 一致 | 见 §5 发现 2 |
| export | `/export` | `#b/1/export` | — | 静态说明页 |
| members | `/family` | `#b/1/members` | **未对齐** | 原型 2 名成员；实现侧 1 名，需邀请流程 |
| mine | `/profile` | `#b/1/mine` | 基本一致 | |
| settings | `/profile`（二级菜单） | `#settings` | 基本一致 | **原型无独立设置路由**：`settings` 与 `mine` 实际是同一个 `/profile`，两图 SHA256 相同，**不构成两个独立原型页面**，统计页数时应按 12 个独立原型路由计 |

## 4. 结构差异（实测，非推测）

1. **首页结余口径不同**：原型「本月结余」= 本月收入 − 本月支出（¥26,910.70）；实现卡片「结余」副标题为"净资产"，值 ¥30,126.70。**同一文案、不同语义**，属需裁决的口径问题。
2. **首页账单行信息密度不同**：原型每行含 商户 + 分类 + 账户 + 支付渠道；实现每行是 分类名 + 备注（实现侧无 merchant 字段，见 `seed-manifest.json` → `fieldDiffs`）。
3. **账户页形态不同**：原型是分组卡片流（净资产头图 + 分类分组）；实现是 `类型/余额/操作` 表格 + 「新增账户」按钮。

## 5. 采集中暴露的实现侧问题（建议进 R0 缺陷清单）

1. **记一笔弹层的分类未按类型过滤**：默认 `类型=支出` 时，`分类` 下拉首项是「工资」（收入类），直接保存会被服务端拒绝（`400 请求参数非法`）。本机种子脚本早期即踩到该问题。
2. **移动端报表「本月支出分类占比」丢失分类名列**：`390×844` 下首列空白，只有 笔数/金额/占比；`1280×800` 下分类名正常（购物 / 超市日用 / 外卖 / 三餐 / 打车 / 公交地铁）。属响应式布局缺陷。
3. **账户余额语义**：实现侧电子钱包余额为负（−716.30 / −1,125.00），因期初余额未实现、余额全部由支出派生；与原型演示值差异明显（D-4 待落地）。

以上 3 条是**采集中实际看到的现象**，未做逐控件复核；是否成立需 QA 按缺陷流程复现确认。

## 6. 复现方式

```bash
# 1) 提取原型数据集（离线，无需服务）
node tools/r0/extract-prototype-dataset.js

# 2) 起一个隔离实例并种子化（需空库 + 一个实例用户）
R0_BASE=http://127.0.0.1:8091 R0_USER=admin R0_PW=*** \
  node tools/r0/seed-from-dataset.js
# 输出（默认在系统临时目录，不入库）：seed-manifest.json / proto-injection.json

# 3) 两侧采集
R0_SIDES=proto R0_PROTO_DIR=./prototype \
R0_PROTO_INJECT=<临时目录>/proto-injection.json \
  node tools/r0/capture-pairs.js

R0_SIDES=impl R0_APP_URL=http://127.0.0.1:8091/ R0_TOKEN=*** \
R0_BOOK_ID=1 R0_TX_ID=1 \
  node tools/r0/capture-pairs.js
```

细节：

- 原型数据层被 toolkit 加了命名空间前缀（`__miaoda_<appid>__:`），脚本**运行时探测**该前缀，不硬编码；
- 实现侧是 hash 路由，脚本用一次性 query 强制新建文档，保证注入脚本生效；
- 令牌只经环境变量传入，不打印、不落盘。

## 7. 未覆盖（不得据此声称 R0 通过）

- 未覆盖 `loading` / `empty` / `error` / `permission`（多角色）/ `offline` / `sync-conflict` 状态；
- 未做像素级 diff（本目录只提供成对图与逐张 SHA256，不做自动相似度打分）；
- `invest` / `members` 两侧数据未对齐（原因见 §3）；
- 仅 1 个角色（账本 owner），未覆盖 admin/member/只读/回收站视角；
- 未在 4 档视口之外验证（平板横屏、超宽屏等）。

## 8. 本目录的状态结论（第五次复核后）

**包 B = PARTIAL，不是 PASS。**

已有：13 页 × 4 视口、两侧各 52 张、同一数据集（含修复后的分类映射）、登录页与记一笔弹层的动作级采集、104 张字节级校验。

仍缺：多角色（仅 owner）、loading/empty/error/permission/offline/conflict 状态、`invest`/`members` 两页数据对齐、像素级 diff。
按任务单口径，**截图数量与采集成功不等于同数据同角色同状态全覆盖**，因此只能判 PARTIAL。

## 9. 文件清单

| 文件 | 说明 |
|---|---|
| `prototype/*.png` | 原型 52 张（13 页 × 4 视口） |
| `impl/*.png` | 实现 52 张（同上） |
| `manifest-prototype.json` | 原型侧逐张元数据：URL/实际路径/视口/字节/SHA256/时间/注入数据集信息 |
| `manifest-impl.json` | 实现侧逐张元数据：同上 + `hasToken`/`action`/`actionMeta`（弹层尺寸与打开状态）/`role`（API 读取的真实角色） |
| `seed-manifest.json` | 数据集映射（原型 id → 实现 id）、余额回填值、字段差异清单、22 笔分类映射审计（`categoryAudit`） |
| `SHA256SUMS-4th-round.txt` | 第四次重采（分类映射修复前）的校验和，仅作历史对照 |
| `SHA256SUMS.txt` | 上述全部文件校验和（108 行） |

> **校验和说明**：`SHA256SUMS.txt` 基于 LF 工作树计算。Windows 上 `core.autocrlf=true` 的 clone 会把 `*.json` / `*.md` / `*.txt` 检出为 CRLF，文本文件哈希据此不同（**PNG 哈希不受影响**）；这是行尾规范化，不是内容篡改。需要逐字节复核时用 `git -c core.autocrlf=false clone`。
