# 问数/问答平台 Kubernetes 资源需求文档

## 文档信息

| 项目 | 内容 |
| --- | --- |
| 文档版本 | v1.1 |
| 编写日期 | 2026-05-08 |
| 适用系统 | 问数/问答平台 |
| 部署环境 | Kubernetes 集群 |
| 数据来源 | AIDGP 平台 |
| 本系统定位 | 应用层、权限层、会话层、指标聚合缓存层 |
| 用户规模 | 1000 用户 |
| 峰值并发 | 50-100 |

---

# 一、业务规模概述

## 1.1 系统边界

本项目不作为完整数据中台，不保存 AIDGP 的全量业务明细、全量知识库文档和检索索引。

本项目主要负责：

| 类型 | 是否本地存储 | 说明 |
| --- | --- | --- |
| 用户登录态 | 是 | 保存本系统登录、会话、Token 或 Cookie 状态 |
| 用户权限映射 | 是 | 保存问数主题权限、问答知识库权限、权限版本 |
| 问答会话记录 | 是 | 保存用户问题、助手回答、引用 ID、会话上下文 |
| 热门问题统计 | 是 | 保存 Top 问题、用户维度统计 |
| 指标聚合数据 | 是 | 保存从 AIDGP 拉取或接收后的轻量聚合结果 |
| AIDGP 原始明细数据 | 否 | 由 AIDGP 平台保存 |
| AIDGP 知识库文档原文 | 否 | 由 AIDGP 平台保存 |
| 向量索引/全文索引 | 否 | 由 AIDGP 或外部检索服务提供 |
| 大模型推理 | 否 | 使用外部 LLM 服务 |
| ASR 语音识别 | 否 | 使用外部 ASR 服务 |

## 1.2 用户规模

| 指标 | 数值 | 说明 |
| --- | --- | --- |
| 总用户数 | 1000 | 系统注册用户总数 |
| 日活用户 DAU | 300-500 | 按 30%-50% 日活率估算 |
| 并发用户峰值 | 50-100 | 高峰时段同时使用问数/问答 |
| 问数主题数 | 3+ | 网格、人流、车流，后续可扩展 |
| 问答知识库数 | 2-10 | 来自 AIDGP 平台 |
| 外部依赖 | AIDGP、LLM、ASR、联网搜索 | 本系统通过接口调用 |

## 1.3 流量估算

| 指标 | 数值 | 计算依据 |
| --- | --- | --- |
| 日均问数/问答请求 | 5000-15000 次 | 300-500 DAU × 10-30 次/人 |
| 平均 QPS | 2-5 | 按 8 小时工作日估算 |
| 峰值 QPS | 30-80 | 高峰时段集中提问、权限刷新、SSE 建连 |
| SSE 并发连接 | 50-100 | 问数/问答流式输出 |
| AIDGP 查询请求 | 3000-10000 次/日 | 问数分析、问答检索、知识库查询 |
| 权限校验请求 | 10000-30000 次/日 | 首页、问数、问答、管理端均会校验 |
| 指标聚合写入 | 低到中频 | 定时同步或事件触发聚合 |
| 语音转写请求 | 视使用频率而定 | 依赖 ASR 服务，按音频时长计费 |

## 1.4 性能要求

| 场景 | 要求 | 说明 |
| --- | --- | --- |
| 问数查询首字响应 | P95 ≤ 3 秒 | 首字可先返回解析/查询进度，AIDGP 查询结果后续流式输出 |
| 问答首字响应 | P95 ≤ 5 秒 | 包含权限校验、AIDGP 检索、LLM 首 token |
| 主页主题入口加载 | P95 ≤ 1.5 秒 | 主题权限和入口数据走 Redis 缓存 |
| 权限变更生效时间 | P95 ≤ 5 秒 | Redis 权限缓存主动失效或版本号刷新 |
| 普通接口响应 | P95 ≤ 1 秒 | 不包含 AIDGP、LLM、ASR、长查询 |
| 单次 AIDGP 查询超时 | 10-30 秒 | 防止外部依赖拖垮后端 |
| 单次问答生成超时 | ≤ 60 秒 | 防止 SSE 长连接无限占用 |

---

# 二、服务器资源清单

## 2.1 集群节点规划

由于全量数据、知识库、检索索引均由 AIDGP 平台承载，本项目不需要部署大型数据库集群、向量库或全文检索集群。

推荐生产部署如下：

| 节点类型 | 数量 | CPU | 内存 | 系统盘 | 数据盘 | 用途说明 |
| --- | --- | --- | --- | --- | --- | --- |
| Master 节点 | 3 | 4 核 | 8GB | 100GB SSD | - | K8s 控制平面，etcd 存储 |
| App Worker 节点 | 3 | 8 核 | 16GB | 100GB SSD | - | 前端、后端、Ingress、异步任务 |
| Data Worker 节点 | 1 | 8 核 | 32GB | 100GB SSD | 300GB SSD | MySQL/PostgreSQL、Redis、轻量聚合数据 |
| Monitor Worker 节点 | 1 | 4 核 | 8GB | 100GB SSD | 100GB SSD | Prometheus、Grafana、日志组件 |
| 合计 | 8 | 48 核 | 112GB | 800GB | 400GB | - |

如果数据库和 Redis 也采用云托管，K8s 节点可进一步简化：

| 节点类型 | 数量 | CPU | 内存 | 系统盘 | 数据盘 | 用途说明 |
| --- | --- | --- | --- | --- | --- | --- |
| Master 节点 | 3 | 4 核 | 8GB | 100GB SSD | - | K8s 控制平面 |
| App Worker 节点 | 3 | 8 核 | 16GB | 100GB SSD | - | 前端、后端、Ingress、异步任务 |
| Monitor Worker 节点 | 1 | 4 核 | 8GB | 100GB SSD | 100GB SSD | 监控、日志、告警 |
| 合计 | 7 | 40 核 | 80GB | 700GB | 100GB | - |

## 2.2 外部依赖资源

| 外部系统 | 用途 | 资源归属 | 本项目要求 |
| --- | --- | --- | --- |
| AIDGP 平台 | 数据查询、知识库检索、权限/知识库同步 | AIDGP 承载 | 需提供稳定 API、鉴权、超时、限流约定 |
| LLM 服务 | 问数解释、问答生成 | 外部模型服务承载 | 需支持流式输出 |
| ASR 服务 | 语音转文字 | 外部语音服务承载 | 需支持音频文件转写 |
| 联网搜索服务 | 外部网页检索 | 外部搜索服务承载 | 可选，需支持超时降级 |
| 对象/文档存储 | 文档原文、附件 | AIDGP 或对象存储承载 | 本项目只保存引用 ID 或跳转链接 |

## 2.3 服务器配置清单

以本项目自带轻量数据库和 Redis 的方案为例：

| 序号 | 主机名 | 角色 | CPU | 内存 | 系统盘 | 数据盘 | IP 地址示例 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | chatdb-master-01 | Master | 4 核 | 8GB | 100GB | - | 192.168.1.11 |
| 2 | chatdb-master-02 | Master | 4 核 | 8GB | 100GB | - | 192.168.1.12 |
| 3 | chatdb-master-03 | Master | 4 核 | 8GB | 100GB | - | 192.168.1.13 |
| 4 | chatdb-app-01 | Worker(App) | 8 核 | 16GB | 100GB | - | 192.168.1.21 |
| 5 | chatdb-app-02 | Worker(App) | 8 核 | 16GB | 100GB | - | 192.168.1.22 |
| 6 | chatdb-app-03 | Worker(App) | 8 核 | 16GB | 100GB | - | 192.168.1.23 |
| 7 | chatdb-data-01 | Worker(Data) | 8 核 | 32GB | 100GB | 300GB | 192.168.1.31 |
| 8 | chatdb-monitor-01 | Worker(Monitor) | 4 核 | 8GB | 100GB | 100GB | 192.168.1.41 |

---

# 三、Kubernetes 工作负载资源

## 3.1 Namespace 划分

| Namespace | 用途 | 资源配额 |
| --- | --- | --- |
| chatdb | 问数/问答业务应用命名空间 | CPU: 32 核，内存: 64GB |
| chatdb-data | 轻量数据库和 Redis，云托管时可不部署 | CPU: 8 核，内存: 32GB |
| chatdb-monitoring | 监控、日志、告警命名空间 | CPU: 6 核，内存: 12GB |
| kube-system | K8s 系统组件 | 系统预留 |
| ingress-nginx | Ingress 控制器 | CPU: 4 核，内存: 8GB |

## 3.2 应用层工作负载

Namespace：`chatdb`

| 工作负载名称 | 类型 | 副本数 | CPU 请求 | CPU 限制 | 内存请求 | 内存限制 | 存储 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| chatdb-backend | Deployment | 3 | 1 核 | 4 核 | 2GB | 8GB | - |
| chatdb-frontend | Deployment | 2 | 500m | 2 核 | 1GB | 4GB | - |
| chatdb-worker | Deployment | 1-2 | 500m | 2 核 | 1GB | 4GB | - |
| ingress-nginx-controller | Deployment | 2 | 500m | 2 核 | 1GB | 4GB | - |

说明：

- `chatdb-backend` 承载问数、问答、SSE、权限、管理端接口。
- `chatdb-frontend` 承载前端页面和 BFF 代理接口。
- `chatdb-worker` 用于 AIDGP 同步、指标聚合计算、车流聚合、热门问题统计。
- 问数/问答使用 SSE，后端建议至少 3 副本。

## 3.3 数据层工作负载

Namespace：`chatdb-data`

本系统只保存少量业务状态和指标聚合结果，不保存 AIDGP 全量明细数据。

| 工作负载名称 | 类型 | 副本数 | CPU 请求 | CPU 限制 | 内存请求 | 内存限制 | 存储 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| mysql 或 postgresql | StatefulSet | 1 主 1 备 | 2 核 | 4 核 | 4GB | 8GB | 100GB × 2 |
| redis | StatefulSet | 1 主 1 备 | 500m | 2 核 | 2GB | 4GB | 20GB × 2 |

本地数据库主要保存：

| 数据 | 说明 |
| --- | --- |
| 用户会话 | 登录态、问答会话、问数会话 |
| 权限映射 | 用户与问数主题、问答知识库的映射 |
| 权限版本 | 支持权限变更 5 秒内生效 |
| 问答消息 | 用户问题、助手回答、引用 ID |
| 热门问题 | Top 问题统计 |
| 指标聚合 | 从 AIDGP 拉取或接收后的轻量汇总数据 |
| 同步任务 | AIDGP 同步任务状态、错误信息、更新时间 |

不在本地保存：

| 数据 | 说明 |
| --- | --- |
| AIDGP 全量业务明细 | 通过 AIDGP 查询接口获取 |
| 全量知识库文档 | 由 AIDGP 保存 |
| 文档全文索引 | 由 AIDGP 或其检索服务提供 |
| 向量索引 | 由 AIDGP 或外部向量服务提供 |
| 大模型权重 | 使用外部 LLM 服务 |

## 3.4 监控层工作负载

Namespace：`chatdb-monitoring`

| 工作负载名称 | 类型 | 副本数 | CPU 请求 | CPU 限制 | 内存请求 | 内存限制 | 存储 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| prometheus | StatefulSet | 1 | 1 核 | 2 核 | 2GB | 4GB | 50GB |
| grafana | Deployment | 1 | 200m | 500m | 512MB | 1GB | 10GB |
| alertmanager | Deployment | 1 | 100m | 500m | 256MB | 512MB | 5GB |
| loki | StatefulSet | 1 | 500m | 2 核 | 1GB | 4GB | 50GB |
| promtail | DaemonSet | 每节点 | 100m | 200m | 128MB | 256MB | - |
| node-exporter | DaemonSet | 每节点 | 50m | 100m | 128MB | 256MB | - |

## 3.5 网络服务资源

| 服务名称 | 类型 | 端口 | 对外暴露 | 说明 |
| --- | --- | --- | --- | --- |
| chatdb-frontend-svc | ClusterIP | 3000 | Ingress | 前端页面 |
| chatdb-backend-svc | ClusterIP | 8000 或 9000 | Ingress | 后端 API 服务 |
| chatdb-worker-svc | ClusterIP | - | 否 | 异步任务服务 |
| mysql-svc | ClusterIP | 3306 | 否 | 本地轻量数据库 |
| postgresql-svc | ClusterIP | 5432 | 否 | 可替代 MySQL |
| redis-svc | ClusterIP | 6379 | 否 | 缓存服务 |
| prometheus-svc | ClusterIP | 9090 | Ingress 或内网 | 监控服务 |
| grafana-svc | ClusterIP | 3000 | Ingress 或内网 | 可视化服务 |

---

# 四、存储资源需求

## 4.1 持久卷 PV 清单

| PV 名称 | 容量 | 存储类 | 访问模式 | 绑定 PVC | 用途 |
| --- | --- | --- | --- | --- | --- |
| pv-db-data-0 | 100GB | ssd-storage | RWO | db-data-0 | 数据库主库数据 |
| pv-db-data-1 | 100GB | ssd-storage | RWO | db-data-1 | 数据库备库数据 |
| pv-redis-data-0 | 20GB | ssd-storage | RWO | redis-data-0 | Redis 节点 0 数据 |
| pv-redis-data-1 | 20GB | ssd-storage | RWO | redis-data-1 | Redis 节点 1 数据 |
| pv-prometheus | 50GB | ssd-storage | RWO | prometheus-data | 监控指标数据 |
| pv-loki | 50GB | ssd-storage | RWO | loki-data | 日志数据 |
| pv-grafana | 10GB | ssd-storage | RWO | grafana-data | Grafana 配置存储 |

## 4.2 存储类 StorageClass 定义

| 存储类名称 | 后端存储 | 回收策略 | 绑定模式 | 默认 |
| --- | --- | --- | --- | --- |
| ssd-storage | SSD 云盘或本地 SSD | Retain | WaitForFirstConsumer | 是 |
| nfs-storage | NFS | Delete | Immediate | 否 |
| object-storage | S3/OSS/MinIO | Retain | Immediate | 否 |

## 4.3 业务数据存储估算

| 数据类型 | 建议容量 | 说明 |
| --- | --- | --- |
| 本地业务数据库 | 100GB-200GB | 用户权限、会话、消息、热门问题、聚合指标 |
| Redis 数据 | 20GB-40GB | 权限缓存、会话状态、限流、热点数据 |
| 日志数据 | 50GB-100GB | 按 7-15 天保留估算 |
| 监控指标 | 50GB | Prometheus 15 天左右保留 |
| AIDGP 明细数据 | 不在本项目存储 | 由 AIDGP 平台承载 |
| AIDGP 知识库文档 | 不在本项目存储 | 由 AIDGP 平台承载 |
| 检索/向量索引 | 不在本项目存储 | 由 AIDGP 或外部检索服务承载 |

---

# 五、网络资源需求

## 5.1 网络规划

| 网络类型 | CIDR | 说明 |
| --- | --- | --- |
| 节点网络 | 192.168.1.0/24 | 物理节点或云主机通信 |
| Pod 网络 | 10.244.0.0/16 | Calico CNI 网络 |
| Service 网络 | 10.96.0.0/12 | ClusterIP 服务网络 |
| Ingress 入口 | 按实际负载均衡规划 | 对外提供 HTTPS 访问 |
| AIDGP 专线/内网 | 按实际环境规划 | 建议使用内网或专线访问 AIDGP |

## 5.2 域名与证书

| 域名 | 用途 | 证书类型 |
| --- | --- | --- |
| chatdb.example.com | 问数/问答平台访问 | SSL 证书 |
| api.chatdb.example.com | 后端 API，可选 | SSL 证书 |
| grafana.chatdb.example.com | 监控面板 | SSL 证书 |
| prometheus.chatdb.example.com | Prometheus，可限制内网访问 | SSL 证书 |

## 5.3 Ingress 路由规则

| 路径 | 后端服务 | 端口 | 说明 |
| --- | --- | --- | --- |
| `/` | chatdb-frontend-svc | 3000 | 前端页面 |
| `/api` | chatdb-frontend-svc | 3000 | 前端 BFF 代理接口 |
| `/backend-api` | chatdb-backend-svc | 8000 或 9000 | 后端 API，可选 |
| `/swagger` | chatdb-backend-svc | 8000 或 9000 | API 文档，可选 |

## 5.4 SSE Ingress 配置

问数/问答使用 SSE 流式输出，Ingress 必须配置长连接和关闭缓冲。

建议 Nginx Ingress 注解：

```yaml
nginx.ingress.kubernetes.io/proxy-read-timeout: "300"
nginx.ingress.kubernetes.io/proxy-send-timeout: "300"
nginx.ingress.kubernetes.io/proxy-buffering: "off"
nginx.ingress.kubernetes.io/proxy-request-buffering: "off"
```

## 5.5 AIDGP 调用要求

| 项目 | 要求 |
| --- | --- |
| 网络链路 | 优先内网、专线或同 VPC 访问 |
| 调用超时 | 普通查询 10 秒，复杂查询 30 秒 |
| 重试策略 | 幂等接口可重试 1-2 次 |
| 限流策略 | 本项目侧按用户、接口、主题限流 |
| 熔断策略 | AIDGP 异常时返回降级提示，避免拖垮后端 |
| 缓存策略 | 热门指标、权限、知识库列表可短时缓存 |

---

# 六、安全资源配置

## 6.1 密钥与配置

| 资源名称 | 类型 | 用途 |
| --- | --- | --- |
| db-secret | Secret | 本地数据库用户名、密码 |
| redis-secret | Secret | Redis 密码 |
| jwt-secret | Secret | 登录态/JWT 签名密钥 |
| aidgp-secret | Secret | AIDGP API Key、Client Secret |
| llm-secret | Secret | LLM API Key |
| asr-secret | Secret | ASR API Key |
| web-search-secret | Secret | 联网搜索 API Key |
| tls-secret | Secret | SSL 证书私钥 |
| chatdb-config | ConfigMap | 后端业务配置 |
| frontend-config | ConfigMap | 前端环境配置 |
| ingress-config | ConfigMap | Ingress 全局配置 |

## 6.2 关键环境变量

后端：

```env
DB_HOST=
DB_PORT=
DB_USER=
DB_PASSWORD=
DB_NAME=

REDIS_HOST=
REDIS_PORT=
REDIS_PASSWORD=

AIDGP_BASE_URL=
AIDGP_CLIENT_ID=
AIDGP_CLIENT_SECRET=
AIDGP_TIMEOUT_SECONDS=

CHATDB_LLM_ENDPOINT=
CHATDB_LLM_API_KEY=

CHATDB_ASR_PROVIDER=
CHATDB_ASR_ENDPOINT=
CHATDB_ASR_API_KEY=

CHATDB_WEB_SEARCH_ENABLED=
CHATDB_WEB_SEARCH_ENDPOINT=
CHATDB_WEB_SEARCH_API_KEY=
```

前端：

```env
CHATDB_API_BASE_URL=http://chatdb-backend-svc:8000
NEXT_PUBLIC_APP_NAME=问答问数平台
```

## 6.3 网络策略

| 策略名称 | 作用范围 | 规则 |
| --- | --- | --- |
| chatdb-network-policy | chatdb 命名空间 | 限制业务 Pod 间通信 |
| backend-egress-policy | 后端 Pod | 仅允许访问数据库、Redis、AIDGP、LLM、ASR、搜索服务 |
| db-network-policy | 数据库 Pod | 仅允许后端和任务服务访问 |
| redis-network-policy | Redis Pod | 仅允许后端、前端 BFF、任务服务访问 |
| monitoring-network-policy | 监控命名空间 | 限制监控入口访问 |

---

# 七、资源配额汇总

## 7.1 CPU 资源汇总

| 用途 | 请求值 | 限制值 |
| --- | --- | --- |
| Master 节点系统预留 | 1 核/节点 | 2 核/节点 |
| Worker 节点系统预留 | 500m/节点 | 1 核/节点 |
| chatdb-backend | 3 核 | 12 核 |
| chatdb-frontend | 1 核 | 4 核 |
| chatdb-worker | 500m-1 核 | 2-4 核 |
| Ingress Controller | 1 核 | 4 核 |
| Redis | 1 核 | 4 核 |
| 本地数据库 | 2 核 | 4 核 |
| Prometheus | 1 核 | 2 核 |
| Grafana | 200m | 500m |
| Loki | 500m | 2 核 |
| 合计 | 约 15-18 核 | 约 42-48 核 |

## 7.2 内存资源汇总

| 用途 | 请求值 | 限制值 |
| --- | --- | --- |
| Master 节点系统预留 | 2GB/节点 | 4GB/节点 |
| Worker 节点系统预留 | 1GB/节点 | 2GB/节点 |
| chatdb-backend | 6GB | 24GB |
| chatdb-frontend | 2GB | 8GB |
| chatdb-worker | 1-2GB | 4-8GB |
| Ingress Controller | 2GB | 8GB |
| Redis | 4GB | 8GB |
| 本地数据库 | 4GB | 8GB |
| Prometheus | 2GB | 4GB |
| Grafana | 512MB | 1GB |
| Loki | 1GB | 4GB |
| 合计 | 约 35-40GB | 约 85-95GB |

## 7.3 存储资源汇总

| 类型 | 容量 | 说明 |
| --- | --- | --- |
| 系统盘 | 700GB-800GB | 7-8 节点 × 100GB |
| 本地数据库数据盘 | 200GB | 主备各 100GB |
| Redis 数据盘 | 40GB | 两节点各 20GB |
| 监控日志盘 | 100GB-200GB | Prometheus、Loki、Grafana |
| 临时缓存盘 | 50GB-100GB | 文件上传临时目录、任务缓存 |
| AIDGP 数据存储 | 不计入本项目 | 由 AIDGP 平台承担 |
| 合计 | 约 1.1TB-1.3TB | 本项目侧资源估算 |

---

# 八、弹性伸缩配置

## 8.1 HPA 配置

| 工作负载 | 最小副本 | 最大副本 | CPU 阈值 | 内存阈值 |
| --- | --- | --- | --- | --- |
| chatdb-backend | 3 | 8 | 60% | 75% |
| chatdb-frontend | 2 | 5 | 60% | 75% |
| chatdb-worker | 1 | 4 | 70% | 80% |
| ingress-nginx-controller | 2 | 4 | 60% | 75% |

## 8.2 建议扩容指标

| 指标 | 建议阈值 | 说明 |
| --- | --- | --- |
| SSE 活跃连接数 | 单后端 Pod 100-300 | 超过后增加副本 |
| 问数首字响应 P95 | > 3 秒 | 检查 AIDGP 查询、LLM、后端排队 |
| 问答首字响应 P95 | > 5 秒 | 检查 AIDGP 检索、LLM 首 token |
| AIDGP 调用 P95 | > 2-5 秒 | 需要和 AIDGP 平台联合排查 |
| DB 连接池占用 | > 80% | 本地状态库压力过高 |
| Redis 内存占用 | > 70% | 需要扩容或优化缓存 TTL |
| LLM 首 token 延迟 | > 3 秒 | 需要优化模型服务或切换供应商 |

## 8.3 VPA 配置，可选

| 工作负载 | 更新模式 | 说明 |
| --- | --- | --- |
| prometheus | Auto | 根据监控数据量自动调整 |
| loki | Auto | 根据日志量自动调整 |
| chatdb-worker | Initial | 根据同步任务规模建议资源 |

---

# 九、成本估算，参考

## 9.1 云服务器月度成本

以本项目保留轻量数据库和 Redis 的方案估算，实际以云厂商报价为准。

| 节点类型 | 规格 | 数量 | 单价，元/月 | 小计，元/月 |
| --- | --- | --- | --- | --- |
| Master 节点 | 4 核 8G | 3 | 400 | 1200 |
| App Worker | 8 核 16G | 3 | 800 | 2400 |
| Data Worker | 8 核 32G | 1 | 1200 | 1200 |
| Monitor Worker | 4 核 8G | 1 | 400 | 400 |
| SSD 云盘 | 400GB | - | 1 元/GB/月 | 400 |
| 合计 | - | - | - | 5600 |

如果数据库和 Redis 使用云托管：

| 资源 | 规格 | 参考费用 |
| --- | --- | --- |
| K8s 节点 | 3 Master + 3 App + 1 Monitor | 4000-5000 元/月 |
| 云数据库 | 2-4 核，8-16GB，100GB 起 | 800-2500 元/月 |
| 云 Redis | 1-2 核，2-4GB | 300-1000 元/月 |
| 合计 | - | 5100-8500 元/月 |

说明：

- AIDGP 平台成本不计入本项目。
- LLM、ASR、联网搜索通常按调用量计费，不包含在基础服务器成本内。
- 如果问答使用频繁，LLM token 成本可能高于本项目服务器成本。

## 9.2 私有化部署成本，参考

| 项目 | 配置 | 数量 | 说明 |
| --- | --- | --- | --- |
| Master 服务器 | 4 核 8G | 3 台 | 控制平面 |
| App 服务器 | 8 核 16G | 3 台 | 前端、后端、任务 |
| Data 服务器 | 8 核 32G，SSD | 1 台 | 轻量数据库、Redis |
| Monitor 服务器 | 4 核 8G | 1 台 | 监控日志 |
| 交换机 | 千兆或万兆 | 2 台 | 按机房标准 |
| 存储 | SSD 1TB 起 | 1 套 | 本地状态数据、日志、监控 |

---

# 十、上线前必须确认事项

## 10.1 与 AIDGP 平台的接口约定

| 项目 | 要求 |
| --- | --- |
| 鉴权方式 | 明确 Token、AK/SK、OAuth 或专线鉴权 |
| 知识库列表接口 | 返回当前用户可访问知识库 |
| 知识库检索接口 | 支持问题、知识库范围、TopK、引用信息 |
| 问数数据查询接口 | 支持网格、人流、车流等主题查询 |
| 指标聚合接口 | 支持按日期、区域、主题拉取汇总指标 |
| 文档原文定位接口 | 支持 citationId 或 documentId 跳转 |
| 权限同步接口 | 支持用户和知识库/主题权限同步 |
| 超时和错误码 | 需要统一错误码、超时、限流和重试策略 |

## 10.2 性能保障

| 项目 | 要求 |
| --- | --- |
| 问数/问答必须使用 SSE 流式响应 | 首字不等待完整回答 |
| AIDGP 查询必须设置超时 | 普通 10 秒，复杂 30 秒 |
| AIDGP 调用必须支持降级 | 外部异常时返回明确提示 |
| 热门指标和权限必须缓存 | Redis 缓存并支持主动失效 |
| LLM、ASR、搜索服务必须设置超时 | 防止外部调用拖垮后端 |
| 重要接口必须限流 | 用户级、IP 级、接口级限流 |
| 指标聚合建议异步化 | 避免主页或问数首字等待聚合计算 |

## 10.3 运维保障

| 项目 | 要求 |
| --- | --- |
| 本地数据库备份 | 每日全量，关键表增量或 binlog |
| Redis 持久化 | 开启 RDB/AOF，按业务要求选择 |
| 日志保留 | 建议 7-15 天 |
| 监控指标 | CPU、内存、QPS、错误率、P95、P99、SSE 连接数、AIDGP 调用耗时 |
| 告警 | 接口错误率、AIDGP 超时、数据库连接池、LLM 超时、磁盘空间 |
| 灰度发布 | 后端和前端均支持滚动更新 |
| 回滚策略 | 保留上一版本镜像和配置 |

---

# 十一、结论

在“数据全部从 AIDGP 平台获取，本项目只保存少量指标聚合数据”的前提下，本项目 Kubernetes 资源需求可以明显降低。

推荐生产配置：

| 类型 | 推荐 |
| --- | --- |
| K8s 节点 | 3 Master + 3 App Worker + 1 Data Worker + 1 Monitor Worker |
| 总 CPU | 约 48 核 |
| 总内存 | 约 112GB |
| 总存储 | 约 1.1TB-1.3TB |
| 后端副本 | 3 起，HPA 到 8 |
| 前端副本 | 2 起，HPA 到 5 |
| 本地数据库 | 2-4 核，8GB，100GB 起 |
| Redis | 1-2 核，2-4GB，20GB 起 |
| 检索/向量库 | 不在本项目部署，由 AIDGP 承载 |
| 全量业务数据 | 不在本项目存储，由 AIDGP 承载 |

如果数据库和 Redis 也采用云托管，K8s 集群可进一步降低到：

| 类型 | 推荐 |
| --- | --- |
| K8s 节点 | 3 Master + 3 App Worker + 1 Monitor Worker |
| 总 CPU | 约 40 核 |
| 总内存 | 约 80GB |
| 总存储 | 约 900GB-1TB |

---

# 十二、附录

## 12.1 参考标准

| 标准 | 说明 |
| --- | --- |
| K8s 版本 | v1.28+ |
| 容器运行时 | containerd |
| CNI 插件 | Calico |
| CSI 驱动 | 根据云厂商或私有化存储选择 |
| Ingress | Nginx Ingress |
| 监控 | Prometheus + Grafana |
| 日志 | Loki + Promtail 或 ELK |

## 12.2 相关文档

| 文档名称 | 位置 |
| --- | --- |
| 问答开发计划 | `docs/qa-development-plan.md` |
| 车流开发计划 | `docs/traffic-flow-development-plan.md` |
| 两日功能完成情况 | `docs/2026-05-06-07-completion-summary.md` |

---

文档版本：v1.1

编写人：问数/问答平台开发团队

审批人：-

审批日期：-

