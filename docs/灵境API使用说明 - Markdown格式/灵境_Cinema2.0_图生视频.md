# 灵境 · Cinema2.0 接口文档

> **请求方式**：`POST`
> **请求地址**：`https://model.jdcloud.com/joycreator/openApi/ydSubmitTask`
> **Content-Type**：`application/json`

---

## 接口说明

本接口用于提交 **Cinema2.0** 任务。请求成功后返回 `genTaskId`，后续通过任务查询接口使用 `genTaskId` 查询获取任务结果。

---

## 请求示例

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/ydSubmitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
    "apiId": "753",
    "params": {
        "image": "https://lj-static.jdcloud.com/-lingjing-only/2urlvfos2xaygq_qydrcqdw6x8t7gakoti69d5hx1k-1m.jpg",
        "image_tail": "https://lj-static.jdcloud.com/-lingjing-only/2urlvfos2xaygq_qydrcqdw6x8t7gakoti69d5hx1k-1m.jpg",
        "prompt": "在此填写提示词内容",
        "model_name": "cinema-generate-2.0",
        "duration": "5",
        "mode": "720p",
        "aspect_ratio": "16:9",
        "generate_audio": true,
        "tools": false

    }
  }'
```

---

## 请求参数说明

> **📌 注意事项：**
> - 参数格式如上示例，`apiId` 字段固定必传，值为 `753`，请勿修改.
> - 请求头 `Authorization` 填写您的 AppKey，格式为 `Bearer ${your_app_key}`，请妥善保管，不要泄露。
> - 请求头`x-jdcloud-request-id`用来跟踪请求异常，每次请求不要重复，可以使用uuid或雪花id。
> - 图片、视频、音频类参数URL需公网可访问
> - 下方表格中的参数均位于 `params` 对象中


### 业务参数（params）

| 参数序号 | 参数名 | 类型 | 必传 | 说明 |
|----------|--------|------|------|------|
| 0 | `image` | string（图片 URL） | 是 | 首帧图（必填），大小 ≤ `10MB`，格式：`*.jpg,*.jpeg,*.png` |
| 1 | `image_tail` | string（图片 URL） | 否 | 尾帧图（可选），大小 ≤ `10MB`，格式：`*.jpg,*.jpeg,*.png` |
| 2 | `prompt` | string | 是 | 结合图片，描述想要生成的内容，文本内容，最大长度 `2500` 字符 |
| 3 | `model_name` | string | 是 | 模型，固定值 `cinema-generate-2.0`，不可修改 |
| 4 | `duration` | string | 是 | 视频参数。可选值：`4`(4s),`5`(5s),`6`(6s),`7`(7s),`8`(8s),`9`(9s),`10`(10s),`11`(11s),`12`(12s),`13`(13s),`14`(14s),`15`(15s) |
| 5 | `mode` | string | 是 | 视频参数。可选值：`480p`(480p),`720p`(720p),`1080p`(1080p) |
| 6 | `aspect_ratio` | string | 是 | 宽高比。可选值：`16:9`(16:9),`9:16`(9:16),`4:3`(4:3),`1:1`(1:1),`3:4`(3:4),`21:9`(21:9) |
| 7 | `generate_audio` | boolean | 是 | 同步音频。可选值：`true`(开),`false`(关) |
| 8 | `tools` | boolean | 否 | 联网搜索。可选值：`true`(开),`false`(关), 默认值：`false` |






---

## 响应参数

| 参数序号 | 参数名 | 类型 | 必传 | 说明 |
|----------|--------|------|------|------|
| 1 | `genTaskId` | string | 是 | 任务 ID，提交成功后返回，用于查询任务状态与结果 |

### 响应示例

```json
{
    "requestId": "s-86e4c7ed0eff4fbf967b81f-0c47d1",
    "error": {
        "code": 19,
        "message": "invalid_api_key"
    },
    "result": {
        "result": {
            "appId":"${your_app_id}",
            "error":"INVALID_API_KEY",
            "errorParamName":"${some one param}",
            "genTaskId":"任务ID:用于查询任务状态",
            "requestId":"${your_request_id}",
            "success":true
        }
    }
}
```

---

> 📌 本文档由系统自动生成，如有疑问请联系接口提供方。
