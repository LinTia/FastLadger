#!/usr/bin/env node
'use strict';
// 故意破损的扫描器替身（仅用于失败关闭负向集成测试）。
// 它对所有输入一律返回 rc=0（CLEAN），即「本应被阻断的凭据也被放行」。
// 用它驱动 verify-fail-closed-gate.js 可证明：一旦 fail-closed 验证失败，上传闸门被跳过、
// Artifact 数量 = 0，从而验证「失败关闭」链路的确依赖验证成功，而非空谈。
// 该文件绝不进入真实上传 / 真实扫描流程。
process.exitCode = 0;
