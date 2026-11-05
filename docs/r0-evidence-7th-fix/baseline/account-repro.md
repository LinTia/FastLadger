# 第 4 项缺陷 · 修复前独立稳定复现（新增账户 POST 后页面不出现）

## 运行信息

- 采集时间：2026-11-05T09:16:27.306Z ~ 2026-11-05T09:17:01.838Z
- commit：`dadc9a3b9c567cad2bbad7e47361fb774365b949`；分支：`fix/r0-sixth-runtime-defects`
- 服务二进制：`C:/Users/47006/WorkBuddy/fastledger/repo_tmp/server/fastledger.exe`
- 浏览器：playwright-core 156.0.4314.58 channel=msedge
- 实例：http://127.0.0.1:56232（临时 SQLite，随机端口，与主仓库数据隔离）
- 复现方式：真实浏览器；每次迭代新建独立 context/page，共用同一临时实例；POST 用 `waitForResponse` 捕获真实响应，`GET /accounts` 用 Node 侧 API 回读，DOM 重绘用 `#view` 的 MutationObserver 记录（确定性信号，不使用固定 sleep 制造结论）。
- 判定口径：`viewTextContentHasName` = DOM 中存在该文本（含被 CSS 隐藏）；`viewInnerTextHasName` = 渲染文本中存在该文本。前者真/后者假 → 「DOM 有但不可见」；两者皆假 → 「API 有、DOM 无」。

## 三次复现

### 迭代 1 · `复现账户-1-1793870188428`

- POST：`POST /api/v1/books/1/accounts` → HTTP **200**；响应体：`{"code":0,"data":{"id":7,"bookId":1,"name":"复现账户-1-1793870188428","category":1,"currency":"CNY","balance":0,"parentId":0,"syncVersion":7,"deleted":false,"createdAt":1793870189,"updatedAt":1793870189},"msg":"ok"}`
- GET /accounts 回读：共 4 个账户，含目标名 = **true**；名称列表：`现金`, `复现账户-1-1793870188428`, `储蓄卡`, `信用卡`
- 提交前：`#view.childElementCount=1`，账户行数=3，`#acctDlg.open=false`
- 点击「＋ 新增账户」：匹配按钮数=1，点击后 `#acctDlg.open=true`
- 提交后：`#view.childElementCount=1`，账户行数=4，`#acctDlg.open=false`，`#acctErr=「」`，toast=「✓ 账户已创建」
- 提交后文本观测：`viewTextContentHasName=true`，`viewInnerTextHasName=false`，`bodyHasName=false`，叶子节点命中=1（其中隐藏=1）
- 提交后全部 `<td>` 文本（共 16 个）：`现金`[不可见]；`现金`；`¥7,987.66`；`编辑 删除`；`复现账户-1-1793870188428`[不可见]；`现金`；`¥0.00`；`编辑 删除`；`储蓄卡`[不可见]；`活期储蓄`；`¥-50.00`；`编辑 删除`；`信用卡`[不可见]；`信用卡`；`¥0.00`；`编辑 删除`
- `#view` 重绘记录（MutationObserver，共 1 次）：{rows=4,textHasName=true,innerTextHasName=false}
- POST 响应后观测到重绘 = true；6s 内目标名出现在可见文本 = false
- 刷新页面后：`#view.childElementCount=1`，账户行数=4，`viewTextContentHasName=true`，`viewInnerTextHasName=false` → 刷新后仍不可见
- **归类：DOM 有但不可见/文本被隐藏**
- console error/pageerror：(无)
- 网络日志（API，共 7 条）：GET /api/v1/books/1/accounts → 200；GET /api/v1/books/1/categories → 200；GET /api/v1/books → 200；POST /api/v1/books/1/accounts → 200；GET /api/v1/books/1/accounts → 200；GET /api/v1/books/1/categories → 200；GET /api/v1/books → 200

### 迭代 2 · `复现账户-2-1793870195582`

- POST：`POST /api/v1/books/1/accounts` → HTTP **200**；响应体：`{"code":0,"data":{"id":8,"bookId":1,"name":"复现账户-2-1793870195582","category":1,"currency":"CNY","balance":0,"parentId":0,"syncVersion":8,"deleted":false,"createdAt":1793870196,"updatedAt":1793870196},"msg":"ok"}`
- GET /accounts 回读：共 5 个账户，含目标名 = **true**；名称列表：`现金`, `复现账户-1-1793870188428`, `复现账户-2-1793870195582`, `储蓄卡`, `信用卡`
- 提交前：`#view.childElementCount=1`，账户行数=4，`#acctDlg.open=false`
- 点击「＋ 新增账户」：匹配按钮数=1，点击后 `#acctDlg.open=true`
- 提交后：`#view.childElementCount=1`，账户行数=5，`#acctDlg.open=false`，`#acctErr=「」`，toast=「✓ 账户已创建」
- 提交后文本观测：`viewTextContentHasName=true`，`viewInnerTextHasName=false`，`bodyHasName=false`，叶子节点命中=1（其中隐藏=1）
- 提交后全部 `<td>` 文本（共 20 个）：`现金`[不可见]；`现金`；`¥7,987.66`；`编辑 删除`；`复现账户-1-1793870188428`[不可见]；`现金`；`¥0.00`；`编辑 删除`；`复现账户-2-1793870195582`[不可见]；`现金`；`¥0.00`；`编辑 删除`；`储蓄卡`[不可见]；`活期储蓄`；`¥-50.00`；`编辑 删除`；`信用卡`[不可见]；`信用卡`；`¥0.00`；`编辑 删除`
- `#view` 重绘记录（MutationObserver，共 1 次）：{rows=5,textHasName=true,innerTextHasName=false}
- POST 响应后观测到重绘 = true；6s 内目标名出现在可见文本 = false
- 刷新页面后：`#view.childElementCount=1`，账户行数=5，`viewTextContentHasName=true`，`viewInnerTextHasName=false` → 刷新后仍不可见
- **归类：DOM 有但不可见/文本被隐藏**
- console error/pageerror：(无)
- 网络日志（API，共 7 条）：GET /api/v1/books/1/accounts → 200；GET /api/v1/books/1/categories → 200；GET /api/v1/books → 200；POST /api/v1/books/1/accounts → 200；GET /api/v1/books/1/accounts → 200；GET /api/v1/books/1/categories → 200；GET /api/v1/books → 200

### 迭代 3 · `复现账户-3-1793870202603`

- POST：`POST /api/v1/books/1/accounts` → HTTP **200**；响应体：`{"code":0,"data":{"id":9,"bookId":1,"name":"复现账户-3-1793870202603","category":1,"currency":"CNY","balance":0,"parentId":0,"syncVersion":9,"deleted":false,"createdAt":1793870203,"updatedAt":1793870203},"msg":"ok"}`
- GET /accounts 回读：共 6 个账户，含目标名 = **true**；名称列表：`现金`, `复现账户-1-1793870188428`, `复现账户-2-1793870195582`, `复现账户-3-1793870202603`, `储蓄卡`, `信用卡`
- 提交前：`#view.childElementCount=1`，账户行数=5，`#acctDlg.open=false`
- 点击「＋ 新增账户」：匹配按钮数=1，点击后 `#acctDlg.open=true`
- 提交后：`#view.childElementCount=1`，账户行数=6，`#acctDlg.open=false`，`#acctErr=「」`，toast=「✓ 账户已创建」
- 提交后文本观测：`viewTextContentHasName=true`，`viewInnerTextHasName=false`，`bodyHasName=false`，叶子节点命中=1（其中隐藏=1）
- 提交后全部 `<td>` 文本（共 24 个）：`现金`[不可见]；`现金`；`¥7,987.66`；`编辑 删除`；`复现账户-1-1793870188428`[不可见]；`现金`；`¥0.00`；`编辑 删除`；`复现账户-2-1793870195582`[不可见]；`现金`；`¥0.00`；`编辑 删除`；`复现账户-3-1793870202603`[不可见]；`现金`；`¥0.00`；`编辑 删除`；`储蓄卡`[不可见]；`活期储蓄`；`¥-50.00`；`编辑 删除`；`信用卡`[不可见]；`信用卡`；`¥0.00`；`编辑 删除`
- `#view` 重绘记录（MutationObserver，共 1 次）：{rows=6,textHasName=true,innerTextHasName=false}
- POST 响应后观测到重绘 = true；6s 内目标名出现在可见文本 = false
- 刷新页面后：`#view.childElementCount=1`，账户行数=6，`viewTextContentHasName=true`，`viewInnerTextHasName=false` → 刷新后仍不可见
- **归类：DOM 有但不可见/文本被隐藏**
- console error/pageerror：(无)
- 网络日志（API，共 7 条）：GET /api/v1/books/1/accounts → 200；GET /api/v1/books/1/categories → 200；GET /api/v1/books → 200；POST /api/v1/books/1/accounts → 200；GET /api/v1/books/1/accounts → 200；GET /api/v1/books/1/categories → 200；GET /api/v1/books → 200

## 汇总与结论

- 三次归类：1:DOM 有但不可见/文本被隐藏；2:DOM 有但不可见/文本被隐藏；3:DOM 有但不可见/文本被隐藏
- 三次 API 回读均含目标名：true
- 三次提交后 DOM 可见文本均含目标名：false
- 三次刷新后可见文本均含目标名：false

> 本文件仅记录修复前观测事实；根因判定见 `../root-cause-and-plan.md`。
