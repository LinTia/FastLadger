# R0 E2E 运行器说明

`tools/r0/e2e/run.js` 是 R0 运行时验收入口：启动隔离的本地服务（临时 SQLite + 随机端口 +
每次新生成的 JWT 密钥）、合成账号与合成数据，再用真实浏览器执行各套件。证据写入
`R0_ARTIFACT_DIR`（默认 `<repo>/artifacts/r0-e2e`）下带时间戳的运行目录。

## 套件与分工

| 套件 | 命令 | 定位 | 门禁性质 |
|---|---|---|---|
| A | `node run.js --suite a` | 11 项状态/权限/同步场景 | 门禁型：任一 FAIL 或 BLOCKED 即退出码 1 |
| C | `node run.js --suite c` | 228 条逐控件运行时验收（对应 `fixtures/controls.json`） | **报告型**：PASS/FAIL/BLOCKED/N/A 分布本身就是结论，允许既有产品缺口 FAIL |
| D | `node run.js --suite d` | 第七次 4 项已确认业务缺陷的回归（含 I-TXNS-01..19 逐条重跑） | 门禁型：任一断言失败即 job 失败 |
| all | `node run.js --suite all` | **A + D + C 全部适用套件**，按 `a → d → c` 顺序执行 | 由各套件结果汇总决定 |

排序理由：D 是门禁型回归，放在报告型 C 之前执行，避免 C 的报告型写操作（编辑/删除首笔交易、
改成员角色、退出登录、生成 Token 等）污染 D 的回归数据；C 作为报告型放最后。

**B 不在本运行器 suite 体系内**：「分类解析合成测试 + 22 笔分类映射回归审计」由
`.github/workflows/r0-e2e.yml` 的独立 step 执行，具体命令为：

```bash
node --test tools/r0/test-resolve-category.js
R0_SERVER_BIN=<repo>/server/fastledger node tools/r0/e2e/scripts/reseed-audit.js
```

因此 `--suite all` **不含 B**，也不代表 B 无需运行或已通过；CI 中 A / D / C / B 分别以独立
step 运行（见 workflow 的「分类解析合成测试」「分类映射回归审计」「包 A」「包 D」「包 C」）。
本轮 P2 之前 `all` 曾被实现为仅 `['a', 'c']`（漏掉 D），头部注释还写着「D 需显式
`--suite d` 运行」，现已统一为 A + D + C。

### 退出码语义

- `0`：无 FAIL 且 A 无 BLOCKED；
- `1`：有 FAIL，或 A 有 BLOCKED；
- `2`：环境错误（服务/浏览器启动、bootstrap 失败等）。

C 是报告型且存在既有 FAIL（例如主报告已登记的产品缺失），因此
**`node run.js --suite all` 通常退出码 1，属预期行为，不代表 D 失败**；判断 D 是否通过应看
`report.json` 中 `D:FAIL` 计数或日志里的「D 汇总」，而不是看 `all` 的整体退出码。
同理，CI 中「包 C」单步设置了 `continue-on-error: true`，**该 step 绿色不等于 228 条控件
全部通过**，只表示报告型步骤已执行完成；228 条的结论必须读 `report.json` /
`C/controls-summary.json` 的分布。

不得为了让 `all` 变绿而放宽 C 的断言或跳过 C。

## 本地运行

```bash
# 需先构建服务端二进制（前端 app.js 内嵌于二进制）
source <tools>/genv.sh && go build -o server/fastledger.exe ./cmd/server

cd tools/r0/e2e
R0_SERVER_BIN=<repo>/server/fastledger.exe \
R0_BROWSER_CHANNEL=msedge \
R0_ARTIFACT_DIR=<repo>/artifacts/r0-e2e \
node run.js --suite all
```

| 环境变量 | 说明 |
|---|---|
| `R0_SERVER_BIN` | 服务二进制路径（默认 `server/fastledger[.exe]`） |
| `R0_BROWSER_CHANNEL` | 本地用 `msedge`；CI 不设（用自带 chromium） |
| `R0_ARTIFACT_DIR` | 证据输出根目录（默认 `<repo>/artifacts/r0-e2e`） |
| `R0_DATA_DIR` | 复用固定实例目录（默认临时目录，运行结束即丢弃） |

## 清单外独立登记（P2）

C 套件 `I-ACCT` 组另产出两个**不占 228 清单任何编号**的独立观察文件，它们不写入 `R`
（`R` 的 key 会进入 228 条计数）、不改变清单分布：

- `C/I-ACCT/visibility-observation-independent.json`：`OBS-ACCT-VIS-390`，记录 390px 视口下
  账户名称列的 `visibleInnerText` / `firstTdDisplay`。**第四项「新增账户视觉可见性」仍为
  FAIL / 未闭环，待 Owner 裁决；本轮不改 `style.css`**。
- `C/I-ACCT/delete-disappearance-independent.json`：`OBS-ACCT-DEL-390`，用
  `td:first-child` 的 `textContent` 对「删除后从列表消失」做真实观测（原 `innerText` 断言
  在 390px 隐藏列下恒真，已确认为假阳性）。

I-ACCT 编号与清单定义的全量错位审计见
`docs/r0-evidence-7th-fix/p2/iacct-mapping-audit.md`。
