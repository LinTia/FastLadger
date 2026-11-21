# B4 · 窄屏表格隐藏规则影响面（候选 CSS，全部未应用）

> 草案状态：未应用。本文件不含任何已生效改动，应用须先获对应授权。

## 1. 结论摘要

1. `server/web/dist/style.css:331` 是**全库唯一**一条 `nth-child` 隐藏规则（`grep -c "nth-child"` = 1），位于 `:327` 的 `@media (max-width: 640px)` 块内。
2. 该规则对**全站 9 张表**生效：**仅 1 张表（T1 交易表）符合设计意图，其余 8 张发生数据丢失**。
3. 表格**没有横向滚动容器**（`grep -n "overflow-x"` 仅 `:328` 的 `#topbar` 一处），因此"隐藏列"是当前唯一的窄屏适配手段；直接删规则会导致表格溢出而非滚动。

## 2. 规则原文与生效范围

```css
/* server/web/dist/style.css:327 */
@media (max-width: 640px) {
  /* :328 */ #topbar { padding: 8px 12px; gap: 8px; overflow-x: auto; }
  /* :331 */ th:nth-child(1), td:nth-child(1), th:nth-child(5), td:nth-child(5) { display: none; }
}
```

| 核验项 | 结果 |
|---|---|
| `grep -c "nth-child" server/web/dist/style.css` | `1`（全库唯一） |
| `grep -n "overflow-x" server/web/dist/style.css` | 仅 `:328`（`#topbar`），**表格无横向滚动容器** |
| `grep -c ":has(" server/web/dist/style.css server/web/dist/app.js` | 均为 `0`（`:has()` 全库当前**0 处使用**） |

## 3. 九张表逐表影响矩阵（行号为 `server/web/dist/app.js`）

| 编号 | 生成位置 | 所在页面 / 渲染函数 | 表头原文 | 列数 | 第 1 列内容 | 第 5 列内容 | ≤640px 实际后果 |
|---|---|---|---|---|---|---|---|
| T1 | `:443`（th 起 `:444`） | 交易列表 / `renderTable()`（`:436`） | `["", "日期", "类型", "账户", "分类", "备注/商户", "金额", ""]` | 8 | 复选框（`:451–457` `input.type=checkbox`） | 分类 | 隐藏复选框列 + 分类列 → **唯一符合设计意图的表** |
| T2 | `:696` | 账户对账 / `renderAccounts`（`:678`） | `["账户","类型","余额","操作"]` | **4** | 账户名（`:701` `tdN.textContent=a.name`、`:722` `tr.append(tdN,…)`） | 不存在 | **账户名整列丢失** |
| T3 | `:794` | 回收站 / `renderRecycle`（`:782`） | `["删除时间","类型","账户","备注","金额",""]` | 6 | 删除时间 | 金额 | **删除时间 + 金额双列丢失** |
| T4 | `:918` | 报表·分类排行 / `renderReport`（`:835`） | `["分类排行","笔数","金额","占比"]` | **4** | 分类名 | 不存在 | **分类名丢失 → 整表失去可读性** |
| T5 | `:1331` | 投资·持仓 / `renderInvest`（`:1277`） | `["标的","数量","成本","现价","市值","浮动盈亏",""]` | 7 | 标的 | 市值 | **标的 + 市值丢失** |
| T6 | `:1392` | 投资·标的库 / `renderInvest`（`:1277`） | `["名称","类型","市场","代码",""]` | 5 | 名称 | 占位空 | **名称丢失** |
| T7 | `:1614` | 设置·API Token / `renderSettings`（`:1549`，`loadTokens()` `:1604` 内） | `["备注","创建时间","最近使用","状态",""]` | 5 | 备注 | 占位空 | 备注丢失 |
| T8 | `:1741` | 家庭成员 / `renderMembers`（`:1667`） | `["成员姓名","角色","可见性",""]` | **4** | 成员姓名（`:1746` `tdN.textContent=mb.username`） | 不存在 | **成员姓名丢失** |
| T9 | `:1844` | 投资账户详情·持仓明细 / `renderAccountDetail`（`:1802`） | `["标的","数量","成本","现价","市值","浮动盈亏"]` | 6 | 标的 | 市值 | **标的 + 市值丢失** |

**统计口径**：9 表中仅 **T1 符合设计意图**；其余 **8 张表全部发生数据丢失**。

### 3.1 分级

| 分级 | 表 | 说明 |
|---|---|---|
| 严重（丢失语义主键，界面不可用） | T2、T4、T8 | 均为 4 列表，只剩非标识字段 → 用户无法辨认行 |
| 严重（丢失关键数值） | T3、T5、T9 | 丢失金额 / 市值等关键数值列 |
| 中等（丢失辅助字段） | T6、T7 | 丢失名称 / 备注等辅助字段 |
| 符合设计意图 | T1 | 复选框列 + 分类列为有意裁剪 |

其中丢失"语义主键"的共 6 张（T2 账户名、T3 删除时间+金额、T4 分类名、T5 标的+市值、T8 成员姓名、T9 标的+市值），丢失辅助字段的 2 张（T6 名称、T7 备注）。

## 4. 上一版枚举的更正

1. **措辞更正**：上一版 `docs/FastLedger-R0-Owner决策与WorkBuddy执行准备清单.md:83` 把 `app.js:1614` 描述为「备注模板 5 列」，**实际是设置页的 API Token 列表**（`loadTokens()` `:1604` → `GET /users/me/tokens`，列头为 备注 / 创建时间 / 最近使用 / 状态）。该措辞错误**不影响**上一版"仅账户表恢复"的结论方向，但影响面枚举不完整。
2. **影响面缺口**：上一版只列了"≥5 列表格"（`Owner清单:83` 原文为「其余 ≥5 列表格**例如**…」），**漏掉了 4 列表格（T2 / T4 / T8）的首列同样被 `td:nth-child(1)` 隐藏**这一更严重的情形；同时低估了 T3 金额、T4 分类名、T5 / T9 市值、T8 成员姓名的实际损失程度。
3. **结论方向不变**：上一版"需把全局裸选择器收窄"的判断仍成立；本轮据实把影响面由"部分 ≥5 列表"更正为"9 张表全量登记、8 张发生丢失"。

## 5. 候选方案（**禁止应用**，全部未应用）

### 候选 A（推荐，纯 CSS，最小止血）

改动位置：`server/web/dist/style.css:331` 单条规则。用 `:has(> thead > tr > th:nth-child(8))` 收窄到 8 列交易表（全站仅 T1 为 8 列）。

```diff
-  th:nth-child(1), td:nth-child(1), th:nth-child(5), td:nth-child(5) { display: none; }
+  table:has(> thead > tr > th:nth-child(8)) > thead > tr > th:nth-child(1),
+  table:has(> thead > tr > th:nth-child(8)) > thead > tr > th:nth-child(5),
+  table:has(> thead > tr > th:nth-child(8)) > tbody > tr > td:nth-child(1),
+  table:has(> thead > tr > th:nth-child(8)) > tbody > tr > td:nth-child(5) { display: none; }
```

| 项 | 内容 |
|---|---|
| 覆盖范围 | 只影响 8 列交易表（T1）；T2–T9 恢复首列可见 |
| 优点 | 纯 CSS，单条规则，不动 `app.js`；`controls.json` / runner / fixture 全不触碰 |
| 风险 | `:has()` 全库**当前 0 处使用**，属**首次引入**；浏览器支持 Chrome 105+ / Safari 15.4+ / Firefox 121+，须在验收时以 390 / 641 / 1280 三视口实测确认；恢复后其余 8 表在 390px 是否横向溢出**未验证**（因无滚动容器）→ **须运行时回归后才能确认无新问题** |
| 需不需要额外授权 | 需要（`style.css` 修改属 Owner 样式授权，OD-03 同源；且影响面已扩大，须重新计算授权范围） |

### 候选 B（无 `:has()` 依赖，改走横向滚动）

改动位置：删除 `:331` 规则，改为表格容器可横向滚动。

```diff
-  th:nth-child(1), td:nth-child(1), th:nth-child(5), td:nth-child(5) { display: none; }
+  .card table { display: block; overflow-x: auto; white-space: nowrap; -webkit-overflow-scrolling: touch; }
```

| 项 | 内容 |
|---|---|
| 覆盖范围 | 全部 9 张表，改为横向滚动 |
| 优点 | 不依赖 `:has()`，兼容性最好；保留全部列 |
| 风险 | `display:block` 改变表格盒模型，列宽与对齐需三视口回归；`white-space:nowrap` 可能致单元格不换行；**未验证** |
| 需不需要额外授权 | 需要（同候选 A） |

### 候选 C（结构性根治，需另获授权）

改动位置：`app.js` 给交易表加语义类名 + `style.css` 规则限定该类名。

```diff
# app.js（示意，仅示意，未获授权，不得实施）
-  const tbl = document.createElement("table");
+  const tbl = document.createElement("table"); tbl.className = "tx-table";

# style.css:331（示意）
-  th:nth-child(1), td:nth-child(1), th:nth-child(5), td:nth-child(5) { display: none; }
+  .tx-table th:nth-child(1), .tx-table td:nth-child(1),
+  .tx-table th:nth-child(5), .tx-table td:nth-child(5) { display: none; }
```

| 项 | 内容 |
|---|---|
| 覆盖范围 | 语义定位，未来新增表格不误伤 |
| 优点 | 无 `:has()` 依赖、无盒模型变更，最稳健 |
| 风险 | 需改 `app.js`（业务代码），**不属 AUTH-A 也不属 AUTH-B**，须**单独授权** |
| 需不需要额外授权 | **是（单独业务代码授权）**；本 diff **仅示意，未获授权，不得实施** |

## 6. 推荐

- **止血**：候选 A（但必须附运行时回归前置：三视口 + 溢出检测）。
- **彻底方案**：候选 C，排后续单独授权。
- **候选 B**：仅在候选 A 的 `:has()` 支持不满足基线时启用。

## 7. 状态声明

**本文件任何 diff 都未应用**。应用需 Owner 授权（OD-03 同源，但影响面已由"部分表格"扩大为"9 表 8 失"，须重新计算授权范围）。

## 验收说明

1. 三视口截图（390 / 641 / 1280）覆盖 T1–T9 九张表。
2. T2 / T4 / T8 首列 `getComputedStyle(td:first-child).display === "table-cell"`。
3. T1 仍隐藏复选框列与分类列（第 1 / 第 5 列 `display === "none"`）。
4. 测试统计 `164/4/59/1/228` 不变。
5. 无横向溢出：`document.documentElement.scrollWidth <= document.documentElement.clientWidth`。

