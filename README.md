# 古代金属冶炼遗址污染指纹识别与现代环境修复系统

> **v2.0 工程化版本** — 基于 Go 模块化架构 + Docker 编排 + Prometheus 监控

一套面向环境考古研究的全栈应用系统，集成古代金属冶炼遗址的重金属污染监测、污染指纹识别与环境修复评估功能。

---

## ✨ 功能特性

### 🌍 全球遗址分布可视化
- 基于 Leaflet + Canvas 绘制全球 30 个古代冶炼遗址分布图
- 遗址标记颜色根据污染程度（PI值）动态变化（绿→黄→橙→红）
- 标记大小根据冶炼规模分级（小型/中型/大型/超大型）
- **低缩放自动网格聚合**，减少渲染压力
- 支持按金属类型（铜、铁、银、铅、汞）筛选显示

### 📈 重金属浓度趋势分析
- 展示遗址近 10 年 Pb、Zn、Cu、As、Hg、Cd 六种重金属浓度变化
- **纯 Canvas 手绘多参数趋势曲线**，支持 DPR 高清屏适配
- 实时计算综合污染指数（PI）

### 🔬 污染指纹识别模型
- 基于重金属比率特征（Pb/Zn、Cu/Pb、As/Hg、Cd/Zn、Cu/As）
- 结合铅同位素比值（Pb206/Pb207、Pb208/Pb207）模拟数据
- **PCA 主成分分析** + **K-Means++ 聚类**建立特征库
- **Bootstrap 重采样** + **轮廓系数** + **Gap Statistic** 质量评估
- 内置 15 种典型冶炼工艺污染指纹
- 自动识别污染来源，计算相似度与加权距离

### 🏗️ 环境修复多属性决策系统
- 10 种主流修复技术数据库
- **AHP 层次分析法**（7x7 专家判断矩阵）+ **熵权法** + **动态 alpha** 组合权重
- **TOPSIS 正负理想解** + 相对接近度计算最终排名
- 支持**数据缺失自适应**（缺失属性权重归零，按比例重新分配）
- 输出 Top 10 推荐修复技术及分项评分

### ⚡ 污染预警与邮件推送
- 基于 GB36600-2018 风险管制标准自动检测超标
- 三级告警：超标预警（中）、修复预警（高）、重度污染（严重）
- 额外生态风险告警（PI ≥ 2.0）
- **时间窗口聚合推送**：严重立即单条发送，其他 30 分钟批量汇总
- SMTP 邮件自动推送告警通知

---

## 🏗️ 系统架构 (v2.0 工程化)

### 模块化架构（Channel 解耦）

```
                    ┌──────────────────────────────────────────┐
                    │              前端 (Frontend)              │
                    │  pollution_map.js  │  site_detail.js      │
                    │  (Leaflet地图)     │  (趋势/指纹/修复)   │
                    └──────────────────┬───────────────────────┘
                                       │
                                       ▼ 80 端口 (Nginx Gzip)
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│                                       Nginx 反向代理                                        │
│  Gzip 压缩 │ 静态资源缓存 │ /api 代理到 backend                                            │
└──────────────────────────────────────────┬─────────────────────────────────────────────────┘
                                           │
┌──────────────────────────────────────────▼─────────────────────────────────────────────────┐
│                                  Go 后端 (模块化架构 v2)                                    │
│                                                                                            │
│  ┌───────────────────────────────────┐    ┌─────────────────────────────────────┐          │
│  │   XRFReceiver (数据接收+入库)     │    │  EventBus (Go Channel 发布订阅)    │          │
│  │   └─ 污染指数计算                 │    │  - XRFReceived                     │          │
│  └─────────────┬─────────────────────┘    │  - FingerprintReady                │          │
│                │                          │  - RemediationReady                │          │
│                │  Publish [XRFReceived]   │  - AlertsGenerated                 │          │
│                └──────────────────────────►  - EmailSent                       │          │
│                                           └───┬───────────┬───────────┬─────────┘          │
│                                               │ Subscribe │ Subscribe │ Subscribe          │
│                                               ▼           ▼           ▼                    │
│                                  ┌──────────────┐  ┌──────────┐  ┌──────────┐           │
│                                  │ Fingerprint  │  │ Remedia- │  │  Alarm   │           │
│                                  │ Analyzer     │  │ tion     │  │  Mailer  │           │
│                                  │ PCA+聚类     │  │ Advisor  │  │  告警    │           │
│                                  │ 指纹匹配     │  │ AHP+熵权+│  │  +聚合   │           │
│                                  │              │  │ TOPSIS   │  │  邮件    │           │
│                                  └──────────────┘  └──────────┘  └──────────┘           │
│                                                                                            │
│  ◄─  :2112 /metrics (Prometheus)   ◄─ :6060 /debug/pprof (性能剖析)                         │
└──────────────────────────────────────────────┬──────────────────────────────────────────────┘
                                               │
                                    ┌──────────▼──────────┐
                                    │ PostgreSQL + PostGIS │
                                    │  BRIN/复合/GIN索引  │
                                    │  物化视图            │
                                    └─────────────────────┘
                                               ▲
                                               │ 定期上报
                                    ┌──────────┴──────────┐
                                    │  XRF 数据模拟器     │
                                    │  30遗址 │ 污染特征  │
                                    │  环境变量可配置      │
                                    └─────────────────────┘
                                               ▲
                                    ┌──────────┴──────────┐
                                    │  Prometheus 监控     │
                                    │  抓取 backend:2112   │
                                    └─────────────────────┘
```

### 事件流

```
XRF 数据上报
    │
    ▼
XRFReceiver.Receive()  →  入库 + 计算PI
    │
    └─ Publish [XRFReceived] ────────────┐
                                            │
                    ┌───────────────────────┼───────────────────────┐
                    ▼                       ▼                       ▼
        FingerprintAnalyzer         RemediationAdvisor          AlarmMailer
         │ PCA + KMeans                │ AHP + 熵权法              │ 三级告警检测
         │ Bootstrap 验证               │ TOPSIS 评分               │ 聚合推送
         │ 匹配指纹库                   │ 领域知识加分               │ SMTP 邮件
         ▼                             ▼                           ▼
    Publish [FingerprintReady]    Publish [RemediationReady]   Publish [AlertsGenerated]
                                                              Publish [EmailSent]
```

---

## 📁 项目结构

```
.
├── backend/                 # Go 后端 (v2 模块化架构)
│   ├── config/             # 配置
│   │   ├── config.go       # .env 加载
│   │   └── params.go       # ✨ 算法参数外置 (8个结构体)
│   ├── database/           # 数据库连接
│   │   └── database.go     # GetPool() + Init()
│   ├── handlers/           # HTTP 处理器
│   │   └── handlers.go     # 聚合 4 模块
│   ├── middleware/         # ✨ 中间件 (新增)
│   │   └── metrics.go      # Prometheus 指标中间件
│   ├── models/             # 数据模型
│   │   └── models.go       # 新旧字段兼容
│   ├── modules/            # ✨ 业务模块 (v2 核心)
│   │   ├── eventbus.go     # EventBus (Channel 发布订阅)
│   │   ├── xrf_receiver.go       # 模块1: XRF接收+入库
│   │   ├── fingerprint_analyzer.go # 模块2: PCA+聚类+指纹
│   │   ├── remediation_advisor.go  # 模块3: MCDM决策
│   │   └── alarm_mailer.go         # 模块4: 告警+聚合邮件
│   ├── repository/         # 数据访问层
│   │   └── repository.go   # +12 个兼容包装函数
│   ├── services/           # LEGACY (保留但不再使用)
│   ├── .env
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go             # pprof + Prometheus + 健康检查
│
├── frontend/               # 前端 (v2 模块化)
│   ├── index.html
│   ├── styles.css
│   ├── app.js              # 胶水层 + 筛选器
│   ├── pollution_map.js    # ✨ 地图模块 (Leaflet+Canvas+聚合)
│   └── site_detail.js      # ✨ 详情模块 (趋势/指纹/修复)
│
├── simulator/              # ✨ XRF 数据模拟器 (v2 增强)
│   ├── main.go             # 30 遗址 + 环境变量 + 特征注入
│   ├── Dockerfile
│   └── go.mod
│
├── database/               # 数据库
│   └── init.sql            # ✨ BRIN/复合/GIN索引 + 物化视图
│
├── nginx/                  # ✨ Nginx (新增)
│   ├── nginx.conf          # Gzip + 反向代理 + 缓存
│   ├── mime.types
│   └── Dockerfile
│
├── prometheus/             # ✨ Prometheus 监控 (新增)
│   └── prometheus.yml      # 抓取 backend + nginx
│
├── logs/                   # 日志目录 (自动创建)
│   └── nginx/
│
├── docker-compose.yml      # ✨ 5 服务编排 (v2)
├── .env.example            # 环境变量模板
└── README.md               # 本文件
```

---

## 🚀 快速部署

### 方式一：Docker Compose（推荐，生产级）

#### 1. 准备环境变量

```bash
# 复制环境变量模板
cp .env.example .env

# 按需修改 (SMTP 配置等)
vim .env
```

#### 2. 一键启动所有服务

```bash
# 构建并启动 (5 个服务)
docker compose up -d --build

# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f        # 所有服务
docker compose logs -f backend # 仅后端
docker compose logs -f simulator # 仅模拟器
```

#### 3. 访问服务

| 服务 | 地址 | 说明 |
|------|------|------|
| **前端** | http://localhost | Nginx Gzip 加速 |
| **API** | http://localhost/api/... | 经 Nginx 代理 |
| **pprof** | http://localhost:6060/debug/pprof/ | 性能剖析 |
| **Prometheus 指标** | http://localhost:2112/metrics | 业务指标 |
| **Prometheus UI** | http://localhost:9090 | 监控面板 |
| **健康检查** | http://localhost/healthz | Nginx |
| **健康检查** | http://localhost:8080/healthz | Go 后端 |

#### 4. 常用命令

```bash
# 停止所有服务
docker compose down

# 停止并删除数据卷 (慎用!)
docker compose down -v

# 重启某个服务
docker compose restart backend

# 只启动特定服务
docker compose up -d postgis backend
```

### 方式二：本地开发（无 Docker）

#### 1. 准备数据库

```bash
# 启动 PostgreSQL + PostGIS
docker run -d --name postgis \
  -e POSTGRES_DB=archaeology_pollution \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  postgis/postgis:16-3.4

# 导入数据
psql -h localhost -U postgres -d archaeology_pollution \
  -f database/init.sql
```

#### 2. 启动后端

```bash
cd backend
go mod download
go run main.go
```

#### 3. 启动模拟器（另一个终端）

```bash
cd simulator
go mod download
go run main.go
```

#### 4. 启动前端

```bash
# 方式1: 直接用 Go 内置静态服务
# 访问 http://localhost:8080

# 方式2: 用本地 Nginx
# 将 frontend/ 挂载到 Nginx 根目录
```

---

## 🔧 XRF 数据模拟器

### 功能特性

- ✅ **内置 30 个全球遗址**（与 init.sql 一一对应）
- ✅ **3 阶段上报模式**：历史数据回填 → 最新年度 → 持续模拟
- ✅ **环境变量全配置**（无需改代码）
- ✅ **污染特征注入**（动态覆盖金属基线）
- ✅ **异常高值模拟**（10% 概率触发告警）

### 环境变量配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `API_BASE` | `http://backend:8080` | 后端 API 地址 |
| `SIM_NUM_SITES` | `0` | 遗址数量（0=全部30个） |
| `SIM_REPORT_INTERVAL` | `30` | 持续上报间隔（秒） |
| `SIM_START_YEAR` | `2015` | 历史数据起始年 |
| `SIM_HISTORY_YEARS` | `10` | 历史数据年数 |
| `SIM_CONTINUOUS` | `true` | 是否持续上报 |
| `SIM_VERBOSE` | `true` | 详细日志 |
| `SIM_PROFILE_OVERRIDES` | `""` | 污染特征注入 |

### 污染特征注入

通过 `SIM_PROFILE_OVERRIDES` 环境变量可以动态修改不同金属的污染基线：

**格式**：`金属类型_属性=值,金属类型_属性=值,...`

**支持的金属类型**：铜、铁、银、铅、汞

**支持的属性**：PB、ZN、CU、AS、HG、CD（不区分大小写）

#### 示例 1：提高铜冶炼的 Pb 和 Cu 基线

```bash
SIM_PROFILE_OVERRIDES="铜_PB=600,铜_CU=15000"
```

#### 示例 2：让汞冶炼遗址严重超标（触发告警）

```bash
SIM_PROFILE_OVERRIDES="汞_HG=500,汞_AS=400"
```

#### 示例 3：多种金属同时调整

```bash
SIM_PROFILE_OVERRIDES="银_PB=5000,铅_PB=10000,铁_ZN=2000"
```

#### Docker Compose 中使用

在 `docker-compose.yml` 中修改：

```yaml
simulator:
  environment:
    SIM_PROFILE_OVERRIDES: "铜_PB=600,汞_HG=500,银_PB=5000"
    SIM_VERBOSE: "true"
    SIM_REPORT_INTERVAL: "60"
```

#### 单独运行模拟器

```bash
# 只跑模拟器，连接已有的后端
docker compose run --rm simulator \
  -e API_BASE=http://your-backend:8080 \
  -e SIM_NUM_SITES=10 \
  -e SIM_CONTINUOUS=false
```

### 内置 30 个遗址清单

| ID | 名称 | 金属类型 | 规模 | 国家 |
|----|------|---------|------|------|
| 1-9 | 蒂尔曼、提姆纳河谷、法尤姆、萨尔茨堡、法伦、基律纳、大冶铜绿山、瑞昌铜岭、中条山 | 铜 | 超大型/大型/中型 | 约旦、以色列、埃及、奥地利、瑞典、中国 |
| 10-18 | 科里亚、梅罗伊、德尔菲、鲁尔、铁桥谷、赫兰、徐州利国驿、巩义铁生沟、南阳宛城 | 铁 | 超大型/大型/中型 | 尼日利亚、苏丹、希腊、德国、英国、中国 |
| 19-23 | 拉乌里科查、萨卡特卡斯、波托西、弗莱贝格、库特纳霍拉 | 银 | 超大型/大型 | 秘鲁、墨西哥、玻利维亚、德国、捷克 |
| 24-26 | 萨德伯里、门迪普、洛林 | 铅 | 大型/中型 | 英国、法国 |
| 27-30 | 阿尔马登、伊德里亚、新阿尔马登、万山汞矿 | 汞 | 超大型/大型 | 西班牙、斯洛文尼亚、美国、中国 |

---

## 📊 监控系统

### Prometheus 指标

Go 后端暴露 14 个业务 + HTTP 指标到 `:2112/metrics`：

#### HTTP 指标
| 指标名 | 类型 | 说明 |
|--------|------|------|
| `http_requests_total` | Counter | HTTP 请求总数（按 method/path/status） |
| `http_request_duration_seconds` | Histogram | HTTP 请求耗时分布 |
| `http_requests_in_flight` | Gauge | 当前正在处理的请求数 |

#### 业务指标
| 指标名 | 类型 | 说明 |
|--------|------|------|
| `xrf_measurements_received_total` | Counter | XRF 数据接收总数 |
| `xrf_measurements_failed_total` | Counter | XRF 数据失败总数 |
| `alerts_generated_total` | Counter | 告警生成数（按 severity 区分） |
| `alerts_emails_sent_total` | Counter | 告警邮件发送总数 |
| `fingerprint_matches_total` | Counter | 指纹匹配次数 |
| `remediation_assessments_total` | Counter | 修复评估次数 |
| `pca_computations_total` | Counter | PCA 计算次数 |
| `db_query_duration_seconds` | Histogram | 数据库查询耗时 |

#### EventBus 指标
| 指标名 | 类型 | 说明 |
|--------|------|------|
| `eventbus_published_total` | Counter | EventBus 发布事件数 |
| `eventbus_dropped_total` | Counter | EventBus 丢弃事件数（队列满） |

### Prometheus 查询示例

打开 Prometheus UI: http://localhost:9090

```promql
// HTTP 请求速率（5分钟平均）
rate(http_requests_total[5m])

// XRF 接收总数
sum(rate(xrf_measurements_received_total[5m]))

// 按严重程度统计告警
sum by (severity) (rate(alerts_generated_total[1h]))

// 最慢的 API 端点（p95 延迟）
histogram_quantile(0.95, sum by (le, path) (rate(http_request_duration_seconds_bucket[5m])))

// EventBus 丢弃事件（通道满了需要调大 buffer）
rate(eventbus_dropped_total[5m])
```

### pprof 性能剖析

访问 pprof UI: http://localhost:6060/debug/pprof/

#### 常用命令

```bash
# 查看 heap profile (内存分配)
go tool pprof http://localhost:6060/debug/pprof/heap

# 查看 30 秒 CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 查看 goroutine 阻塞
go tool pprof http://localhost:6060/debug/pprof/block

# 查看 goroutine 数量
curl http://localhost:6060/debug/pprof/goroutine?debug=1
```

---

## 🗄️ 数据库索引优化

`init.sql` 中配置了多级索引，针对不同查询场景：

| 索引类型 | 适用场景 | 表/字段 |
|---------|---------|---------|
| **GiST 空间索引** | 空间查询（ST_Contains、ST_DWithin 等） | `sites.geom` |
| **BRIN 索引** | 时间序列数据（按插入顺序物理存储，体积比 B-Tree 小 100x） | `xrf_measurements.measurement_year`、`created_at` |
| **复合索引** | 覆盖高频查询（无需回表） | `xrf_measurements(site_id, measurement_year DESC)` |
| **Partial 索引** | 只索引活跃数据，大幅减小索引体积 | `alerts(site_id) WHERE is_sent = FALSE` |
| **GIN 索引** | 数组和 JSONB 查询 | `remediation_technologies.applicable_metals` |
| **物化视图** | 高频概览查询（最新污染指数） | `mv_site_latest_pollution` |

**索引使用检查**：

```sql
-- 查看索引使用情况
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
WHERE schemaname = 'public'
ORDER BY idx_scan DESC;

-- 查看未使用的索引
SELECT
    schemaname || '.' || tablename AS table_name,
    indexname
FROM pg_stat_user_indexes
WHERE idx_scan = 0
  AND schemaname = 'public';
```

---

## 🌐 API 接口列表

### 系统
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/healthz` | 健康检查 |
| GET | `/api/stats` | 系统概览统计 |

### 遗址管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/sites` | 获取所有遗址列表（含最新污染指数） |
| GET | `/api/sites/:id` | 获取单个遗址详情 |
| GET | `/api/sites/:id/trend` | 获取遗址近 10 年浓度趋势数据 |

### XRF 数据
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/sites/:id/xrf` | 上传 XRF 检测数据（自动触发所有下游模块） |

### 污染指纹
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/fingerprint/pca` | 对所有遗址执行 PCA 降维 + 聚类 + 质量评估 |
| GET | `/api/pca` | 同上（别名） |
| GET | `/api/sites/:id/fingerprint` | 对指定遗址进行指纹匹配识别 |

### 修复评估
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/remediation/technologies` | 获取修复技术数据库 |
| GET | `/api/technologies` | 同上（别名） |
| GET | `/api/sites/:id/remediation` | 对指定遗址进行修复方案评估推荐 |

### 告警
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/standards` | 获取风险管制标准 |
| GET | `/api/alerts?limit=50` | 获取告警记录 |
| POST | `/api/sites/:id/check-alerts` | 主动检查告警 |
| POST | `/api/alerts/flush` | 手动刷新告警聚合邮件 |

---

## 🎛️ 端口清单

| 端口 | 服务 | 说明 |
|------|------|------|
| **80** | Nginx | 前端 + API 统一入口（Gzip） |
| **5432** | PostgreSQL | 数据库 |
| **8080** | Go 后端 | API（直接访问，绕过 Nginx） |
| **6060** | Go pprof | 性能剖析 |
| **2112** | Go Prometheus | 指标抓取 |
| **9090** | Prometheus UI | 监控面板 |
| **8081** | Nginx 内部 | Nginx stub_status |

---

## ⚙️ 算法参数外置

所有硬编码参数集中在 [backend/config/params.go](backend/config/params.go)，修改配置无需改业务代码：

| 配置结构体 | 说明 |
|-----------|------|
| `PollutionStandards` | GB36600-2018 管制值（Pb/Zn/Cu/As/Hg/Cd） |
| `PCAConfig` | PCA + 聚类全参数（NumComponents、NumBootstraps、ElbowWeight 等） |
| `FingerprintConfig` | 指纹权重配置（同位素权重 2.5、比率权重 2.0 等） |
| `AHPConfig` | 判断矩阵 + RI 表（7x7 Saaty 矩阵，CR < 0.1） |
| `AlphaConfig` | 组合权重动态 α（Min=0.3, Max=0.7） |
| `ScoreBenchmarkConfig` | TOPSIS 打分基准（成本 15000 元/m³、周期 60 月等） |
| `EcoRiskConfig` | Hakanson 生态风险（毒性系数 Hg=40、Cd=30 等） |
| `AlertConfig` | 告警参数（30 分钟聚合、1.5 倍升级） |
| `EventBusChannelBufferSize` | EventBus 通道缓冲（默认 100） |

---

## 🛠️ 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.21, Gin Web Framework |
| 模块化 | EventBus (Go Channel), sync.Mutex, time.AfterFunc |
| 科学计算 | gonum（矩阵运算、PCA、统计分析） |
| 算法 | K-Means++、Bootstrap、轮廓系数、Gap Statistic、AHP、熵权法、TOPSIS |
| 数据库 | PostgreSQL 16 + PostGIS 3.4（GiST/BRIN/GIN 索引） |
| 邮件 | gomail.v2（时间窗口聚合推送） |
| 前端 | Leaflet 1.9、Canvas API、原生 JS（IIFE 模块化） |
| 监控 | pprof、Prometheus 2.52 |
| 反向代理 | Nginx 1.27（Gzip、缓存、安全头） |
| 部署 | Docker, Docker Compose v3.8 |

---

## 📝 常见问题

### Q: Docker 构建失败，提示找不到 frontend 目录？

**A**: Dockerfile 使用项目根目录作为构建上下文，请确保从根目录执行 `docker compose build`。

### Q: 模拟器无法连接后端？

**A**: 检查 `SIM_API_BASE` 环境变量，Docker 内部网络使用 `http://backend:8080`。

### Q: 邮件发送失败？

**A**: 在 `.env` 中正确配置 SMTP 信息。如果没有 SMTP 服务器，告警仍会记录在数据库中，只是不发送邮件。

### Q: 如何清理数据库重新开始？

**A**: `docker compose down -v`（删除数据卷），然后 `docker compose up -d`。

### Q: 如何调整 EventBus 缓冲大小？

**A**: 修改 [backend/config/params.go](backend/config/params.go) 中的 `EventBusChannelBufferSize`。如果 `eventbus_dropped_total` 指标持续增长，说明缓冲不够。

### Q: 前端访问慢？

**A**: Nginx 已开启 Gzip 压缩和静态资源缓存。检查浏览器开发者工具，确认响应头包含 `Content-Encoding: gzip` 和 `Cache-Control`。

---

## 📄 许可证

MIT License
