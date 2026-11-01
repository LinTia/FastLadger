# FastLedger 全局主题 Web 前端设计规范（Design Tokens v1.0）

| 项目 | 内容 |
|---|---|
| 文档名称 | FastLedger 全局主题 Web 前端设计规范 |
| 版本 | v1.0 |
| 日期 | 2026-11-02 |
| 适用范围 | Web 端（go:embed 托管）、新版可交互原型（`prototype-v2/`）、Flutter 客户端主题映射参考 |
| 取证来源 | `prototype/assets/index-VjxVMuh2.css` 的 `:root` 主题变量（Tailwind v4 + shadcn 风格主题）；`docs/FastLedger-产品需求文档-PRD.md`；`server/web/dist/style.css` 现行实现 |
| 配套交付 | `docs/design-tokens.json`（机器可读 token）、`prototype-v2/index.html`（规范示范原型） |

> 本规范是"全局主题"的单一事实来源（Single Source of Truth）。所有 Web/客户端界面的颜色、字体、间距、圆角、阴影、动效一律引用本 token，禁止在业务代码里硬编码色值。后续如需换肤（深色模式），只需覆盖 `:root` 同名变量。

---

## 1. 设计原则

1. **克制**：记账工具是"每天要看的东西"，视觉噪音必须最小。大面积暖白底 + 白卡片，主色只出现在动作与强调处。
2. **语义化色彩**：颜色承载含义——收入绿、支出红、转账灰、危险红、警告橙，全局唯一口径，不因页面而异。
3. **移动优先**：原型与 PRD 均以手机竖屏为主形态；Web 端是同一设计语言在宽屏上的延展（底部 Tab → 顶部导航）。
4. **快而有感**：微反馈全部走 `0.15s / cubic-bezier(.4,0,.2,1)`，快到不拖沓、慢到能感知；`prefers-reduced-motion` 下全部关闭。
5. **数据可读性**：金额一律等宽数字（`tabular-nums`）右对齐，列表数字列不跳动。

---

## 2. 配色方案

### 2.1 核心主题色（shadcn 语义层）

| Token | 值 | 用途 |
|---|---|---|
| `--background` | `#f9f8f5` | 页面底色（暖白，非纯白，降低刺眼感） |
| `--foreground` | `#1f2e26` | 主文字（带绿相的深墨绿，与主色同族） |
| `--card` | `#ffffff` | 卡片/面板底 |
| `--card-foreground` | `#1f2e26` | 卡片内主文字 |
| `--primary` | `#34b262` | 品牌主色：主按钮、选中态、链接、进度条 |
| `--primary-foreground` | `#ffffff` | 主色上的文字 |
| `--secondary` | `#eaf6ee` | 次级强调底（主色 10% 左右的浅绿） |
| `--secondary-foreground` | `#288a4c` | 次级强调文字（主色加深，对比达标） |
| `--muted` | `#f2f8f4` | 弱底色：悬停底、代码块、次级按钮 |
| `--muted-foreground` | `#758a80` | 弱文字：辅助说明、占位、时间戳 |
| `--accent` | `#eaf6ee` | 强调底（与 secondary 同值，导航选中底） |
| `--accent-foreground` | `#288a4c` | 强调文字 |
| `--border` | `#e3e8e6` | 边框/分隔线 |
| `--input` | `#e3e8e6` | 输入框描边 |
| `--ring` | `#34b262` | 键盘焦点环 |
| `--popover` | `#ffffff` | 弹层底色 |
| `--destructive` | `#ef4343` | 危险：删除、报错、超支 |

### 2.2 功能语义色（收支与状态口径 —— 中国习惯：红支出、绿收入）

| Token | 值 | 用途 |
|---|---|---|
| `--success` | `#31a545` | 成功提示、收入金额 |
| `--success-foreground` | `#f2fdf1` | 成功底上的文字 |
| `--warning` | `#ed6e0c` | 警告：预算"接近"（≥80%）、待处理 |
| `--warning-foreground` | `#fff6eb` | 警告底上的文字 |
| `--info` | `#2463eb` | 信息提示、中性徽标 |
| `--info-foreground` | `#f5f7ff` | 信息底上的文字 |
| `--expense` | `#ef4343`（= destructive） | **支出**金额、超支状态 |
| `--income` | `#34b262`（= primary） | **收入**金额 |
| `--transfer` | `#758a80`（= muted-foreground） | **转账**（不计收支，用弱色） |

> **铁律**：红=支出/危险、绿=收入/成功、灰=转账。这条口径全站唯一，与 `server/web/dist/style.css` 的 `.income/.expense/.transfer` 一致。图表配色同样不得反转。

### 2.3 中性色阶（Feishu 风 neutral，用于文字分层与灰阶图表）

| Token | 值 | 典型用途 |
|---|---|---|
| `--neutral-50` | `#f5f6f7` | 最浅灰底 |
| `--neutral-100` | `#f2f3f5` | 表头底、浅分隔 |
| `--neutral-300` | `#dee0e3` | 禁用描边 |
| `--neutral-500` | `#bbbfc4` | 禁用文字 |
| `--neutral-600` | `#8f959e` | 占位文字 |
| `--neutral-700` | `#646a73` | 次要文字 |
| `--neutral-900` | `#2b2f36` | 强文字（非主题页） |
| `--neutral-950` | `#1f2329` | Toast 深底 |

### 2.4 图表与分类色板

**趋势图序列色**（来自原型 `--chart-1..5`，同绿相家族渐变，保证图表整体和谐）：

| Token | 值 | 用途 |
|---|---|---|
| `--chart-1` | `#34b262` | 首要序列（收入/主指标） |
| `--chart-2` | `#34b2a1` | 第二序列 |
| `--chart-3` | `#3484b2` | 第三序列 |
| `--chart-4` | `#3444b2` | 第四序列 |
| `--chart-5` | `#6234b2` | 第五序列 |

**分类图标色板**（12 色循环：餐饮、交通、购物、居住、娱乐、医疗、教育、旅游、人情、宠物、工资、理财……）取 Tailwind 500 阶，仅用于分类图标底/前景，不参与按钮与状态：

`green #32a645` · `blue #5083fb` · `orange #ed6d0c` · `purple #9f6ff1` · `teal #10a893` · `pink #df58a5` · `red #f54a45` · `amber #e5ce00` · `cyan #1295ca` · `indigo #757df0` · `violet #cf5ecf` · `lime #7b9207`

配套 50 阶浅底（如 `green-50 #e4fae1`、`blue-50 #f0f4ff`）用作分类圆形图标的背景色。完整色阶见 `docs/design-tokens.json`。

### 2.5 遮罩与选中

| Token | 值 | 用途 |
|---|---|---|
| `--bg-mask` | `rgba(0,0,0,.4)` | 对话框遮罩 |
| `--bg-text-selection` | `rgba(20,86,240,.3)` | 文本选区 |
| `--line-divider` | `rgba(31,35,41,.15)` | 模块间分隔线（可替代 `--border`） |

---

## 3. 字体体系

### 3.1 字体族

```css
--font-sans: "Noto Sans SC", -apple-system, BlinkMacSystemFont, "Segoe UI",
             "PingFang SC", "Microsoft YaHei", Roboto, sans-serif;
```

- 中文正文与数字统一 **Noto Sans SC**（原型 `@import` 引入 400/500/600/700 四字重；离线部署时回退到 PingFang/雅黑，字号节奏不变）。
- 金额、表格数字启用 `font-variant-numeric: tabular-nums`；金额字体不使用衬线与手写体。

### 3.2 字号阶梯（Web 端 15px 基准）

| Token | 值 | 行高 | 用途 |
|---|---|---|---|
| `--text-xs` | 12px / `.75rem` | 1.333 | 徽标、时间戳、表头 |
| `--text-sm` | 14px / `.875rem` | 1.428 | 次要正文、表格单元格、辅助说明 |
| `--text-base` | 15px（原型 16px/1rem） | 1.55 | 正文、列表主行 |
| `--text-lg` | 18px / `1.125rem` | 1.55 | 卡片标题、页面区块标题 |
| `--text-xl` | 20px / `1.25rem` | 1.4 | 页面大标题 |
| 金额大数字 | 22–28px / 600 | 1.2 | 首页本月支出/收入/结余 |

### 3.3 字重

| Token | 值 | 用途 |
|---|---|---|
| `--font-weight-normal` | 400 | 正文 |
| `--font-weight-medium` | 500 | 按钮、导航、行内强调 |
| `--font-weight-semibold` | 600 | 卡片标题、金额 |
| `--font-weight-bold` | 700 | 品牌名、大数字 |

---

## 4. 间距与栅格

### 4.1 间距基数

原型 `--spacing: .25rem`（4px），全部间距为 4 的倍数：

| Token | 值 | 用途 |
|---|---|---|
| `space-1` | 4px | 图标与文字微间距 |
| `space-2` | 8px | 表单元素间距、紧凑内边距 |
| `space-3` | 12px | 列表行内边距、卡片小内边距 |
| `space-4` | 16px | 卡片标准内边距、栅格列间距 |
| `space-5` | 20px | 区块间距 |
| `space-6` | 24px | 卡片宽松内边距（桌面） |

### 4.2 布局栅格

| 场景 | 规则 |
|---|---|
| 移动端（<640px） | 单列；页面左右留白 10–12px；底部 Tab 高 56px + safe-area |
| 平板/桌面（≥640px） | 内容容器 `max-width: 980px` 居中；首页概览 3 列网格（支出/收入/结余）；报表页 2 列 |
| 卡片列表 | 卡片间距 16px；卡片内行高 ≥44px（触控目标） |
| 底部导航 | 4 个 Tab：首页 / 记一笔(中央凸起) / 报表 / 我的；激活态为主色药丸 |

---

## 5. 圆角与阴影

### 5.1 圆角

| Token | 值 | 用途 |
|---|---|---|
| `--radius-xs` | 2px | 徽标内元素 |
| `--radius` | 8px（原型 `--radius: .5rem`） | 按钮、输入框、小卡片 |
| `--radius-lg` | 12px | 标准卡片、表格容器 |
| `--radius-xl` | 14px | 对话框 |
| `--radius-full` | 999px | 药丸导航、徽标、头像 |

### 5.2 阴影（极轻，来自原型 `--shadow-*`）

| Token | 值 | 用途 |
|---|---|---|
| `--shadow-sm` | `0px 2px 12px 0px rgba(0,0,0,.02)` | 顶栏、静态卡片 |
| `--shadow` | `0px 2px 12px 0px rgba(0,0,0,.05), 0px 1px 2px -1px rgba(0,0,0,.05)` | 悬浮卡片 |
| `--shadow-md` | `0px 2px 12px 0px rgba(0,0,0,.05), 0px 2px 4px -1px rgba(0,0,0,.05)` | 下拉、Toast |
| `--shadow-lg` | `0px 2px 12px 0px rgba(0,0,0,.05), 0px 4px 6px -1px rgba(0,0,0,.05)` | 对话框、抽屉 |

> 阴影整体透明度 ≤5%，靠 `--border` 描边区分层次；这是原型"轻盈感"的关键，不得加深。

---

## 6. 组件样式

### 6.1 按钮

| 变体 | 规格 |
|---|---|
| **primary** | 底 `--primary`、字白、radius 8px、padding 7px 16px、字重 500；hover 加深为 `#2fa257`；active `scale(.98)`；disabled 透明度 .5 + not-allowed |
| **secondary/ghost** | 透明底 + `--border` 描边 + 字 `--foreground`；hover 底变 `--muted` |
| **destructive** | 底 `--destructive` 字白；hover `#e03535`；用于删除等不可逆动作 |
| **pending** | 提交中：内容换 spinner + 禁用，防止双击重复提交（PRD §9.2） |
| **focus** | `:focus-visible` 外圈 2px `color-mix(primary 45%)`，offset 2px |

### 6.2 表单控件

- 输入框/选择器：白底、`--input` 描边、radius 8px、padding 7px 10px；hover 描边加深；**focus 描边变主色 + 3px 主色 18% 光晕**（`box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent)`）。
- 校验失败：描边变 `--destructive`，下方 12px 红字提示，输入不抖动。
- checkbox：`accent-color: var(--primary)`，15px。
- 标签在输入框上方，13px `--muted-foreground`。
- 金额输入：数字键盘（`inputmode="decimal"`）、右侧币种"¥"、`tabular-nums`。

### 6.3 卡片

- 白底 + `--border` 描边 + radius 12px + `--shadow-sm`；内边距 18px（移动 14px）。
- 进入动画 `fadeUp .22s`（透明度 0→1，translateY 6px→0）。
- 概览数字卡：标题 13px 弱色 + 数字 22px/600 等宽。

### 6.4 导航

- **底部 Tab（移动）**：白底、上描边；图标+文字，激活项主色（图标底变 `--secondary` 药丸）。
- **顶部导航（桌面/Web）**：白底 sticky + 下描边 + `--shadow-sm`；品牌字重 700 主色；导航项 14px 药丸 hover `--muted`、active `--secondary` 底 + 主色字。
- 中央"记一笔"主按钮：主色圆形/药丸凸起，比其他 Tab 大一号。

### 6.5 徽标与金额

- 类型徽标：胶囊 radius 999px、12px 字、底色为对应语义色 10–12% 混白（`color-mix`）。
- 金额：收入 `+` 绿、支出红、转账灰（无符号只标"转"）；右对齐 `tabular-nums`；千分位；负余额红色（`--balance-neg`）。

### 6.6 列表与表格

- 交易列表按日期分组：组头 13px 弱色 + 当日小计。
- 行内布局：左侧 32px 分类圆图标（分类色板浅底+深字）→ 主行（商户/备注 15px）+ 副行（账户·渠道·时间 12px 弱色）→ 右侧金额。
- Web 表格：行 hover `--muted` 60% 透明底，行高 ≥44px，数字列右对齐；分隔线 `--border`。

### 6.7 对话框

- radius 14px、`--shadow-lg`、进入动画 `.18s`（透明度+ translateY 4px + scale .95→1）；遮罩 `rgba(0,0,0,.4)` + 2px 背景模糊。
- 结构：标题 16px/600 → 说明 14px 弱色 → 表单/正文 → 右下按钮组（主按钮在右）。
- 删除确认：destructive 主按钮 + 明确后果文案（PRD §4.4 二次确认）。
- 关闭方式：遮罩点击、Esc、取消按钮三通道。

### 6.8 Toast

- 底部居中（移动端 bottom 18px）、radius 10px、深底白字 14px、`--shadow-md`；进入 `.2s` 上滑淡入，1.8–2.5s 自动退出。
- 变体：`ok` 深绿底 `#234d33` / `err` 红底 `--destructive` / `info` 蓝灰底 `#2f4858`；左侧配语义图标。

### 6.9 进度与状态（预算）

- 进度条：高 8px、radius 999px、底 `--muted`；填充色随状态切换——**正常 `--primary`、接近(≥80%) `--warning`、超支 `--destructive`**（PRD §4.7）。
- 状态徽标随进度条同色。

### 6.10 空状态与骨架屏

- 空状态：大图标（34px）+ 一句话说明 + 主行动按钮；垂直居中，`fadeUp .25s`。
- 加载：列表用骨架屏（`shimmer` 1.1s 渐变扫光，`#eceff1 → #f6f7f8`）；按钮用 spinner；禁止白屏静默。

### 6.11 开关（Switch）

- 关闭：`--neutral-300` 底白球；开启：`--primary` 底白球；宽 40px 高 22px，过渡 .15s。
- **可见性开关旁必须常驻隐私提示**："关闭互看仅对未来同步生效；成员此前已查看或已导出的数据，服务端无法收回。"（PRD §7.3 裁决）

---

## 7. 交互状态与动效

### 7.1 标准交互态

| 状态 | 规格 |
|---|---|
| hover | 底色/描边微变，`.15s` |
| active/pressed | `scale(.98)` + 底色加深，`.1s` |
| focus-visible | 2px 主色 45% 外圈（键盘可达性） |
| disabled | opacity .5 + cursor not-allowed，不响应 hover |
| loading | spinner / 骨架屏；按钮禁用防重 |
| 选中 | 主色字 + `--secondary` 药丸底 |

### 7.2 动效 token

| Token | 值 | 用途 |
|---|---|---|
| `--ease` | `cubic-bezier(.4,0,.2,1)` | 默认缓动（标准） |
| `--ease-out` | `cubic-bezier(0,0,.2,1)` | 进入类动画 |
| 时长 | `.1s`（按压）/ `.15s`（悬停、开关）/ `.18s`（对话框）/ `.2–.25s`（页面淡入、Toast） | 全站统一 |
| 页面切换 | 内容区 `fadeUp`（透明度+上移 6px） | 不做整页横向滑动 |

### 7.3 动画清单

`fadeUp`（页面/卡片进入）、`dialogIn`（对话框）、`toastIn/toastOut`、`shimmer`（骨架屏）、`fadeOut`（删除行）、`spin`（spinner）、`pulse`（OTA/识别中）。

### 7.4 无障碍与兼容

- `@media (prefers-reduced-motion: reduce)`：关闭全部 animation/transition，`scroll-behavior: auto`。
- 触控目标 ≥44×44px；正文对比度 ≥4.5:1（`#758a80` on `#fff` ≈ 4.6:1 达标，弱文字只用于辅助信息）。
- 全部交互可用键盘到达（Tab 顺序 + focus-visible）；对话框 Esc 可关。
- 零外部请求：字体与图标本地化，`@import` 仅在可访问外网时生效，回退字体栈保证离线一致。

---

## 8. 信息架构与页面清单（对应 PRD §3.1 / 原型 13 页）

### 8.1 导航结构

```
底部 Tab（移动）/ 顶部导航（Web）
├── 首页（本月概览 + 账单流 + 筛选 + 记一笔入口）
├── 记一笔（中央主按钮 → 类型切换：支出/收入/转账 + 截图识别入口）
├── 报表（月度趋势 + 分类占比 + 时间/账本切换）
└── 我的
    ├── 账户管理（15 类资金账户、余额、多币种）
    ├── 分类管理（两级分类）
    ├── 预算（总预算 + 分类信封）
    ├── 投资资产（持仓、实物资产、净资产）
    ├── 家庭成员（成员/角色/可见性开关 + 隐私提示）
    ├── 数据导出（CSV/JSON、筛选、预览）
    └── 设置（资料、API Token、服务器信息、备份）
```

### 8.2 关键页面交互规格（详见 `prototype-v2/index.html`）

| 页面 | 必备交互 |
|---|---|
| 登录 | 预填演示账号、按钮 pending、错误红字、成功后整页过渡进首页 |
| 首页 | 概览三卡实时联动、按日分组账单、行点击进详情、筛选条、骨架屏 |
| 记一笔 | 类型分段切换（支出/收入/转账，转账显双账户）、必填校验红字、提交 pending → toast → 返回首页顶部插入 |
| 截图识别 | 三级漏斗演示（端侧 OCR → 规则解析 → LLM 兜底开关）→ 确认弹窗预填 → 确认入账；**绝不自动入账** |
| 交易详情 | 全字段展示、编辑入口、删除二次确认（destructive）→ toast + 余额/概览回滚 |
| 账户 | 账户卡（余额、类型徽标）、净资产汇总、正负余额颜色 |
| 分类 | 大类卡片 → 子类列表，分类色板图标 |
| 预算 | 总进度条 + 分类信封列表，正常/接近/超支三态色 |
| 报表 | 月度柱状趋势（chart-1 主色）+ 分类占比环形图（conic-gradient）+ 月份切换 |
| 家庭成员 | 角色徽标（owner/admin/member）、可见性开关 + 常驻隐私提示 |
| 设置 | API Token 管理（生成/复制/撤销）、备份入口、退出登录 |

---

## 9. Token 命名与落地约定

1. **CSS 变量层**：Web 端在 `:root` 定义本规范同名变量（`server/web/dist/style.css` 已对齐，变量别名映射：`--ink=--foreground`、`--bg=--background`、`--line=--border`、`--danger=--destructive`、`--muted-fg=--muted-foreground`、`--primary-soft=--secondary`）。
2. **机器可读**：`docs/design-tokens.json` 按分组导出，前端可生成 CSS/Flutter `ThemeData`/Tailwind config。
3. **Flutter 映射**：`primary→ColorScheme.primary`、`background→scaffoldBackgroundColor`、`destructive→error`、radius 8/12→`ShapeBorder`，字重与行高同表。
4. **变更流程**：改 token 必须先改本文档 + JSON，再同步三端实现；禁止实现侧先行。

---

*本规范 v1.0 取证自可交互原型构建产物；如原型后续从在线工程重新导出并调整主题，以导出产物的 `:root` 变量为准回填本表。*
