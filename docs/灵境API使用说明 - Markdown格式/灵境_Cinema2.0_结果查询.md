## 3. API 结果查询 (Task 结果查询)

该接口用于非开放平台前端调用。后端逻辑将验证 API Key 的合法性，并根据提交任务时返回的 `genTaskId` 查询最终结果。查询结果包含加水印与非水印的资源地址。

### 3.1 快速开始：Curl 调用示例

```
curl -X POST "https://{domain}/joycreator/openApi/queryTasKResult" \
  -H "Authorization: Bearer {your_api_key}" \
  -H "Content-Type: application/json" \
  -H "x-jdcloud-request-id: {request_id}" \
  -d '{
    "genTaskId": "1024"
  }'

```

### 3.2 请求参数说明

**Header 参数**

| 参数名 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| **Authorization** | string | 是 | 格式： `Bearer xxx`。xxx 为用户在开放平台中某个应用下的 API Key。 |
| **x-jdcloud-request-id** | string | 是 | 建议使用类 UUID 数据，用于请求追踪。 |
| **Content-Type** | string | 是 | 固定值： `application/json` |

**Body 参数**

| 参数名 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| **genTaskId** | string | 是 | 提交任务接口返回的任务 ID。 |

---

## 4. 返回参数说明

### 4.1 基础响应结构

| 字段名 | 类型 | 说明 |
| --- | --- | --- |
| **requestId** | string | 请求唯一标识。 |
| **error** | string | 错误信息（正常时为空）。 |
| **result** | object | 业务结果对象。 |

### 4.2 ServiceError 错误信息（外层 error）

| 字段 | 中文名 | 类型 | 说明 |
| --- | --- | --- | --- |
| `code` | 错误码 | int | 错误编号 |
| `message` | 错误描述 | String | 错误提示信息 |
| `status` | 错误状态 | String | 错误状态标识 |
| `details` | 错误详情列表 | Map[] | 详细错误信息数组 |
| `extra` | 额外扩展信息 | Map | 扩展错误信息 |

---

### 4.3 业务字段 OpenApiTaskResult

| 字段 | 中文名 | 类型 | 说明 |
| --- | --- | --- | --- |
| `id` | 任务记录ID | long | 任务唯一记录ID |
| `pin` | 用户标识 | String | 用户身份标识 |
| `sceneCode` | 场景编码 | String | 如 image-generation、video-generation |
| `modelCode` | 模型编码 | String | 使用的模型标识，如 Doubao-Seedream-5.0-lite |
| `reqParam` | 请求参数列表 | List<OriginRequestParamView> | 已转换为可读格式的请求参数列表 |
| `taskStatus` | 任务状态 | Integer | 0-任务中，1-任务完成，2-任务失败 |
| `billingItemKey` | 计费项标识 | String | 计费项唯一标识，如 "image-gen-count"。 **注意：此字段仅作为客户与平台间对账的部分依据之一，不可作为其他任何用途，不代表最终结算依据，请以平台账单为准。** |
| `billingItemValue` | 计费项数值 | BigDecimal | 本次消耗的计费数值。 **注意：此字段仅作为客户与平台间对账的部分依据之一，不可作为其他任何用途，不代表最终结算依据。部分模型按Token计费，返回值为 x个千Tokens。** |
| `createTime` | 任务创建时间 | LocalDateTime | 任务创建的时间 |
| `updateTime` | 任务更新时间 | LocalDateTime | 任务最近更新的时间 |
| `finishedTime` | 任务完成时间 | LocalDateTime | 任务完成的时间 |
| `taskResults` | 子任务结果集 | List<SubTask> | 子任务结果数组 |
| `resultNum` | 成功结果数量 | Integer | 成功生成的结果数 |
| `targetNum` | 应生成结果数量 | Integer | 目标应生成的结果数 |
| `appId` | 应用ID | String | 调用方应用标识 |
| `error` | 错误枚举 | AIModelApiServiceError | 成功时为 null，失败时含 code 和 desc |
| `errorParamName` | 错误参数名称 | String | 参数校验失败时指向具体出错的参数名 |
| `success` | 是否成功 | boolean | 请求是否成功 |
| `errMsg` | 错误信息 | String | 过滤后的错误信息 JSON 数组，包含 exactCode / exactDesc / subTaskId |

> **计费字段说明： `billingItemKey`（计费项标识）与 `billingItemValue`（计费项数值）仅可作为客户与平台进行对账时的部分参考依据，不构成最终结算的唯一凭证，且不可作为其他任何用途。实际计费以平台账单系统记录为准。**

---

### 4.4 reqParam 参数项 OriginRequestParamView

| 字段 | 中文名 | 类型 | 说明 |
| --- | --- | --- | --- |
| `filedName` | 参数字段名称 | String | 参数的显示名称，如"模型"、"尺寸"、"提示词" |
| `values` | 参数值 | Object | 参数值，可为字符串 / 数组 / 数字等 |

---

### 4.5 taskResults 子任务 SubTask （部分存在生图任务）

| 字段 | 中文名 | 类型 | 说明 |
| --- | --- | --- | --- |
| `subStatus` | 子任务状态 | Integer | 0-任务中，1-任务完成，2-任务失败，4-水印任务完成 |
| `url` | 子任务作品URL | String | 无水印作品访问地址（白名单用户返回预签名 URL，否则为 null） |
| `watermarkUrl` | 子任务水印作品URL | String | 带水印的作品访问地址（预签名 URL） |
| `errorReason` | 子任务失败原因 | String | 失败时的错误原因，成功时为 null |

---

---

## 5. 响应示例

### 成功响应

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "error": null,
  "result": {
    "result": {
      "id": 12345,
      "pin": "xxx",
      "sceneCode": "image-generation",
      "modelCode": "Doubao-Seedream-5.0-lite",
      "reqParam": [
        { "filedName": "模型", "values": "Doubao-Seed" },
        { "filedName": "尺寸", "values": "2048x2048" },
        { "filedName": "生成数量", "values": 1 }
      ],
      "taskStatus": 1,
      "billingItemKey": "image-gen-count",
      "billingItemValue": 1.00,
      "createTime": "2026-05-22T16:48:00",
      "updateTime": "2026-05-22T16:50:20",
      "finishedTime": "2026-05-22T16:50:20",
      "taskResults": [
        {
          "subStatus": 1,
          "url": "",
          "watermarkUrl": "",
          "errorReason": null
        }
      ],
      "resultNum": 1,
      "targetNum": 1,
      "appId": "app-12345",
      "error": null,
      "errorParamName": null,
      "success": true,
      "errMsg": null
    }
  }
}
```

### 失败响应（任务不存在）

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "error": null,
  "result": {
    "result": {
      "id": 0,
      "pin": null,
      "sceneCode": null,
      "modelCode": null,
      "reqParam": null,
      "taskStatus": null,
      "billingItemKey": null,
      "billingItemValue": null,
      "createTime": null,
      "updateTime": null,
      "finishedTime": null,
      "taskResults": [],
      "resultNum": null,
      "targetNum": null,
      "appId": null,
      "error": { "code": 21, "desc": "task_not_exist" },
      "errorParamName": null,
      "success": false,
      "errMsg": null
    }
  }
}
```

### 含错误信息的响应

```json
{
  "requestId": "550e8400-e29b-41d0",
  "error": null,
  "result": {
    "result": {
      "id": 67890,
      "pin": "xxx",
      "sceneCode": "video-generation",
      "modelCode": "Seedance-1.0",
      "reqParam": [],
      "taskStatus": 2,
      "billingItemKey": "video-gen-count",
      "billingItemValue": 0,
      "createTime": "2026-05-22T16:48:00",
      "updateTime": "2026-05-22T16:50:20",
      "finishedTime": "2026-05-22T16:50:20",
      "taskResults": [
        {
          "subStatus": 2,
          "url": null,
          "watermarkUrl": null,
          "errorReason": "模型内部错误"
        }
      ],
      "resultNum": 0,
      "targetNum": 1,
      "appId": "app-12345",
      "error": null,
      "errorParamName": null,
      "success": true,
      "errMsg": "[{\"exactCode\":\"MODEL_INTERNAL_ERROR\",\"exactDesc\":\"模型内部处理失败，请稍后重试\",\"subTaskId\":\"sub-001\"}]"
    }
  }
}
```
