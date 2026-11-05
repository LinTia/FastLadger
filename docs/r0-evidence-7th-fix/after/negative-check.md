# 负向合成验证：新增测试在修复前确实会失败

- 采集时间：2026-11-05T09:42:41.727Z ~ 2026-11-05T09:42:54.594Z
- 注入内容：`git show HEAD:server/web/dist/app.js`（commit `dadc9a3b9c567cad2bbad7e47361fb774365b949`，即修复前的 2241 行版本），SHA256=`32d853df552ce1e38ee51aaf474559959f09eae71eb5d7583eacad115323d37d`
- 方式：Playwright `page.route(/\/app\.js$/)` 在网络层把脚本响应替换为修复前版本；磁盘业务代码、Go 后端、API、DB 均未改动，服务端二进制为修复后重新构建的版本。
- 浏览器：真实 msedge；390×844 视口（与 C/D 套件一致）。

## 逐项观测

### NEG-1 · 对应 D-TXNS-01 / I-TXNS-01..19

```json
{
  "viewChildren": 0,
  "innerTextLen": 0,
  "hasTable": false
}
```

修复前为何会 FAIL：viewChildren=0 且无表格，D-TXNS-01 断言 childElementCount>=1 / 行数>=1 会 FAIL，I-TXNS-01..19 无控件可交互

### NEG-2 · 对应 D-CAT-01 / D-CAT-02 / D-CAT-03 / D-CAT-04

```json
{
  "expense": {
    "count": 30,
    "value": "23",
    "first": "工资",
    "values": [
      "23",
      "26",
      "28",
      "24",
      "25",
      "27",
      "29",
      "30",
      "1",
      "6",
      "10",
      "14",
      "17",
      "20",
      "2",
      "3",
      "4",
      "5",
      "7",
      "8",
      "9",
      "11",
      "12",
      "13",
      "15",
      "16",
      "18",
      "19",
      "21",
      "22"
    ]
  },
  "income": {
    "count": 30,
    "value": "23",
    "first": "工资",
    "values": [
      "23",
      "26",
      "28",
      "24",
      "25",
      "27",
      "29",
      "30",
      "1",
      "6",
      "10",
      "14",
      "17",
      "20",
      "2",
      "3",
      "4",
      "5",
      "7",
      "8",
      "9",
      "11",
      "12",
      "13",
      "15",
      "16",
      "18",
      "19",
      "21",
      "22"
    ]
  },
  "incomeIdsInExpense": 8,
  "expenseIdsInIncome": 22,
  "unchangedOnTypeSwitch": true
}
```

修复前为何会 FAIL：支出下拉混入收入类 id（首项「工资」），D-CAT-01/02 的集合相等断言会 FAIL；切换类型下拉不变，D-CAT-03 重置断言会 FAIL；默认选中收入类提交会被 400，D-CAT-04 会 FAIL

### NEG-3 · 对应 D-MEM-03

```json
{
  "inviteBtnCount": 1,
  "viewHasText": true
}
```

修复前为何会 FAIL：member 身份按钮数=1 且 #view 含「邀请成员」，D-MEM-03 断言 count===0 且无该文本会 FAIL（后端 403 与 UI 越权无关）

### NEG-4 · 对应 D-ACCT-01..04 / 旧 C 的 I-ACCT-06

```json
{
  "name": "NEG-账户-1793871765479",
  "postStatus": 200,
  "dom": {
    "viewChildren": 1,
    "innerTextHasName": false,
    "textContentHasName": true,
    "firstColDisplay": "none",
    "rows": 4
  },
  "apiHasName": true
}
```

修复前为何会 FAIL：修复前 app.js 与修复后一致地重绘了列表（POST 2xx、textContent 含名称），但 390px 下 style.css 隐藏首列使 innerText 不含名称——这正是旧 C 用 waitText(body.innerText) 恒 FAIL 的原因；D-ACCT-01 改用 textContent + 行数信号后判定才成立。本项证明第 4 项属测试口径问题、业务代码无需改动

> 结论：D 套件的断言不是恒真式；在修复前实现下，上述 4 组观测值分别使对应用例 FAIL。修复后的 a/c/d 结果见同目录 `*-after.log`。
