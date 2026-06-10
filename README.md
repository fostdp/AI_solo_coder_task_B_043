# 古代金属冶炼遗址污染指纹识别与现代环境修复系统

一套面向环境考古研究的全栈应用系统，集成古代金属冶炼遗址的重金属污染监测、污染指纹识别与环境修复评估功能。

## 功能特性

### 🌍 全球遗址分布可视化
- 基于 Leaflet + Canvas 绘制全球30个古代冶炼遗址分布图
- 遗址标记颜色根据污染程度（PI值）动态变化（绿→黄→橙→红）
- 标记大小根据冶炼规模分级（小型/中型/大型/超大型）
- 支持按金属类型（铜、铁、银、铅、汞）筛选显示

### 📈 重金属浓度趋势分析
- 展示遗址近10年Pb、Zn、Cu、As、Hg、Cd六种重金属浓度变化
- Canvas 绘制多参数趋势曲线
- 实时计算综合污染指数（PI）

### 🔬 污染指纹识别模型
- 基于重金属比率特征（Pb/Zn、Cu/Pb、As/Hg、Cd/Zn、Cu/As）
- 结合铅同位素比值（Pb206/Pb207、Pb208/Pb207）模拟数据
- **PCA主成分分析** + **K-Means聚类分析**建立特征库
- 内置15种典型冶炼工艺污染指纹（青铜冶炼、块炼铁、灰吹法炼银、汞齐法、辰砂炼汞等）
- 自动识别污染来源，计算相似度与欧氏距离

### 🏗️ 环境修复多属性决策系统
- 10种主流修复技术数据库（植物修复、固化稳定化、土壤淋洗、电动修复、热脱附、生物修复等）
- 基于重金属类型、土壤类型、污染程度、形态分布多维度评分
- 综合考虑：修复效率、经济性、周期、环境影响、可持续性
- 输出Top5推荐修复技术及分项评分

### ⚡ 污染预警与邮件推送
- 基于GB36600-2018风险管制标准自动检测超标
- 分级告警：超标预警（中）、修复预警（高）、重度污染（严重）、生态风险
- SMTP邮件自动推送告警通知

## 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        前端 (Frontend)                           │
│  Leaflet地图 + Canvas标记  │  趋势图表  │  指纹分析  │ 修复评估   │
└─────────────────────────────────────────────────────────────────┘
                                    │ REST API
                                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Go 后端 (Backend)                         │
│  Gin HTTP  │  PCA+聚类服务  │  多属性决策  │  邮件告警服务       │
└─────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                   PostgreSQL + PostGIS                           │
│  遗址表  │  XRF检测表  │  指纹库  │  修复技术库  │  告警表       │
└─────────────────────────────────────────────────────────────────┘
                                    ▲
                                    │ 定期上报
┌─────────────────────────────────────────────────────────────────┐
│                    XRF 数据模拟器 (Simulator)                    │
│     模拟30个遗址每年现场XRF检测数据，自动触发告警                 │
└─────────────────────────────────────────────────────────────────┘
```

## 项目结构

```
.
├── backend/                 # Go 后端
│   ├── config/             # 配置加载
│   ├── database/           # 数据库连接
│   ├── handlers/           # HTTP处理器
│   ├── models/             # 数据模型
│   ├── repository/         # 数据访问层
│   ├── services/           # 业务服务（PCA/聚类/多属性决策/告警）
│   ├── .env                # 环境配置
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
├── frontend/               # 前端页面
│   ├── index.html
│   ├── styles.css
│   └── app.js
├── simulator/              # XRF 数据模拟器
│   ├── main.go
│   ├── Dockerfile
│   └── go.mod
├── database/               # 数据库初始化
│   └── init.sql           # 完整DDL + 初始数据
└── docker-compose.yml      # Docker一键部署
```

## 快速开始

### 方式一：Docker Compose（推荐）

```bash
# 一键启动
docker-compose up -d

# 查看服务状态
docker-compose ps

# 访问前端
# 打开浏览器访问 http://localhost:8080
```

### 方式二：手动部署

#### 1. 准备数据库
```bash
# 确保已安装 PostgreSQL + PostGIS
createdb archaeology_pollution
psql -d archaeology_pollution -f database/init.sql
```

#### 2. 启动后端
```bash
cd backend
# 修改 .env 配置数据库连接
go mod download
go run main.go
```

#### 3. 启动模拟器（可选）
```bash
cd simulator
go mod download
go run main.go
```

#### 4. 访问前端
打开浏览器访问 `http://localhost:8080`

## API 接口列表

### 统计信息
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/stats` | 获取系统概览统计数据 |

### 遗址管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/sites` | 获取所有遗址列表（含最新污染指数） |
| GET | `/api/sites/:id` | 获取单个遗址详情 |
| GET | `/api/sites/:id/trend` | 获取遗址近10年浓度趋势数据 |

### XRF数据
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/xrf` | 上传XRF检测数据（自动触发告警检测） |

### 污染指纹
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/fingerprints` | 获取污染指纹特征库 |
| GET | `/api/sites/:id/fingerprint` | 对指定遗址进行指纹匹配识别 |
| GET | `/api/pca` | 对所有遗址执行PCA降维+聚类分析 |

### 修复评估
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/technologies` | 获取修复技术数据库 |
| GET | `/api/sites/:id/remediation` | 对指定遗址进行修复方案评估推荐 |

### 告警管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/standards` | 获取风险管制标准 |
| GET | `/api/alerts?limit=50` | 获取告警记录 |

## 核心算法说明

### 污染指纹识别
1. **特征构建**：计算Pb/Zn、Cu/Pb、As/Hg、Cd/Zn、Cu/As五组特征比率
2. **PCA降维**：使用gonum库对6种重金属浓度对数进行主成分分析，提取前3个主成分
3. **K-Means聚类**：基于PCA结果进行K=8的聚类，区分不同冶炼工艺类别
4. **指纹匹配**：结合重金属比率、同位素比值加权欧氏距离，取最近邻指纹

### 修复技术多属性决策
采用加权评分法（TOPSIS简化版），权重分配：
- 重金属覆盖度：30%
- 修复效率：20%
- 经济性：15-25%（污染越严重权重越低）
- 修复周期：15-35%（污染越严重权重越高）
- 土壤适应性：10%
- 环境影响与可持续性：剩余权重

## 内置数据

### 30个全球古代冶炼遗址
- 铜冶炼（9个）：蒂尔曼、提姆纳河谷、大冶铜绿山、法伦铜矿等
- 铁冶炼（8个）：梅罗伊、鲁尔工业区、徐州利国驿、铁桥谷等
- 银冶炼（5个）：波托西、拉乌里科查、萨卡特卡斯、库特纳霍拉等
- 铅冶炼（3个）：门迪普山区、萨德伯里、洛林矿区
- 汞冶炼（4个）：阿尔马登、伊德里亚、新阿尔马登、万山汞矿

### 15种污染指纹特征
涵盖青铜冶炼（典型I型、含砷型、铅青铜型）、块炼铁、高炉炼铁、灰吹法炼银、汞齐法炼银、罗马铅冶炼、辰砂炼汞、混合冶炼等类别

### 10种主流修复技术
植物萃取修复、植物稳定修复、水泥基固化稳定化、螯合剂固化稳定化、化学淋洗、电动修复、热脱附、微生物修复、化学氧化还原、客土换土法

## 配置说明

### 后端配置 (backend/.env)

```env
# 数据库连接
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=archaeology_pollution
DB_SSLMODE=disable

# 服务端口
SERVER_PORT=8080

# SMTP邮件告警
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=alert@example.com
SMTP_PASSWORD=yourpassword
SMTP_FROM=alert@example.com
ALERT_RECIPIENTS=admin@example.com,archaeologist@example.com
```

## v1.1 优化更新说明

### 问题定位与改动总览

| 问题 | 定位文件 | 改动规模 |
|------|---------|---------|
| PCA+聚类过拟合 | `backend/services/fingerprint.go` | 新增 ~600 行 |
| 多属性决策权重不合理 | `backend/services/mcdm.go`（新增） | 新增 ~500 行 |
| 前端地图性能下降 | `frontend/app.js` | 修改 ~200 行 |
| 邮件推送频率过高 | `backend/services/alert.go` | 新增 ~400 行 |

---

### 1. PCA + 聚类分析过拟合修复

**问题**：遗址数量较少（30个）时，简单K-Means聚类容易过拟合，聚类结果不稳定。

**改动位置**：`backend/services/fingerprint.go`

**解决方案**：

| 技术 | 说明 | 关键函数 |
|------|------|---------|
| **Bootstrap重采样** | 100次有放回抽样，用Jaccard相似度评估聚类稳定性 | `bootstrapStability()` |
| **轮廓系数** | 衡量聚类内聚度和分离度，取值[-1,1]，越接近1越好 | `calculateSilhouette()` |
| **Gap Statistic** | 比较实际数据与随机参考分布的聚类效果，自动选K | `gapStatistic()` |
| **K-Means++初始化** | 概率选择初始质心，避免局部最优 | `kmeansPlusPlusInit()` |
| **最优K值综合选择** | 手肘法40% + 轮廓系数30% + Gap Statistic30% | `findOptimalK()` |
| **PCA质量评估** | 解释方差比、累积方差、KMO检验 | `PerformPCAWithQuality()` |

**新增结构体**：`PCAResultWithQuality`，包含解释方差、累积方差、聚类数量、轮廓系数、Bootstrap稳定性、Gap值等质量指标。

---

### 2. 多属性决策权重优化

**问题**：修复成本数据缺失时固定权重分配不合理，缺乏专家经验融入。

**改动位置**：
- 新增文件：`backend/services/mcdm.go`（完整MCDM服务）
- 修改文件：`backend/services/fingerprint.go`（`RemediationService`内嵌MCDM服务）

**解决方案**：

| 模块 | 说明 | 关键函数 |
|------|------|---------|
| **AHP层次分析法** | 7x7专家判断矩阵（Saaty 1-9标度），几何平均法计算权重，一致性检验CR<0.1 | `calculateAHPWeights()`, `GetConsistencyRatio()` |
| **熵权法** | 基于数据离散度计算客观权重，数据越分散权重越大 | `CalculateEntropyWeights()` |
| **组合权重** | alpha*AHP + (1-alpha)*熵权法，线性加权融合 | `CalculateCombinedWeights()` |
| **动态alpha** | 污染越重，专家权重占比越高（0.3~0.7） | `GetDynamicAlpha()` |
| **数据缺失自适应** | 自动检测缺失属性，将缺失权重按比例重新分配 | `AdjustWeightsForMissingData()` |
| **TOPSIS评分** | 正负理想解 + 相对接近度，计算最终排名 | `ScoreTechnologies()` |

**AHP判断矩阵（7个评估维度）**：
重金属覆盖度 > 修复效率 > 土壤适应性 > 修复周期 > 经济性 > 环境影响 > 可持续性

**数据缺失处理**：成本或周期数据缺失时，缺失属性的权重按其他属性权重占比重新分配，确保权重和为1。

---

### 3. 前端地图聚合显示优化

**问题**：渲染大量遗址时Canvas绘制性能下降，标记重叠难以辨识。

**改动位置**：`frontend/app.js`

**解决方案**：

| 功能 | 说明 | 关键函数 |
|------|------|---------|
| **网格聚合算法** | 按屏幕坐标网格化分组，低缩放聚合、高缩放分散 | `buildClusters()` |
| **动态聚合阈值** | zoom≤5时启用聚合，网格大小随缩放动态调整（120px→60px） | `shouldUseClustering()`, `getClusterGridSize()` |
| **聚合点样式** | 颜色取区域内最严重污染值，大小随遗址数量递增（18~42px） | `drawCluster()`, `getClusterRadius()` |
| **聚合点交互** | 点击聚合点缩放至该区域，单遗址聚合直接显示详情 | `handleCanvasClick()` |
| **视口裁剪优化** | 只绘制视口范围内的遗址和聚合点 | `buildClusters()` 内判断 |

**聚合分级**：
- 2个及以下：半径18px
- 3-5个：半径24px
- 6-10个：半径30px
- 11-20个：半径36px
- 20个以上：半径42px

---

### 4. 邮件告警聚合推送优化

**问题**：多遗址同时告警时单条发送频率高，邮箱易被轰炸。

**改动位置**：`backend/services/alert.go`

**解决方案**：

| 功能 | 说明 | 关键函数/结构体 |
|------|------|---------------|
| **告警分级发送** | "严重"级别立即单条发送，"高/中/低"级别聚合批量发送 | `AlertAggregator`, `AddAlerts()` |
| **时间窗口聚合** | 30分钟时间窗口，到达后自动批量发送 | `AlertAggregator.flushPeriod`, `time.AfterFunc` |
| **批量汇总邮件** | 单封邮件包含所有待发送告警的摘要和详情 | `SendAggregatedDigest()` |
| **按遗址分组** | 同一遗址的多条告警合并展示，便于查看 | `groupAlertsBySite()` |
| **严重程度统计** | 汇总邮件顶部展示告警总数、涉及遗址数、各级别数量 | `countBySeverity()`, `buildDigestHTML()` |
| **可视化严重度条** | HTML邮件中的彩色严重程度分布条 | `buildDigestHTML()` |

**告警聚合流程**：
```
告警产生 → AlertAggregator.AddAlerts()
    ├─ 严重级别 → 立即单条发送（带[紧急]标识）
    └─ 非严重级别 → 加入待发送队列
                      ↓
              30分钟时间窗口到达
                      ↓
              SendAggregatedDigest()
                      ↓
              单封汇总邮件（按遗址分组）
```

**邮件类型**：
- 单条严重告警：主题带`[紧急]`前缀，红色高亮，强调立即处理
- 批量汇总邮件：主题带`[汇总]`前缀，包含统计概览+按遗址分组的详细列表

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.21, Gin Web Framework |
| 数据库 | PostgreSQL 16 + PostGIS 3.4 |
| 科学计算 | gonum（矩阵运算、PCA、统计分析） |
| 邮件 | gomail.v2 |
| 前端 | Leaflet 1.9, Canvas API, 原生JS |
| 部署 | Docker, Docker Compose |
| 统计算法 | Bootstrap、轮廓系数、Gap Statistic、AHP、熵权法、TOPSIS |

## 许可证

MIT License
