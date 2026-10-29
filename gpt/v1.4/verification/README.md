# v1.4 源码隔离核验

需要 Python 3.10+、Go 1.27+ 与支持 CGO 的 C 编译器。依赖版本固定于 go.mod/go.sum。

在仓库根目录运行：

```bash
python3 gpt/v1.4/verification/run.py
```

也可用 `--go /path/to/go` 指定工具链，`--output /path/to/results.json` 指定结果。

脚本在临时目录建立最小编译环境，将以下材料组合执行：

- `source/`：审查提交 9a3a2e3 的原文 Go 片段与完整 DDL；摘要见 source-manifest.json。反例源不可当作新业务实现。
- `models.go.txt/database.go.txt/authz_stub.go.txt`：补充设计仓库尚未提供的最小表映射、数据库变量、Hub 权限桩；没有改写原业务语句。Hub 的未使用 context import 仅以显式 shim 引用，使其可独立编译。
- `services_test.go.txt/hub_test.go.txt`：对原片段的真实数据库故障注入、并发和 race 验证。
- 当前指南 §13.1、§16.1、§22.4 与方案 §6.1：实时抽取修订创建、持仓、数据库初始化及唯一版本实现，配合 canonical_test.go.txt 执行预期行为回归。

原片段仍按旧 DDL 运行；新入口用测试迁移补 result_version。临时编译产物不进入仓库；脚本会更新本目录 go.mod/go.sum 和运行结果文件。

结果分为：原片段反例/正向测试、修订骨架回归、原初始化“预期编译失败”三类。测试 pass 的含义由测试名称与报告明确，不能解释成旧实现健康或整个产品通过验收。

本轮确实执行了 `go test -race`，覆盖交易函数和 Hub 核心；权限使用桩，不含真实 HTTP/WS 端到端、Flutter 或生产部署。运行时信息和完整编译错误见 runtime-results.json。
