# 灵境 Cinema2.0 接入 new-api 原生渠道 · 说明文档

> 把京东云灵境 Cinema2.0 视频生成作为 new-api 的**原生渠道（类型 59，JDCloud）**接入。
> 客户端用 OpenAI 风格的 `/v1/video/generations` 调 new-api，new-api 内部翻译成灵境的
> `ydSubmitTask` / `queryTasKResult` 协议。无需外部代理/shim。
>
- **源码目录**：`new-api-1.0.0-rc.15/`
- **测试客户端**：`video_api.py`（同目录）
- **灵境接口文档**：`灵境_Cinema2.0_*.html`（同目录）
- **改动日期**：2026-07

---

## 1. 背景（为什么这么做）

灵境 Cinema2.0 用的是**自定义异步任务协议**，与 OpenAI/Sora 视频格式完全不兼容：

| | OpenAI/Sora（new-api 对外） | 灵境 Cinema2.0（上游） |
|---|---|---|
| 提交 | `POST /v1/video/generations` → `{id}` | `POST /joycreator/openApi/ydSubmitTask` `{apiId:"752",params:{...}}` → `genTaskId` |
| 查询 | `GET /v1/video/generations/{id}` | `POST /joycreator/openApi/queryTasKResult` `{genTaskId}` → `taskStatus`+`taskResults` |
| 鉴权 | `Authorization: Bearer sk-xxx` | 还要 `x-jdcloud-request-id` |

new-api 的任务适配器是写死的 switch（只支持豆包/可灵/即梦/Vidu/Sora/Gemini 等），**没有京东云**。
所以在 new-api 源码里新增了 `jdcloud` 适配器，在中间做协议翻译。

---

## 2. 改动清单（升级 new-api 时要重新应用！）

后端（5 处）：

| 文件 | 改动 |
|---|---|
| `constant/channel.go` | 新增 `ChannelTypeJDCloud = 59`、`ChannelBaseURLs` 加 `https://model.jdcloud.com`、`ChannelTypeNames` 加 `"JDCloud"` |
| `relay/channel/task/jdcloud/constants.go` | **新建**：模型清单、固定 `apiId=752`/`cinema-generate-2.0`、提交/查询路径 |
| `relay/channel/task/jdcloud/adaptor.go` | **新建**：核心适配器，协议双向翻译 |
| `relay/relay_adaptor.go` | `import` + `GetTaskAdaptor` switch 注册 `case ChannelTypeJDCloud` |
| `controller/channel-test.go` | `unsupportedTestChannelTypes` 加 JDCloud（视频渠道不支持测试按钮） |

前端（3 处，`web/default/`）：

| 文件 | 改动 |
|---|---|
| `src/features/channels/constants.ts` | `CHANNEL_TYPES` 加 `59:'JDCloud'` + `CHANNEL_TYPE_DISPLAY_ORDER` 加 `59` |
| `src/i18n/locales/en.json` | `"JDCloud": "JDCloud"` |
| `src/i18n/locales/zh.json` | `"JDCloud": "京东云灵境"` |

> 协议翻译逻辑要点：`metadata.duration/ratio/resolution` → 灵境 `duration/mode/aspect_ratio`；
> `taskStatus` 0=进行中 / 1=完成(取 `taskResults[].url`，无则 `watermarkUrl`) / 2=失败；
> 视频地址最终落在客户端响应的 `metadata.url`。

---

## 3. 部署步骤（一次性）

在 `new-api-1.0.0-rc.15/` 下：

```bash
# 1) 构建前端（后台下拉框才会出现「JDCloud」+ 嵌入二进制）
make build-all-frontends
# 2) 构建后端
go build -o new-api .
# 3) 把 new-api 替换到 172.10.x.x 上正在运行的版本，重启服务
```
也可 `make all` 或用 Docker（Dockerfile 已含前端构建）。

---

## 4. 后台配置

### 4.1 新建渠道
后台 → **渠道** → **添加渠道**：

| 字段 | 填什么 |
|---|---|
| 类型 | **JDCloud / 京东云灵境**（59，重建前端后才有） |
| 名称 | 随意，如 `灵境Cinema2.0` |
| 分组 | 与令牌分组一致，通常 `default`（⚠️ 漏配 = 503 无可用渠道） |
| 模型 | 手动添加 **`cinema-generate-2.0`** |
| 密钥 | 灵境 AppKey（京东云灵境/JoyBuilder 控制台创建应用获取） |
| 代理/Base URL | `https://model.jdcloud.com`（灵境给了别的 `{domain}` 就填那个；尾斜杠代码已自动去除） |

保存。点「测试」会提示「JDCloud channel test is not supported」—— **正常现象**，不代表渠道有问题。

### 4.2 令牌
**令牌** → 编辑 `video_api.py` 里那个 `sk-...` → 分组 = `default`；若开了模型限制要包含 `cinema-generate-2.0`。

### 4.3 计费（建议）
**设置 → 运营设置 → 模型固定价格/倍率**：给 `cinema-generate-2.0` 配价（参考灵境单次计费），避免额度校验异常。

---

## 5. 客户端用法

`video_api.py` 已配好，直接：
```bash
python video_api.py
```
要点（已写进代码，无需再改）：
- `MODEL_NAME = "cinema-generate-2.0"`
- 视频参数走 `metadata={"duration":5,"ratio":"16:9","resolution":"720p"}`（顶层 width/height/float duration 会被 new-api 丢弃）
- 结果视频地址从 `metadata.url` 读取
- 异常会打印 `状态码` + `响应体`，便于排错

---

## 6. 验证

期望输出：
```
📹 创建视频生成任务...
📝 创建响应: { ... "status": "queued" ... }
🆔 任务 ID: task_xxxx
尝试 n/60: 任务状态 = in_progress
✅ 视频生成完成！
🎬 视频地址: https://...
```

---

## 7. 排错速查

| 现象 | 原因 / 处理 |
|---|---|
| 后台没有「JDCloud」选项 | 前端没重建，回第 3 步 |
| 调用报 `503 无可用渠道` | 渠道分组 ≠ 令牌分组，或渠道没加 `cinema-generate-2.0` |
| 调用报 `500` | 看 new-api 运行日志；多半 AppKey 不对或灵境域名不对 |
| 一直 `in_progress` | 查日志里 `queryTasKResult` 返回；确认 AppKey 有 Cinema2.0 权限+额度 |
| 完成但无视频地址 | 非白名单用户灵境只回 `watermarkUrl`（代码已兜底）；若都没有查控制台权限 |

---

## 8. ⚠️ 升级 new-api 时注意

new-api 出新版本时，**上面第 2 节的改动会被覆盖**，需重新应用。重打补丁顺序：
1. `constant/channel.go` 加常量 59 / BaseURL / 名称
2. 拷回整个 `relay/channel/task/jdcloud/` 目录
3. `relay/relay_adaptor.go` 加 import + switch case
4. `controller/channel-test.go` 加到不支持测试清单
5. 前端 `constants.ts` + en/zh.json 加对应项
6. 重新 `make build-all-frontends && go build`

> 若新版 new-api 已经官方支持了京东云灵境，就可以撤掉本补丁直接用官方实现。

---

## 9. 扩展点

- **图生视频 / 参考生视频**：当前只做文生视频（与 `video_api.py` 一致）。适配器已解析 `content[].image_url`，
  扩展时把它透传到灵境对应接口（注意 apiId 可能不同）。
- **查询域名**：默认 `https://model.jdcloud.com`；若灵境文档/控制台对查询接口给了单独域名，
  改 `jdcloud/constants.go` 的 `queryPath` 前缀或拆分提交/查询 BaseURL。
- **其它 locale**（fr/ru/ja/vi）：未加 `JDCloud` 键，会回退显示 `JDCloud`；如需本地化在对应 `locales/*.json` 加。
