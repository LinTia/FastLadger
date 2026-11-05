# 第四方独立审计（D）反例集与交叉验证说明

本目录为第四方独立审计员 D 在 R0 阶段对 `tools/r0/scan-credentials.js` 的反例证据。
**所有 token/JWT 均为合成样本，不含任何真实凭据**，仅用于验证扫描器规则覆盖范围。

## 交叉验证方法

在仓库根目录执行：

```bash
node tools/r0/scan-credentials.js --json
```

然后过滤本目录 `cases/` 下的文件，对照下表「实测」列。若 GPT/WorkBuddy 复跑结果与本表「实测」列不一致，说明环境或规则已变化，需重新核对。

## 反例对照表

| 文件 | 内容形态 | 预期（应检出） | 实测（D 本机复跑） | 对应 finding |
|---|---|---|---|---|
| n1-utf8.txt | UTF-8 `Authorization: Bearer <JWT>` | 检出 | BEARER_TOKEN + JWT_LITERAL | 对照组 |
| n2-xkey.txt | UTF-8 `x-api-key: <JWT>` | 检出（靠 JWT 三段兜底） | JWT_LITERAL（无 X_API_KEY 类型） | K3 部分：JWT 形态能被兜底 |
| n3-utf16.txt | **UTF-16LE 编码** `Authorization: Bearer <JWT>` | **应检出（当前规则失效）** | **0 检出** | D-R10 / K1 绕过 |
| n4-shortkey.txt | `x-api-key: <28位非JWT高熵值>` | **应检出（当前规则失效）** | **0 检出** | D-R11 / K3 绕过 |
| n5-query.txt | URL query `?access_token=sk-live-...` | **应检出（当前规则失效）** | **0 检出** | D-R07（NEW） |
| n6-har.json | HAR 数组 `{"name":"x-api-key","value":"sk-live-..."}` | **应检出（当前规则失效）** | **0 检出** | D-R08（NEW，对应 network-log.json 场景） |
| n7-jwtconst.js | JS 常量 `const t = "<JWT>"` | 检出 | JWT_LITERAL | 对照组 |
| n8-bearersk.txt | `Authorization: Bearer sk-live-...` | 检出 | BEARER_TOKEN | 对照组 |

## 关键结论

- **对照组（n1/n2/n7/n8）**：扫描器工作正常，JWT 三段式与 `Bearer ` 前缀能兜底检出。
- **绕过组（n3/n4/n5/n6）**：4 个形态均 0 检出，对应报告 D-R07/D-R08/D-R10/D-R11。
- n2 是重要细节：`x-api-key` 下若是 JWT，靠 JWT_LITERAL 兜底仍能检出；若是非 JWT 的普通高熵 key（n4），则完全漏检。

## 复现命令（一行）

```bash
node tools/r0/scan-credentials.js --json | python3 -c "import sys,json; d=json.load(sys.stdin); [print(r['file'],r['type'],r['kind']) for r in d['rows'] if 'r0-audit-d/cases/' in r['file']]"
```
