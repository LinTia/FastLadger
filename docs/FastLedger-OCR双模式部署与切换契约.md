# FastLedger OCR 双模式部署与切换契约

## 目标与隐私边界

账单截图优先由用户客户端侧识别；仅在用户明确调用服务端识别兜底时，截图上传到该用户自托管 FastLedger 服务器的本机 OCR 服务。该裁决替代旧 PRD §4.4、§5.1、§8.3、§9.2 中“图片不出本机、仅文字发往用户选定 LLM”的旧口径。服务端兜底不调用 LLM，不写 transactions 或 audit。

## HTTP API

两端点均要求现有 JWT Bearer 鉴权，并使用 `{code,msg,data}` envelope：

- `POST /api/v1/ocr/recognize`：`multipart/form-data`，字段 `image`；返回 `data={engine:"server",lines:[{text,confidence}]}`。
- `POST /api/v1/ocr/parse`：JSON `{ "text": "..." }`；返回 `data={candidates:[...]}`，candidate 字段为 `amountCents`（整数分）、`direction`（`income`/`expense`）、`occurredDate`（ISO 日期）、`description`、`confidence`。当前规则解析只有金额、方向和日期均可明确确认时才生成候选；歧义、冲突、非法日期、精度超过两位或溢出均为空 candidates，不猜。

### 解析细则（保守契约，QA 两轮对抗验证后冻结）

- **MIME 以服务端嗅探为准**：客户端 multipart 头可伪造，Go 侧按图片魔数（PNG/JPEG/WebP/BMP/TIFF）判定真实类型；非白名单内容一律 400，且不消耗配额、不触达下游。
- **带符号金额一律拒绝**：`-1.00`（退款/冲正语义不明）与 `+123.00`（符号歧义）都不产生候选，绝不取绝对值猜测。
- **方向关键词**：仅 `收入/入账/收款/到账/转入`（income）与 `支出/付款/消费/转出/扣款`（expense）判定方向；平台名（微信支付/支付宝）本身不代表收支，单独出现不产生候选。"微信支付收入"/"支付宝收入"通过收入词命中 income。
- **confidence 语义**：`recognize` 返回的 lines 携带下游 OCR 引擎置信度（0–1）；`parse` 候选恒为 0——规则解析没有置信度模型，0 表示"无置信度"而非"低可信"，客户端不得据此过滤。
- **配额语义**：`recognize`/`parse` 共享 `ocr:<uid>` 每分钟桶；`recognize` 在内容校验通过后才计数（垃圾请求不占配额），`parse` 每次请求计数。

`recognize` 与 `parse` 共用 uid 每分钟限额，默认 10 次，限流返回 429。图片最大 10 MiB，在 multipart 解析前限制请求体；服务未配置或下游异常统一 503。不得把候选自动写入交易，用户确认仍由既有交易入口处理。

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---:|---|
| `EBK_OCR_FALLBACK_URL` | 空 | 空值禁用服务端识别；配置本机 OCR **base URL**，例如 `http://127.0.0.1:8500`。**Go 侧自动追加 `/ocr` 路径**，配置值不要自带 `/ocr` 后缀（否则拼成 `/ocr/ocr`） |
| `EBK_OCR_TIMEOUT_MS` | 8000 | 下游请求超时，正整数，最大 120000 |
| `EBK_OCR_MAX_IMAGE_BYTES` | 10485760 | 最大请求图片字节数，正整数，不得超过 10 MiB |
| `EBK_OCR_RATE_PER_MIN` | 10 | uid 每分钟识别/解析共享次数，正整数，最大 1000 |

非法数值启动 fail-fast，不静默回退。下游固定访问配置端点 `/ocr`，客户端不能指定目标 URL；禁止跟随重定向，响应体有界。

## 本机 Python OCR 服务

建议独立虚拟环境安装 `fastapi`, `uvicorn`, `rapidocr_onnxruntime`, `pillow`。服务提供 `POST /ocr`，接收 multipart `image`（Content-Type 必为 Go 侧嗅探出的 `image/jpeg|png|webp|bmp|tiff`），返回 `{ "engine": "rapidocr_onnxruntime", "lines": [{"text":"...","confidence":0.98}] }`。`engine` 字段为 Go 侧强校验契约（`DisallowUnknownFields` 严格解码 + 值必须等于 `rapidocr_onnxruntime`），字段漂移或引擎替换需同步修改 Go 侧 `ocrDownstreamEngine` 常量。仅监听 `127.0.0.1:8500`，禁止暴露公网端口。

systemd unit 核心约束：

```ini
[Service]
User=fastledger
WorkingDirectory=/opt/ocr-fallback
ExecStart=/opt/ocr-fallback/.venv/bin/uvicorn app:app --host 127.0.0.1 --port 8500
Restart=on-failure
MemoryMax=1G
NoNewPrivileges=true
PrivateTmp=true
```

将 `EBK_OCR_FALLBACK_URL=http://127.0.0.1:8500`（base URL，Go 自动追加 `/ocr`）配入 FastLedger 服务环境后重启 FastLedger；回退时清空该值并重启即可。

**生产部署状态（2026-11-01）**：已部署并验证。`/opt/ocr-fallback/.venv`（rapidocr-onnxruntime 1.4.4 / onnxruntime 1.31.1 / fastapi / uvicorn / pillow，清华源）、`/etc/systemd/system/ocr-fallback.service`（active，enabled，仅监听 127.0.0.1:8500，MemoryMax=1G）、`EBK_OCR_FALLBACK_URL` 已配入 `/opt/fastledger/env`。实测：真图识别 score≈0.99、parse `¥1,234.56`→123456 分、停 sidecar→503 友好回退→恢复、`ocr:<uid>` 10 次/分钟共享配额第 11 次 429、RSS≈205MB。

## 失败语义与验收

- 无配置、下游连接失败、超时、非 2xx、无效 JSON 或响应过大：503 友好错误。
- 每个 uid 的 recognize/parse 合并计数；达到限制返回 429。
- 上传超限 413，格式/参数非法 400，未登录 401。
- 规则解析仅产出候选，不触发交易/审计写入；默认不猜、不补当前日期、不将小数浮点化。
- 测试需覆盖金额/全角/千分位/日期/方向规则、真实 multipart、超限、下游成功与失败、限流、鉴权及不写交易。
