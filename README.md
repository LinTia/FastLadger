# FastLedger · 家庭智能记账与资产管理系统

一款**自托管**（部署在家庭 NAS / Docker）的家庭智能记账系统，核心体验是 **截图 → 本地识别 → 人工确认 → 家庭共享**，并提供统一的家庭资产/负债与净资产视图。

> 数据在自己手里 · 识别优先在端侧完成 · 多端协同 · 家庭共享 · 资产全景

## 产品特性

- **截图秒记**：手机/电脑截图后，端侧 OCR + 规则解析，预填确认即可入账；识别结果必须人工确认。
- **隐私优先**：图片不上传，识别默认在端侧完成；云端大模型仅作可选兜底（只传文本）。
- **多端协同**：Windows / Android / iOS 客户端（Flutter）+ Web 快捷浏览（无 OCR）。
- **家庭共享**：多账本、多成员（2 人起步，可扩展到 5+）、角色与可见性可控。
- **账目不重复**：资金账户与支付渠道分离，正确处理微信/支付宝/信用卡的同一笔消费。
- **资产全景**：现金、信用卡、多券商、基金、房产、车辆、黄金统一到家庭净资产；预留自动行情。

## 仓库导航

| 路径 | 内容 | 用途 |
|---|---|---|
| [`prototype/`](prototype/) | 完整可交互原型（13 个核心页面，**零外部请求、可离线**） | 看产品长什么样、怎么交互，**可直接在浏览器运行**；**公网在线版：[点此直接体验](https://a38ce1ad4e917321e.app.workbuddy.host/?sharecode=BPE6OFTPKE)**（由 WorkBuddy 托管，无需本地构建） |
| [`docs/FastLedger-产品需求文档-PRD.md`](docs/FastLedger-产品需求文档-PRD.md) | 产品需求文档（PRD） | 做什么、为什么、范围与验收 |
| [`docs/FastLedger-落地方案.md`](docs/FastLedger-落地方案.md) | 软件开发 8 环节落地方案 | 整体技术方案、数据模型、API、版本规划 |
| [`docs/FastLedger-新手开发完整指南.md`](docs/FastLedger-新手开发完整指南.md) | 新手开发完整指南（含可照敲代码骨架） | 应届生照着做即可开发 |
| [`docs/FastLedger-枚举字典.md`](docs/FastLedger-枚举字典.md) | 统一枚举字典（唯一权威来源） | 各端枚举值对齐 |
| [`docs/ezbookkeeping-代码导读.md`](docs/ezbookkeeping-代码导读.md) | 开源参考项目 ezbookkeeping 代码导读 | 可借鉴的设计与实现 |
| [`gpt/v1.4/`](gpt/v1.4/) | GPT 定向审核与真实运行核验 | v1.4 §6 复核、规范字段意见及入口裁决 |
| [`docs/FastLedger-转账入口裁决.md`](docs/FastLedger-转账入口裁决.md) | 唯一 Create / 转账事务边界 | API、幂等、版本与接线的统一规则 |
| [`CHANGELOG.md`](CHANGELOG.md) | 更新日志 | 各版本变更记录 |
| [`workbuddy/第三方审阅者导读.md`](workbuddy/第三方审阅者导读.md) | 第三方审阅者导读 | **新来审阅 / 提意见者的对齐入口**（背景、迭代史、审阅约定） |
| [`workbuddy/FastLedger-文档评审-D.md`](workbuddy/FastLedger-文档评审-D.md) | 第四个审阅者 D 工程评审 | D-R01–D-R06（2×P1+4×P2），聚焦 Gin/WS/Flutter/OCR/同步/迁移删除恢复 |

### 文档分工（避免内容重复漂移）

| 文档 | 只回答 | 不负责 |
|---|---|---|
| PRD | 做什么、为什么、范围与验收 | 具体代码实现 |
| 落地方案 | 技术决策、架构、数据模型、API、版本规划 | 逐行代码教学 |
| 新手开发指南 | 怎么一步步写出来（可照敲骨架） | 重新讨论需求 |
| 枚举字典 | 枚举值的唯一权威 | 业务流程 |
| 原型 | 长什么样、怎么交互 | 数据持久化 |

> 规则：需求/范围以 **PRD** 为准；技术选型以**落地方案**为准；枚举值以**枚举字典**为准。三份文档冲突时，按此优先级，并在 CHANGELOG 记录。

## 快速开始（运行原型）

> **想直接体验，不用本地搭环境？** 打开公网在线版 👉 [https://a38ce1ad4e917321e.app.workbuddy.host/?sharecode=BPE6OFTPKE](https://a38ce1ad4e917321e.app.workbuddy.host/?sharecode=BPE6OFTPKE)（由 WorkBuddy 托管，含全部交互页面，演示账号已预填）。

原型是纯前端应用，无需后端：

```bash
cd prototype
python3 -m http.server 8765
# 浏览器打开 http://localhost:8765/
# 演示账号已预填（me / 123456），直接点登录
```

> 直接用 file:// 打开可能因浏览器安全策略导致模块加载受限，建议用上面的本地服务器方式。

## 技术栈（规划）

- **后端**：Go + Gin + xorm，SQLite 起步（预留 PostgreSQL/MySQL），目标常驻内存 < 150 MB。
- **客户端**：Flutter（Windows / Android / iOS）。
- **Web**：前端页面由后端 go:embed 托管。
- **部署**：Docker / docker-compose，Caddy 反代 + DDNS + 自动 HTTPS。

## 版本路线（摘要）

- **v0.1（MVP）**：手动 + 截图记账、账户/分类、基础报表、导出、多账本、2 人家庭、投资骨架。
- **v0.2**：5+ 成员、完整预算、Web 端完善。
- **v0.3**：投资自动行情（A股/港股/基金/黄金）。
- **v0.4**：完整资产负债与净资产、多币种汇总。

详见 PRD 与落地方案。

---

*本仓库为 FastLedger 的设计与原型资料汇总；正式代码工程将基于《新手开发完整指南》逐步实现。*
