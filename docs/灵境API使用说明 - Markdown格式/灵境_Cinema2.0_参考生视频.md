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
    "apiId": "754",
    "params": {
        "multi_model_url": [
          {
            "ref_name": "图1",
            "url": "https://lj-static.jdcloud.com/-lingjing-only/wn24a1svupgzy7oxr2jfhoiqy1kcp61jzsx6aleepsi-1m.jpeg",
            "media_type": "image"
          }
        ],
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
> - 参数格式如上示例，`apiId` 字段固定必传，值为 `754`，请勿修改.
> - 请求头 `Authorization` 填写您的 AppKey，格式为 `Bearer ${your_app_key}`，请妥善保管，不要泄露。
> - 请求头`x-jdcloud-request-id`用来跟踪请求异常，每次请求不要重复，可以使用uuid或雪花id。
> - 图片、视频、音频类参数URL需公网可访问
> - 下方表格中的参数均位于 `params` 对象中


### 业务参数（params）

| 参数序号 | 参数名 | 类型 | 必传 | 说明 |
|----------|--------|------|------|------|
| 0 | `multi_model_url` | object[] | 是 | 上传图/音/视频，混合媒体列表（图片/视频/音频），详见下方媒体限制说明。最多支持上传9张图片、3个视频、3个音频，视频格式尺寸要求：[640×640, 2206×946]，视频总时长[2-15]秒，建议720p视频。 |
| ↳ 0.1 | `multi_model_url.ref_name` | string | 是 | 参考名 |
| ↳ 0.2 | `multi_model_url.media_type` | string | 是 | 支持的媒体类型：`image`,`video`,`audio` |
| ↳ 0.3 | `multi_model_url.url` | string | 是 | 媒体url |
| 1 | `prompt` | string | 是 | 结合参考素材，描述想要生成的内容，文本内容，最大长度 `2500` 字符 |
| 2 | `model_name` | string | 是 | 模型，固定值 `cinema-generate-2.0`，不可修改 |
| 3 | `duration` | string | 是 | 视频参数。可选值：`4`(4s),`5`(5s),`6`(6s),`7`(7s),`8`(8s),`9`(9s),`10`(10s),`11`(11s),`12`(12s),`13`(13s),`14`(14s),`15`(15s) |
| 4 | `mode` | string | 是 | 视频参数。可选值：`480p`(480p),`720p`(720p),`1080p`(1080p) |
| 5 | `aspect_ratio` | string | 是 | 宽高比。可选值：`16:9`(16:9),`9:16`(9:16),`4:3`(4:3),`1:1`(1:1),`3:4`(3:4),`21:9`(21:9) |
| 6 | `generate_audio` | boolean | 是 | 同步音频。可选值：`true`(开),`false`(关) |
| 7 | `tools` | boolean | 否 | 联网搜索。可选值：`true`(开),`false`(关), 默认值：`false` |


### `multi_model_url` 媒体限制说明

> 最多支持上传9张图片、3个视频、3个音频，视频格式尺寸要求：[640×640, 2206×946]，视频总时长[2-15]秒，建议720p视频。

**图片限制**

| 限制项 | 值 |
|--------|----|
| 数量范围 | `[0, 9]` |
| 文件大小上限 | `30MB` |
| 支持格式 | `*.jpg,*.jpeg,*.png,*.webp` |
| 宽度范围 | `[300px, 6000px]` |
| 高度范围 | `[300px, 6000px]` |
| 宽高比范围 | `[0.4, 2.5]` |

**视频限制**

| 限制项 | 值 |
|--------|----|
| 数量范围 | `[0, 3]` |
| 文件大小上限 | `50MB` |
| 支持格式 | `*.mp4,*.mov` |
| 宽度范围 | `[300px, 6000px]` |
| 高度范围 | `[300px, 6000px]` |
| 宽高比范围 | `[0.4, 2.5]` |
| 时长范围 | `[2s, 15s]` |
| 总时长上限 | `15s` |
| 帧率范围 | `[24fps, 60fps]` |
| 尺寸额外说明 | `要求视频格式尺寸：[640×640, 2206×946]` |

**音频限制**

| 限制项 | 值 |
|--------|----|
| 数量范围 | `[0, 3]` |
| 文件大小上限 | `15MB` |
| 支持格式 | `*.mp3,*.wav` |
| 时长范围 | `[2s, 15s]` |
| 总时长上限 | `15s` |





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
