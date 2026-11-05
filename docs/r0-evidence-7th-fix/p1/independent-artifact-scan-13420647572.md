# 独立下载扫描证据 · artifact 13420647572（commit 2182a58）

目的：在 CI 上传前扫描之外，独立下载**已存储**的 artifact 并递归扫描，证明存储产物本身不含凭据。

- 下载方式：`curl -sSL`（跟随 302；GitHub API 的 Authorization 头在跨域跳转到 Azure blob 时由 curl 自动剥离，SAS 令牌在 Location URL 查询串内，无需鉴权）。
- 解压：zipfile 提取 101 个条目，`testzip()` 完整性 None（无损坏）。
- 扫描器：`tools/r0/e2e/scripts/scan-artifacts.js`（零跳过，latin1 逐字节映射，5 条文本规则 + 二进制魔数附加防线，失败关闭）。
- 说明：本扫描器存在 K1–K7 已知盲点（见交付回执 §9），但 artifact 实际内容（80 PNG 截图 / 15 JSON 报告 / 3 TXT / 3 MD）不含 JWT/Bearer/Authorization/Cookie/SECRET 文本；凭据泄漏源 `trace.zip`（含 `trace.network` 真实 JWT）已在 P1 整改中不再上传。

## 扩展名清单
-      80 png
-      15 json
-       3 txt
-       3 md

## 扫描器输出
```
[scan] 待上传目录：<仓库外:artscan_13420647572>
[scan] 文件总数=101 内容已全量扫描=101 其中二进制文件=80 跳过内容扫描=0 符号链接=0
[scan] 扫描口径：全部文件以 latin1 逐字节映射后应用 5 条文本规则，无任何内容跳过分支
[scan] 命中明细（仅文件路径 + 命中类型 + 命中数量，不输出匹配值）：
[HIT] （无） | — | 0
[scan] 命中文件/类型组合=0 命中总数=0
[scan] 结论：CLEAN —— 允许上传
```
