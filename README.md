# kilncurve-release

`kilncurve-release` 是面向陶瓷烧成工艺工程师、窑炉试烧操作员和质量复核员的烧成曲线试烧定版工作台。系统把原料与设备边界、分段曲线、试烧证据、自动判偏、整改复试和工艺卡签发放在一条受控流程中，避免未经验证的曲线进入批量生产。

## 业务流程

课题状态按以下路径推进：

1. 工艺工程师建立带唯一编号的课题，登记坯体、釉料、装窑方式、窑炉能力和成品质量边界。
2. 草拟且尚无冻结曲线时，工艺工程师可修订适用边界；系统按固定字段顺序记录原因、操作者及前后值，课题编号始终不可变。
3. 工艺工程师编排升温、保温和降温段。系统逐段校验温度连续性、方向、斜率、保温时长、峰值和总周期，并可比较同课题两个修订的分段及整体指标差异。
4. 合法曲线被冻结并生成 SHA-256 内容摘要。冻结版本不能原位修改，只能从选定冻结版本完整复制出带来源引用的可编辑派生版。
5. 试烧操作员按冻结版本开始 `RECORDING` 草稿，分批保存严格递增的测温点和五项质检值；详情持续返回缺失项和完整度，确认完整后才原子冻结证据并正式评估。
6. 系统自动评估测温轨迹、成熟温度和成品质量；失败检查形成可追踪偏差，不完整证据不能进入复核。
7. 工艺工程师可把同一失败冻结版本下的多个开放偏差组成整改批次，关联合法冻结派生版；系统自动生成去重、稳定排序的定向复试清单，复试后逐项关闭或重新开放。
8. 全部偏差关闭后，质量复核员可批准或退回。批准会签发包含曲线快照、适用边界、证据索引和摘要的不可变工艺卡。

浏览器请求中的状态命令都携带 `expectedVersion` 和 `idempotencyKey`。前者用于乐观并发控制，后者随本地快照持久化，页面重试不会重复冻结、评估或签发。

## 构建与运行

标准构建：

```text
go build ./...
```

启动服务：

```text
go run ./cmd/server -addr=127.0.0.1:19081
```

然后访问 `http://127.0.0.1:19081/`。默认监听地址为 `127.0.0.1:19081`，仅允许回环地址。显式 `-addr` 优先；未提供时若 `PORT` 是合法端口号，则绑定 `127.0.0.1:<PORT>`。可用 `-data` 指定 JSON 快照位置，默认是 `.kilncurve/state.json`。

执行测试：

```text
go test ./...
```

执行真实 HTTP 自检：

```text
go run ./cmd/server -selfcheck -addr=127.0.0.1:19082 -timeout=20s
```

自检会使用临时持久化目录，启动真实回环监听器，经 JSON HTTP 完成建题、曲线冻结、失败试烧、偏差整改、派生版本、定向复试、质量批准、工艺卡签发和独立完整性核验，随后正常关闭服务并释放端口。

## 数据与完整性

业务状态保存在带 `schemaVersion` 的单一 JSON 快照中。每次提交在单一写入协调器内完成：复制当前状态、校验引用和工艺卡摘要、追加审计摘要链、写临时文件、`Sync`、原子 `Rename`，成功后才发布新的内存状态。启动时会拒绝未来版本、损坏引用、断裂审计链或摘要不一致的工艺卡。

系统不连接外部服务，也不引入 Node 构建链。`internal/web/static/index.html`、`app.css` 和 `app.js` 由 Go 服务同源提供。

## 主要 HTTP 接口

- `GET /`：单页工作台。
- `GET /api/health`：服务及持久化完整性就绪检查。
- `POST /api/projects`、`GET /api/projects`：建题和课题清单。
- `GET /api/projects/{projectId}`：详情、检查矩阵、时间线和工艺卡。
- `PATCH /api/projects/{projectId}/boundaries`：草拟课题适用边界修订及留痕。
- `POST /api/projects/{projectId}/curve/validate`：定位曲线错误。
- `GET /api/projects/{projectId}/revisions/compare`：比较本课题两个曲线修订。
- `POST /api/projects/{projectId}/revisions`、`PUT /api/projects/{projectId}/revisions/{revisionId}`：新建或编辑可修改曲线。
- `POST /api/projects/{projectId}/revisions/{revisionId}/derive`：从选定冻结修订安全派生。
- `POST /api/projects/{projectId}/revisions/{revisionId}/freeze`：冻结曲线并生成摘要。
- `POST /api/projects/{projectId}/trial-runs/drafts`：开始并锁定试烧草稿。
- `PUT /api/projects/{projectId}/trial-runs/{runId}/evidence`：保存草稿证据快照。
- `POST /api/projects/{projectId}/trial-runs/{runId}/complete`：确认完整并执行一次正式评估。
- `POST /api/projects/{projectId}/trial-runs`：兼容原有的一次性提交并评估入口。
- `POST /api/projects/{projectId}/deviation-batches`：原子建立偏差整改批次及复试清单。
- `POST /api/projects/{projectId}/deviations/{deviationId}/correct`：登记偏差原因和纠正动作。
- `POST /api/projects/{projectId}/review`：质量批准或退回。
- `GET /api/process-cards/{cardId}/verify`：独立重算并核验工艺卡摘要。
