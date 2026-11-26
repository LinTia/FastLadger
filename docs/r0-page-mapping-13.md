# R0-T21 · 13 个原型页面逐页七要素映射

> 任务编号：R0-T21（九条第 1 条）。文档性质：**只读采集与台账编制**，不改任何业务代码、测试、CSS、统计、fixture、controls.json、runner、CI，不补拍截图、不启动生产服务。
> 基线：`work/r0-t25-doc-align` 最新 HEAD `b6199faefcf1c61c89d9b887904c4ff7fb06b053`。
> 结论边界：本表只证明「13 页七要素已逐格建账」，**不构成九条第 1 条 PASS，也不构成 R0 PASS**；R0 维持 BLOCK。

---

## 1. 七要素定义（依据 `收尾清单:181`）

| 要素 | 含义 | 证据要求 |
|---|---|---|
| ① 原型证据 | 原型侧（`prototype/`）的文件、清单或成对截图 | `文件:行号` 或 `截图相对路径`，无则写 `缺` |
| ② 实现证据 | 实现侧（`server/web/dist/`）的源码函数或截图 | `文件:行号` 或 `截图相对路径`，无则写 `缺` |
| ③ 差异 | 原型与实现的差异（含结构/数据/口径） | 指向权威差异描述的行号 |
| ④ 目标 | 该页在门禁第 1 条下的对齐结论 | `一致` / `待裁决` / `待验证` / `不适用` |
| ⑤ 影响 | 差异对业务语义或健壮性的影响 | 关联 R0-E 编号或 `无` |
| ⑥ 依赖 | 达成对齐所需的后续任务或前置 | 关联 T 编号或 `无` |
| ⑦ 测试映射 | 对应截图基线、成对图或测试用例 | 截图路径或 `缺` |

## 2. 行号口径（重要）

- 源码行号以**当前 HEAD `b6199fa` 的 `server/web/dist/app.js` 实际 `grep` 结果**为准（本表已逐函数核实）。
- `基准冻结:82–96`（13 页清单）与 `基准冻结:122–136`（交互矩阵）记录的 render 行号是**业务代码证据基准 `bff6a2b` 口径**（见 `基准冻结:142`），与当前 HEAD 函数名一致、行号存在偏移；本表实现证据列以当前 HEAD 行号为准，并保留 `基准冻结` 行号作盘点来源引用。
- 截图相对路径均以 `docs/r0-screenshots/` 为根；`截图README` 指 `docs/r0-screenshots/README.md`，`pairs/README` 指 `docs/r0-screenshots/pairs/README.md`。

## 3. 13 页 × 7 要素台账

| # | 页面 | 1 原型证据 | 2 实现证据 | 3 差异 | 4 目标 | 5 影响 | 6 依赖 | 7 测试映射 |
|---|---|---|---|---|---|---|---|---|
| 1 | 登录 | `prototype/README.md:11`；`pairs/prototype/login__proto__{360x800,390x844,430x932,1280x800}.png`；`pairs/README:82` | `server/web/dist/app.js:161` renderLogin；`login/{360x800,390x844,430x932,1280x800}/normal.png`（`截图README:85-88`）；`基准冻结:84` | 错误红字/pending `[待确认]`（`基准冻结:124`）；「加入账本」入口缺（R0-E-38，`基准冻结:163`） | 待验证 | 登录为入口；pending/错误态未验影响可用性判定 | T22 状态矩阵 | `截图README:85-88`（4 视口 normal）；`runtime/README.md`（accessToken 实测）；登录 pending/error 截图 `缺` |
| 2 | 首页账单 | `prototype/README.md:11`；`pairs/prototype/home__proto__{360x800,390x844,430x932,1280x800}.png` | `server/web/dist/app.js:1963` renderHome + `:1906` renderBillStream；`home/{360x800,390x844,430x932,1280x800}/empty.png` + `home/390x844/error.png`（`截图README:91-95`）；`runtime/01-pending-390x844.png` | 原型采用本月收入减本月支出（`pairs/README:98`）；当前实现端将「结余」展示为净资产（¥30,126.70），属实现口径不一致；账单行缺账户/渠道/商户（`pairs/README:99`）；失败态骨架屏未清理 R0-E-35（`基准冻结:125`）；首页无筛选（`基准冻结:125`） | Owner 口径已确定：首页结余＝本月收入−本月支出（不用全部账户余额合计、不用投资页净资产；投资页仍用独立净资产口径）；实现端待对齐、待验证 | R0-E-20（口径，尚未修复）、R0-E-21（信息缺失）、R0-E-35（骨架残留） | T23（R0-E-35 独立复跑）；T22 状态矩阵 | `截图README:91-95`（4 empty + 1 error）；`runtime/01-05`；pairs home；有数据态截图 `缺` |
| 3 | 截图识别记账 | `prototype/README.md:11`（13 页清单含本页）；`PRD:210-216`（§4.3.2 端侧 OCR） | Web 无路由 / 无 OCR 调用（`基准冻结:86`） | Web 不实现符合 PRD §5.2（`基准冻结:86,127`；`PRD:333`「Web 不做 OCR」） | 不适用 | 无（Web R0 范围外） | Flutter 端 OCR 另行审计（`基准冻结:127`） | N/A + `PRD:326-333`（§5.2 多端形态与 OCR 分工） |
| 4 | 手动记账/转账 | `prototype/README.md:11`；`pairs/prototype/add-transaction__proto__{360x800,390x844,430x932,1280x800}.png` | `server/web/dist/app.js:548` openTxForm + `:614` openTxEditForm；`index.html #formDlg`；`add-transaction/{360x800,390x844,430x932,1280x800}/normal.png`（`截图README:96-99`） | 整页 vs 弹层（`pairs/README:84`）；缺支付渠道/商户；类型非分段控件（`基准冻结:126`） | 待验证 | R0-E-23 / R0-E-29 | T22 状态矩阵 | `截图README:96-99`（4 视口 normal）；pairs add-transaction；校验失败态截图 `缺` |
| 5 | 交易详情 | `prototype/README.md:11`；`pairs/prototype/transaction-detail__proto__{360x800,390x844,430x932,1280x800}.png` | `server/web/dist/app.js:2008` renderTxDetail；`pairs/impl/transaction-detail__impl__{360x800,390x844,430x932,1280x800}.png`；`pairs/README:85`（§3 一致） | 未展示 paymentChannel；备注非空时 merchant 不显示；删除文案与回收站能力矛盾（`基准冻结:128`） | 待验证 | R0-E-22 / R0-E-24 | T22 状态矩阵 | pairs transaction-detail（4 视口）；43 张基线批次独立交易详情截图 `缺`（`transactions/` 为交易总表扩展页） |
| 6 | 账户 | `prototype/README.md:11`；`pairs/prototype/accounts__proto__{360x800,390x844,430x932,1280x800}.png` | `server/web/dist/app.js:678` renderAccounts + `:1802` renderAccountDetail；`accounts/{360x800,390x844,430x932,1280x800}/normal.png` + `account-detail/{390x844,1280x800}/normal.png`（`截图README:102-107`） | 分组卡片流 vs 表格（`pairs/README:100`）；无净资产摘要；无期初余额输入（`基准冻结:129`） | 待裁决 | R0-E-06 / R0-E-28 | T22 状态矩阵 | `截图README:102-107`（6 张）；pairs accounts |
| 7 | 两级分类 | `prototype/README.md:11`；`pairs/prototype/categories__proto__{360x800,390x844,430x932,1280x800}.png` | `server/web/dist/app.js:2103` renderCategories；`categories/{390x844,1280x800}/normal.png`（`截图README:108-109`） | 删除引用计数 `[待确认]`（`基准冻结:130`）；分类映射已修复（`pairs/README:39-46`） | 待验证 | 基本具备，视觉待验 | T22 状态矩阵 | `截图README:108-109`（2 张）；pairs categories |
| 8 | 投资资产 | `prototype/README.md:11`；`pairs/prototype/invest__proto__{360x800,390x844,430x932,1280x800}.png` | `server/web/dist/app.js:1277` renderInvest；`invest/{390x844,1280x800}/empty.png`（`截图README:116-117`） | 数据未对齐（持仓需 instrument 前置，无公开创建 API，`pairs/README:88`）；净资产在 invest 页（`基准冻结:129`） | 待验证 | 投资/持仓是否独立管理待定（`基准冻结:502`）；对齐度待截图核验（`基准冻结:91`） | T22（invest 数据对齐） | `截图README:116-117`（2 张 empty）；pairs invest（未对齐） |
| 9 | 预算 | `prototype/README.md:11`；`pairs/prototype/budget__proto__{360x800,390x844,430x932,1280x800}.png` | `server/web/dist/app.js:1139` renderBudget；`budget/{390x844,1280x800}/empty.png`（`截图README:114-115`） | 三态色 `[待确认]`（`基准冻结:131`）；数据一致 5 条预算（`pairs/README:89`） | 待验证 | 视觉待验 | T22 状态矩阵 | `截图README:114-115`（2 张 empty）；pairs budget |
| 10 | 报表 | `prototype/README.md:11`；`pairs/prototype/report__proto__{360x800,390x844,430x932,1280x800}.png` | `server/web/dist/app.js:835` renderReport；`report/{390x844,1280x800}/empty.png`（`截图README:112-113`） | 移动端分类名丢失（`pairs/README:105`）；空态 `[待确认]`（`基准冻结:132`） | 待验证 | 响应式布局缺陷 | T22 状态矩阵 | `截图README:112-113`（2 张 empty）；pairs report |
| 11 | 导出 | `prototype/README.md:11`；`pairs/prototype/export__proto__{360x800,390x844,430x932,1280x800}.png` | `server/web/dist/app.js:964` renderExport；`export/{390x844,1280x800}/normal.png`（`截图README:118-119`） | 静态说明页（`pairs/README:91`）；空态/超大截断 `[待确认]`（`基准冻结:134`） | 待验证 | 基本具备 | T22 状态矩阵 | `截图README:118-119`（2 张 normal）；pairs export |
| 12 | 家庭成员 | `prototype/README.md:11`；`pairs/prototype/members__proto__{360x800,390x844,430x932,1280x800}.png` | `server/web/dist/app.js:1667` renderMembers；`members/{390x844,1280x800}/normal.png`（`截图README:120-121`） | 数据未对齐（原型 2 名成员 vs 实现 1 名，`pairs/README:92`）；无账本级开关/隐私提示/邀请撤销（`基准冻结:133`） | 待验证 | R0-E-19 / R0-E-26 | T22（members 数据对齐） | `截图README:120-121`（2 张 normal）；pairs members（未对齐） |
| 13 | 我的设置 | `prototype/README.md:11`；`pairs/prototype/mine__proto__{4视口}.png` + `pairs/prototype/settings__proto__{4视口}.png`（原型同 `/profile`，`pairs/README:94`） | `server/web/dist/app.js:2172` renderMine + `:1549` renderSettings；`mine/{390x844,1280x800}/normal.png` + `settings/{360x800,390x844,430x932,1280x800}/normal.png`（`截图README:122-127`） | 设置缺改密/会话管理/服务器备份区块（`基准冻结:135`）；mine 8 入口与平铺顶栏重复（R0-E-01，`基准冻结:136`）；原型无独立设置路由（`pairs/README:94`） | 待裁决 + 待验证 | R0-E-17 / R0-E-18 / R0-E-14 / R0-E-01 | T22 状态矩阵 | `截图README:122-127`（6 张 normal）；pairs mine + settings |

## 4. `缺` 项汇总（作为后续 R0-T22 输入，不在 T21 内自行解决）

以下为七要素逐格核对后明确标记为 `缺` 或 `[待确认]` 的证据，T21 只登记、不补采、不解决：

1. 登录：登录 pending / 错误红字运行时截图 `缺`（`基准冻结:124` 标 `[待确认]`）。
2. 首页账单：有数据态（非 empty）截图 `缺`（本地 transactions total=0，`基准冻结:200`）；筛选/月份切换非本页能力。
3. 手动记账/转账：校验失败态截图 `缺`（`基准冻结:233`）。
4. 交易详情：43 张基线批次独立交易详情截图 `缺`（仅 pairs 成对图；`transactions/` 为交易总表扩展页）。
5. 投资资产：两侧数据未对齐 `缺`（`pairs/README:88`）。
6. 家庭成员：两侧数据未对齐 `缺`（`pairs/README:92`）。
7. 全 13 页：loading / empty / error / success / permission / offline / conflict 状态逐页矩阵 `缺`（`pairs/README:120-126`；`截图README:145-302`）。
8. 多角色（admin/member/只读）截图 `缺`（仅 owner，`pairs/README:7`）。
9. 像素级 diff `缺`（`pairs/README:121`）。

以上 9 类缺口全部转交 R0-T22 的状态矩阵与同视口对照，T21 不据此宣称第 1 条 PASS。

## 5. 三页抽查记录（登录 / 投资资产 / 我的设置）

抽查方式：`sed -n` 读取函数首行核对函数名，`xxd` 校验 PNG 魔数 `89504e470d0a1a0a`。

| 页 | 源码核对 | 截图核对 |
|---|---|---|
| 登录 | `app.js:161` = `function renderLogin(mode) {` ✅ | `login/390x844/normal.png` 存在，PNG 魔数正确 ✅ |
| 投资资产 | `app.js:1277` = `async function renderInvest(bookId) {` ✅ | `invest/390x844/empty.png` 存在，PNG 魔数正确 ✅ |
| 我的设置 | `app.js:2172` = `async function renderMine(bookId) {`、`app.js:1549` = `async function renderSettings() {` ✅ | `mine/390x844/normal.png`、`settings/390x844/normal.png` 存在，PNG 魔数正确 ✅ |

## 6. 检查命令与结果（13 页完整性）

```text
$ grep -c '^| [0-9]' docs/r0-page-mapping-13.md
13
$ grep '^| [0-9]' docs/r0-page-mapping-13.md | awk -F'|' '{print NF}' | sort -u
11
```

- 主表数据行 = 13（页 1–13 全覆盖）；每行 9 列（竖线分隔为 11 段）一致，七要素无空格、无空单元格。
- 测试映射列明确标「证据 `缺`」的共 4 页（登录 / 首页账单 / 手动记账 / 交易详情），完整缺口清单见 §4（9 类，转交 R0-T22）。
- 截图识别记账（页 3）按 PRD §5.2 记 `N/A + PRD:326-333`，为「有理由的 N/A」，不计入缺口。
