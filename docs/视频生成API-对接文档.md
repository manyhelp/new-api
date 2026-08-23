# 视频生成 API · 对接文档

> 本文档面向应用接入方。通过本网关调用 **视频生成** 能力（模型 `cinema-generate-2.0`），支持**文生视频**、**图生视频**与**任务结果查询**。
>
> 接口为 OpenAI 兼容风格，只需标准 HTTP + JSON 即可对接。

---

## 1. 概述

### 1.1 接入信息

| 项目 | 值 |
|---|---|
| 网关地址（Base URL） | `https://<your-gateway-host>` |
| 接口风格 | OpenAI 兼容 |
| 鉴权方式 | Bearer Token（网关令牌，`sk-xxxxxxxx`） |
| 模型名 | `cinema-generate-2.0` |
| 内容类型 | `application/json` |

### 1.2 接口列表

| 接口 | 方法 | 路径 | 说明 |
|---|---|---|---|
| 创建任务 | `POST` | `/v1/video/generations` | 提交文生 / 图生任务，立即返回任务 ID |
| 查询任务 | `GET` | `/v1/video/generations/{task_id}` | 查询任务状态与结果（视频地址） |

### 1.3 调用流程

视频生成为**异步任务**：

1. 调「创建任务」接口，得到 `task_id`；
2. 用 `task_id` 周期性调「查询任务」接口；
3. 当 `status` 变为 `SUCCESS` 时，从 `result_url` 取视频地址。

---

## 2. 鉴权

所有请求需在 Header 携带网关令牌：

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

> 令牌由网关后台「令牌」页面创建。令牌所在分组须与可服务 `cinema-generate-2.0` 的渠道分组一致（默认 `default`），否则会返回 `503 model_not_found`。

---

## 3. 文生视频（Text-to-Video）

### 3.1 请求

- **请求方式**：`POST`
- **请求地址**：`https://<your-gateway-host>/v1/video/generations`
- **Content-Type**：`application/json`

### 3.2 请求示例

```bash
curl -X POST "https://<your-gateway-host>/v1/video/generations" \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "cinema-generate-2.0",
    "prompt": "一只猫在花园里追逐蝴蝶，阳光明媚，电影感镜头",
    "metadata": {
      "duration": 5,
      "resolution": "720p",
      "ratio": "16:9",
      "generate_audio": true
    }
  }'
```

### 3.3 请求参数说明

> **📌 注意事项：**
> - `model` 固定为 `cinema-generate-2.0`，不可修改。
> - 视频参数必须放在 `metadata` 对象内。**顶层的 `duration` / `width` / `height` 等字段会被网关忽略**。
> - `metadata` 内所有参数均为可选，未传时使用默认值。

#### 业务参数（顶层）

| 参数序号 | 参数名 | 类型 | 必传 | 说明 |
|---|---|---|---|---|
| 0 | `prompt` | string | 是 | 描述想要生成的内容，支持联网搜索获取最新信息，最大长度 `2500` 字符 |
| 1 | `model` | string | 是 | 模型名，固定值 `cinema-generate-2.0`，不可修改 |
| 2 | `metadata` | object | 否 | 视频参数对象，其内字段见下表 |

#### 视频参数（`metadata` 内）

| 参数序号 | 参数名 | 类型 | 必传 | 说明 |
|---|---|---|---|---|
| 0 | `duration` | int | 否 | 视频时长（秒）。可选值：`4`、`5`、`6`、`7`、`8`、`9`、`10`、`11`、`12`、`13`、`14`、`15`。默认 `5` |
| 1 | `resolution` | string | 否 | 分辨率。可选值：`480p`、`720p`、`1080p`。默认 `720p` |
| 2 | `ratio` | string | 否 | 宽高比。可选值：`16:9`、`9:16`、`4:3`、`1:1`、`3:4`、`21:9`。默认 `16:9` |
| 3 | `generate_audio` | bool | 否 | 是否同步生成音频。可选值：`true`、`false`。默认 `true` |
| 4 | `tools` | bool | 否 | 是否启用联网搜索。可选值：`true`、`false`。默认 `false` |

---

## 4. 图生视频（Image-to-Video）

在文生视频基础上，请求体**增加 `image` 字段**（首帧图）即可，网关会自动按图生视频处理。

### 4.1 请求

- **请求方式**：`POST`
- **请求地址**：`https://<your-gateway-host>/v1/video/generations`
- **Content-Type**：`application/json`

### 4.2 请求示例

```bash
curl -X POST "https://<your-gateway-host>/v1/video/generations" \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "cinema-generate-2.0",
    "prompt": "让画面中的猫咪向镜头跑来",
    "image": "https://your-cdn.example.com/cat.jpg",
    "metadata": {
      "image_tail": "https://your-cdn.example.com/cat-night.jpg",
      "duration": 5,
      "resolution": "720p",
      "ratio": "16:9"
    }
  }'
```

### 4.3 请求参数说明

> **📌 注意事项：**
> - 图生视频必须传 `image`（首帧图）；文生视频不要传该字段。
> - 图片、视频类参数 URL **需公网可访问**。
> - 单张图片大小 ≤ `10MB`，格式：`*.jpg`、`*.jpeg`、`*.png`。
> - `prompt` 在图生视频时**仍为必填**，用于结合图片描述生成意图。

#### 业务参数（顶层）

| 参数序号 | 参数名 | 类型 | 必传 | 说明 |
|---|---|---|---|---|
| 0 | `prompt` | string | 是 | 结合图片，描述想要生成的内容，最大长度 `2500` 字符 |
| 1 | `model` | string | 是 | 模型名，固定值 `cinema-generate-2.0`，不可修改 |
| 2 | `image` | string（URL） | 是 | 首帧图（必填），大小 ≤ `10MB`，格式：`*.jpg`、`*.jpeg`、`*.png` |
| 3 | `metadata` | object | 否 | 视频参数对象，其内字段见下表 |

#### 视频参数（`metadata` 内）

| 参数序号 | 参数名 | 类型 | 必传 | 说明 |
|---|---|---|---|---|
| 0 | `image_tail` | string（URL） | 否 | 尾帧图（可选），大小 ≤ `10MB`，格式：`*.jpg`、`*.jpeg`、`*.png` |
| 1 | `duration` | int | 否 | 视频时长（秒）。可选值：`4` ~ `15`。默认 `5` |
| 2 | `resolution` | string | 否 | 分辨率。可选值：`480p`、`720p`、`1080p`。默认 `720p` |
| 3 | `ratio` | string | 否 | 宽高比。可选值：`16:9`、`9:16`、`4:3`、`1:1`、`3:4`、`21:9`。默认 `16:9` |
| 4 | `generate_audio` | bool | 否 | 是否同步生成音频。默认 `true` |
| 5 | `tools` | bool | 否 | 是否启用联网搜索。默认 `false` |

> 首帧 + 尾帧也可用 `images` 数组一次传入：`"images": ["<首帧URL>", "<尾帧URL>"]`，等价于 `image` + `metadata.image_tail`。

### 4.4 创建响应

与文生视频一致（见 [5. 响应结构](#5-响应结构)）。后台任务日志会把图生任务标记为 `generate`（图生视频），文生标记为 `textGenerate`（文生视频）。

---

## 5. 响应结构

### 5.1 创建任务响应

```json
{
  "id": "task_1IfXEwEFZoWpdLHECcidBDIdQfxGXuKl",
  "task_id": "task_1IfXEwEFZoWpdLHECcidBDIdQfxGXuKl",
  "object": "video",
  "model": "cinema-generate-2.0",
  "status": "queued",
  "progress": 0,
  "created_at": 1783059791
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` / `task_id` | string | 任务 ID（两者相同），用于查询 |
| `object` | string | 固定 `video` |
| `model` | string | 模型名 |
| `status` | string | 初始为 `queued`（已受理） |
| `progress` | int | 进度，初始 `0` |
| `created_at` | int | 创建时间（Unix 秒） |

> 返回 `id` 仅代表任务**已受理**，不代表视频已生成，需继续查询。

### 5.2 查询任务响应（成功示例）

`GET /v1/video/generations/{task_id}`

```json
{
  "code": "success",
  "message": "",
  "data": {
    "id": 2,
    "created_at": 1783059791,
    "updated_at": 1783060176,
    "task_id": "task_1IfXEwEFZoWpdLHECcidBDIdQfxGXuKl",
    "platform": "59",
    "user_id": 1,
    "group": "default",
    "channel_id": 1,
    "quota": 125000,
    "action": "textGenerate",
    "status": "SUCCESS",
    "fail_reason": "",
    "result_url": "https://<网关域名>/api/videos/task_1IfXEwEFZoWpdLHECcidBDIdQfxGXuKl.mp4",
    "submit_time": 1783059791,
    "start_time": 1783059817,
    "finish_time": 1783060176,
    "progress": "100%",
    "properties": {
      "input": "",
      "upstream_model_name": "cinema-generate-2.0",
      "origin_model_name": "cinema-generate-2.0"
    },
    "data": {
      "request_params": [
        { "filedName": "prompt", "values": "一只猫在花园里追逐蝴蝶，阳光明媚" },
        { "filedName": "model_name", "values": "cinema-generate-2.0" },
        { "filedName": "duration", "values": "5" },
        { "filedName": "mode", "values": "720p" },
        { "filedName": "aspect_ratio", "values": "16:9" },
        { "filedName": "generate_audio", "values": true },
        { "filedName": "tools", "values": false }
      ]
    }
  }
}
```

### 5.3 响应字段说明

#### 外层与 `data` 顶层字段

| 字段 | 路径 | 说明 |
|---|---|---|
| `code` | 顶层 | 固定 `success` 表示请求被正确处理 |
| `message` | 顶层 | 错误时填写，正常为空 |
| `data.task_id` | `data.task_id` | 任务 ID |
| `data.action` | `data.action` | 任务类型：`textGenerate`（文生）/ `generate`（图生） |
| `data.status` | `data.status` | **任务状态（大写）**，见 5.4 |
| `data.progress` | `data.progress` | 进度，如 `"50%"`、`"100%"` |
| `data.result_url` | `data.result_url` | **视频地址（mp4 直链）**：`https://<网关域名>/api/videos/{task_id}.mp4`浏览器可直接下载/播放，支持 Range 拖动； |
| `data.fail_reason` | `data.fail_reason` | 失败原因，状态为 `FAILURE` 时填写 |
| `data.created_at` / `updated_at` | 同名 | 创建 / 最近更新时间（Unix 秒） |
| `data.submit_time` / `start_time` / `finish_time` | 同名 | 提交 / 开始 / 完成时间（Unix 秒） |
| `data.quota` | `data.quota` | 本次任务消耗的配额 |
| `data.properties` | `data.properties` | 模型等元信息 |

#### `data.data`

| 字段 | 路径 | 说明 |
|---|---|---|
| `request_params` | `data.data.request_params` | 本次请求的参数列表，每项 `{ "filedName": "...", "values": ... }` |

仅返回：`prompt` / `model_name` / `duration` / `mode` / `aspect_ratio` / `generate_audio` / `tools`
（即你提交时的视频参数，如 `720p`、`5s`）。

### 5.4 轮询建议

- 轮询间隔：**10 秒**。
- 一段 5 秒视频通常 3 ~ 5 分钟**完成。
- 建议设置最大轮询次数（如 60 次），避免无限等待。

```python
import time, requests

def wait_for_video(base_url, api_key, task_id, interval=30, max_attempts=60):
    url = f"{base_url}/v1/video/generations/{task_id}"
    headers = {"Authorization": f"Bearer {api_key}"}
    for _ in range(max_attempts):
        resp = requests.get(url, headers=headers, timeout=30).json()
        d = resp["data"]
        if d["status"] == "SUCCESS":
            return d["result_url"]            # ✅ 视频地址
        if d["status"] == "FAILURE":
            raise RuntimeError(d.get("fail_reason"))
        time.sleep(interval)
    raise TimeoutError("轮询超时")
```

---

## 6. 错误处理

### 6.1 常见错误码

| HTTP 状态 | `error.code` | 含义 | 处理建议 |
|---|---|---|---|
| `400` | `invalid_request` | 缺 `prompt` / `model` 等必填字段 | 检查请求体 |
| `401` | — | 令牌无效或缺失 | 检查 `Authorization` |
| `503` | `model_not_found` | 分组下无可用渠道、模型名不对 | 联系管理员检查渠道分组与模型配置；确认模型名为 `cinema-generate-2.0` |
| `502` | `invalid_response` | 上游返回错误（如参数违规、图片不可访问） | 看响应 `message`；图生确认图片公网可访问 |
| `500` | — | 网关内部错误 | 联系管理员查网关日志 |

### 6.2 错误响应格式

```json
{
  "error": {
    "code": "model_not_found",
    "message": "No available channel for model cinema-generate-2.0 under group default",
    "type": "new_api_error"
  }
}
```

---

## 7. 完整端到端示例（Python）

```python
import time, requests, urllib3
urllib3.disable_warnings()

BASE_URL = "https://<your-gateway-host>"
API_KEY  = "sk-xxxxxxxx"
HEADERS  = {"Authorization": f"Bearer {API_KEY}", "Content-Type": "application/json"}

def create_task(prompt, image=None, **meta):
    body = {
        "model": "cinema-generate-2.0",
        "prompt": prompt,
        "metadata": {"duration": 5, "resolution": "720p", "ratio": "16:9", **meta},
    }
    if image:
        body["image"] = image
    r = requests.post(f"{BASE_URL}/v1/video/generations",
                      headers=HEADERS, json=body, timeout=30, verify=False)
    r.raise_for_status()
    return r.json()["id"]

def wait_video(task_id, interval=5, max_attempts=60):
    url = f"{BASE_URL}/v1/video/generations/{task_id}"
    for _ in range(max_attempts):
        d = requests.get(url, headers=HEADERS, timeout=30, verify=False).json()["data"]
        if d["status"] == "SUCCESS":
            return d["result_url"]
        if d["status"] == "FAILURE":
            raise RuntimeError(d.get("fail_reason"))
        time.sleep(interval)
    raise TimeoutError("轮询超时")

# 文生视频
tid = create_task("一只猫在花园里追逐蝴蝶，阳光明媚")
print("task_id:", tid)
print("video:", wait_video(tid))

# 图生视频
tid = create_task("让画面中的猫咪向镜头跑来",
                  image="https://your-cdn.example.com/cat.jpg")
print("video:", wait_video(tid))
```

---

## 8. 约束与最佳实践

- **模型名固定** `cinema-generate-2.0`；文生 / 图生用同一模型名，由是否传 `image` 自动区分。
- **视频参数走 `metadata`**：顶层的 `width` / `height` / 浮点 `duration` 会被网关忽略。
- **图生图片必须公网可访问**，单张 ≤ `10MB`，格式 `jpg` / `jpeg` / `png`。
- **异步模型**：创建接口只返回 `task_id`，务必轮询查询接口获取最终视频地址。
- **轮询节流**：间隔 ≥ 5 秒，避免无谓请求。
- **视频地址**：返回的是网关本地直链 `/api/videos/<task_id>.mp4`，**长期有效**，浏览器可直接下载/播放。
- **HTTPS / 证书**：若网关使用自签证书，客户端需关闭 SSL 校验（`verify=False` 或 curl 加 `-k`）。

---

## 附录：状态码速查

| 客户端可见 `data.status` | 上游 `taskStatus` | 含义 |
|---|---|---|
| `QUEUED` / `NOT_STARTED` | `0` | 排队 / 处理中 |
| `IN_PROGRESS` | `0` | 生成中 |
| `SUCCESS` | `1` 或 `4` | 成功，视频就绪 |
| `FAILURE` | `2` | 失败 |

---

*如网关版本升级，接口契约以本网关 `/v1/video/generations` 接口实际行为为准。*
