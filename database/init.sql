-- ========================================
-- 古代金属冶炼遗址污染指纹识别与环境修复系统
-- PostgreSQL + PostGIS 数据库初始化脚本
-- ========================================

-- 启用扩展
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS postgis_topology;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ========================================
-- 遗址表
-- ========================================
CREATE TABLE IF NOT EXISTS sites (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    country VARCHAR(100) NOT NULL,
    metal_type VARCHAR(20) NOT NULL CHECK (metal_type IN ('铜', '铁', '银', '铅', '汞')),
    scale VARCHAR(20) NOT NULL CHECK (scale IN ('小型', '中型', '大型', '超大型')),
    era VARCHAR(100),
    description TEXT,
    geom GEOMETRY(Point, 4326) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sites_geom ON sites USING GIST(geom);
CREATE INDEX IF NOT EXISTS idx_sites_metal_type ON sites(metal_type);
CREATE INDEX IF NOT EXISTS idx_sites_scale ON sites(scale);

-- ========================================
-- XRF检测数据表（每年上报一次）
-- ========================================
CREATE TABLE IF NOT EXISTS xrf_measurements (
    id SERIAL PRIMARY KEY,
    site_id INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    sample_depth VARCHAR(50),
    measurement_year INTEGER NOT NULL,
    pb NUMERIC(12, 4),
    zn NUMERIC(12, 4),
    cu NUMERIC(12, 4),
    as_val NUMERIC(12, 4),
    hg NUMERIC(12, 4),
    cd NUMERIC(12, 4),
    ph NUMERIC(5, 2),
    organic_matter NUMERIC(8, 4),
    cation_exchange_capacity NUMERIC(10, 4),
    soil_type VARCHAR(100),
    remark TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, measurement_year, sample_depth)
);

CREATE INDEX IF NOT EXISTS idx_xrf_site_id ON xrf_measurements(site_id);
CREATE INDEX IF NOT EXISTS idx_xrf_year ON xrf_measurements(measurement_year);

-- ========================================
-- 重金属形态分析表
-- ========================================
CREATE TABLE IF NOT EXISTS metal_speciation (
    id SERIAL PRIMARY KEY,
    site_id INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    measurement_year INTEGER NOT NULL,
    metal_type VARCHAR(2) NOT NULL CHECK (metal_type IN ('Pb', 'Zn', 'Cu', 'As', 'Hg', 'Cd')),
    exchangeable NUMERIC(8, 4),
    carbonate_bound NUMERIC(8, 4),
    fe_mn_oxide_bound NUMERIC(8, 4),
    organic_bound NUMERIC(8, 4),
    residual NUMERIC(8, 4),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, measurement_year, metal_type)
);

CREATE INDEX IF NOT EXISTS idx_speciation_site ON metal_speciation(site_id, measurement_year);

-- ========================================
-- 同位素比值表（用于污染指纹识别）
-- ========================================
CREATE TABLE IF NOT EXISTS isotope_ratios (
    id SERIAL PRIMARY KEY,
    site_id INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    measurement_year INTEGER NOT NULL,
    pb206_pb204 NUMERIC(10, 6),
    pb207_pb204 NUMERIC(10, 6),
    pb208_pb204 NUMERIC(10, 6),
    pb206_pb207 NUMERIC(10, 6),
    pb208_pb207 NUMERIC(10, 6),
    cu65_cu63 NUMERIC(10, 6),
    zn68_zn64 NUMERIC(10, 6),
    hg202_hg198 NUMERIC(10, 6),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, measurement_year)
);

CREATE INDEX IF NOT EXISTS idx_isotope_site ON isotope_ratios(site_id, measurement_year);

-- ========================================
-- 污染指纹库表
-- ========================================
CREATE TABLE IF NOT EXISTS pollution_fingerprints (
    id SERIAL PRIMARY KEY,
    fingerprint_name VARCHAR(255) NOT NULL,
    metal_type VARCHAR(20) NOT NULL CHECK (metal_type IN ('铜', '铁', '银', '铅', '汞', '混合')),
    process_type VARCHAR(100),
    pb_zn_ratio NUMERIC(10, 4),
    cu_pb_ratio NUMERIC(10, 4),
    as_hg_ratio NUMERIC(10, 4),
    cd_zn_ratio NUMERIC(10, 4),
    cu_as_ratio NUMERIC(10, 4),
    pb206_pb207 NUMERIC(10, 6),
    pb208_pb207 NUMERIC(10, 6),
    pca_pc1 NUMERIC(12, 6),
    pca_pc2 NUMERIC(12, 6),
    pca_pc3 NUMERIC(12, 6),
    cluster_id INTEGER,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_fingerprints_metal ON pollution_fingerprints(metal_type);
CREATE INDEX IF NOT EXISTS idx_fingerprints_cluster ON pollution_fingerprints(cluster_id);

-- ========================================
-- 修复技术数据库
-- ========================================
CREATE TABLE IF NOT EXISTS remediation_technologies (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(50) NOT NULL CHECK (category IN ('植物修复', '固化稳定化', '土壤淋洗', '电动修复', '热脱附', '生物修复', '化学氧化还原', '客土法')),
    applicable_metals TEXT[] NOT NULL,
    applicable_soil_types TEXT[],
    cost_low NUMERIC(12, 2),
    cost_high NUMERIC(12, 2),
    duration_months_low INTEGER,
    duration_months_high INTEGER,
    remediation_efficiency NUMERIC(5, 2),
    environmental_impact_score NUMERIC(3, 1),
    applicability_score NUMERIC(3, 1),
    sustainability_score NUMERIC(3, 1),
    description TEXT,
    advantages TEXT,
    limitations TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ========================================
-- 修复评估表
-- ========================================
CREATE TABLE IF NOT EXISTS remediation_assessments (
    id SERIAL PRIMARY KEY,
    site_id INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    assessment_date DATE NOT NULL,
    pollution_index NUMERIC(8, 4),
    eco_risk_index NUMERIC(10, 4),
    recommended_technology_id INTEGER REFERENCES remediation_technologies(id),
    score NUMERIC(5, 2),
    assessment_details JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_assessment_site ON remediation_assessments(site_id);

-- ========================================
-- 风险管制标准表
-- ========================================
CREATE TABLE IF NOT EXISTS risk_standards (
    id SERIAL PRIMARY KEY,
    standard_name VARCHAR(255) NOT NULL,
    metal_type VARCHAR(2) NOT NULL CHECK (metal_type IN ('Pb', 'Zn', 'Cu', 'As', 'Hg', 'Cd')),
    screening_value NUMERIC(10, 4),
    intervention_value NUMERIC(10, 4),
    unit VARCHAR(20) DEFAULT 'mg/kg',
    land_use_type VARCHAR(50) DEFAULT '工业用地',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ========================================
-- 告警记录表
-- ========================================
CREATE TABLE IF NOT EXISTS alerts (
    id SERIAL PRIMARY KEY,
    site_id INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    measurement_id INTEGER REFERENCES xrf_measurements(id) ON DELETE SET NULL,
    alert_type VARCHAR(50) NOT NULL CHECK (alert_type IN ('超标预警', '重度污染', '生态风险', '修复预警')),
    metal_type VARCHAR(2),
    concentration NUMERIC(12, 4),
    threshold NUMERIC(10, 4),
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('低', '中', '高', '严重')),
    is_sent BOOLEAN DEFAULT FALSE,
    email_recipients TEXT[],
    message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_alerts_site ON alerts(site_id);
CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(severity);
CREATE INDEX IF NOT EXISTS idx_alerts_sent ON alerts(is_sent);

-- ========================================
-- 工程化增强：高级索引
-- ========================================

-- BRIN索引：适合时间序列数据（按插入顺序物理存储），比B-Tree小100x
CREATE INDEX IF NOT EXISTS idx_xrf_year_brin ON xrf_measurements USING BRIN(measurement_year);
CREATE INDEX IF NOT EXISTS idx_xrf_created_brin ON xrf_measurements USING BRIN(created_at);
CREATE INDEX IF NOT EXISTS idx_isotope_year_brin ON isotope_ratios USING BRIN(measurement_year);
CREATE INDEX IF NOT EXISTS idx_speciation_year_brin ON metal_speciation USING BRIN(measurement_year);

-- 复合索引：覆盖高频查询
CREATE INDEX IF NOT EXISTS idx_xrf_site_year ON xrf_measurements(site_id, measurement_year DESC);
CREATE INDEX IF NOT EXISTS idx_xrf_site_created ON xrf_measurements(site_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_isotope_site_year ON isotope_ratios(site_id, measurement_year DESC);
CREATE INDEX IF NOT EXISTS idx_speciation_site_year_metal ON metal_speciation(site_id, measurement_year DESC, metal_type);

-- Partial索引：仅索引活跃数据，大幅减小索引体积
CREATE INDEX IF NOT EXISTS idx_alerts_pending ON alerts(site_id) WHERE is_sent = FALSE;
CREATE INDEX IF NOT EXISTS idx_alerts_high_severity ON alerts(severity, created_at DESC) WHERE severity IN ('高', '严重');

-- GIN索引：加速JSONB和数组查询
CREATE INDEX IF NOT EXISTS idx_remediation_metals_gin ON remediation_technologies USING GIN(applicable_metals);
CREATE INDEX IF NOT EXISTS idx_assessment_details_gin ON remediation_assessments USING GIN(assessment_details);

-- ========================================
-- 工程化增强：物化视图（用于高频概览查询）
-- ========================================

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_site_latest_pollution AS
SELECT
    m.site_id,
    s.name AS site_name,
    s.metal_type,
    s.scale,
    s.country,
    m.measurement_year,
    m.pb, m.zn, m.cu, m.as_val, m.hg, m.cd,
    ROUND(
        (CASE WHEN m.pb > 0 THEN (m.pb / 800.0) ELSE 0 END +
         CASE WHEN m.zn > 0 THEN (m.zn / 5000.0) ELSE 0 END +
         CASE WHEN m.cu > 0 THEN (m.cu / 1800.0) ELSE 0 END +
         CASE WHEN m.as_val > 0 THEN (m.as_val / 250.0) ELSE 0 END +
         CASE WHEN m.hg > 0 THEN (m.hg / 38.0) ELSE 0 END +
         CASE WHEN m.cd > 0 THEN (m.cd / 47.0) ELSE 0 END) / 6.0,
    4) AS pollution_index,
    ST_AsGeoJSON(s.geom) AS geom_json,
    ST_X(s.geom) AS lng,
    ST_Y(s.geom) AS lat
FROM xrf_measurements m
JOIN sites s ON m.site_id = s.id
WHERE m.measurement_year = (SELECT MAX(measurement_year) FROM xrf_measurements m2 WHERE m2.site_id = m.site_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_pollution_site_id ON mv_site_latest_pollution(site_id);
CREATE INDEX IF NOT EXISTS idx_mv_pollution_pi ON mv_site_latest_pollution(pollution_index DESC);
CREATE INDEX IF NOT EXISTS idx_mv_pollution_metal ON mv_site_latest_pollution(metal_type);
CREATE INDEX IF NOT EXISTS idx_mv_pollution_scale ON mv_site_latest_pollution(scale);

-- 统计信息收集
ANALYZE sites;
ANALYZE xrf_measurements;
ANALYZE pollution_fingerprints;
ANALYZE remediation_technologies;

-- ========================================
-- 污染指数计算视图（保留向后兼容）
-- ========================================
CREATE OR REPLACE VIEW v_pollution_index AS
SELECT
    m.site_id,
    s.name AS site_name,
    s.metal_type,
    s.scale,
    m.measurement_year,
    m.pb, m.zn, m.cu, m.as_val, m.hg, m.cd,
    ROUND(
        (CASE WHEN m.pb > 0 THEN (m.pb / 800.0) ELSE 0 END +
         CASE WHEN m.zn > 0 THEN (m.zn / 5000.0) ELSE 0 END +
         CASE WHEN m.cu > 0 THEN (m.cu / 1800.0) ELSE 0 END +
         CASE WHEN m.as_val > 0 THEN (m.as_val / 250.0) ELSE 0 END +
         CASE WHEN m.hg > 0 THEN (m.hg / 38.0) ELSE 0 END +
         CASE WHEN m.cd > 0 THEN (m.cd / 47.0) ELSE 0 END) / 6.0,
    4) AS pollution_index,
    ST_AsGeoJSON(s.geom) AS geom_json
FROM xrf_measurements m
JOIN sites s ON m.site_id = s.id
WHERE m.measurement_year = (SELECT MAX(measurement_year) FROM xrf_measurements m2 WHERE m2.site_id = m.site_id);

-- ========================================
-- 插入风险管制标准数据（GB36600-2018 第二类用地筛选值/管制值）
-- ========================================
INSERT INTO risk_standards (standard_name, metal_type, screening_value, intervention_value, unit, land_use_type) VALUES
('GB36600-2018', 'Pb', 800.0, 2500.0, 'mg/kg', '工业用地'),
('GB36600-2018', 'Zn', 5000.0, 10000.0, 'mg/kg', '工业用地'),
('GB36600-2018', 'Cu', 18000.0, 36000.0, 'mg/kg', '工业用地'),
('GB36600-2018', 'As', 250.0, 250.0, 'mg/kg', '工业用地'),
('GB36600-2018', 'Hg', 38.0, 82.0, 'mg/kg', '工业用地'),
('GB36600-2018', 'Cd', 47.0, 172.0, 'mg/kg', '工业用地')
ON CONFLICT DO NOTHING;

-- ========================================
-- 插入修复技术数据库
-- ========================================
INSERT INTO remediation_technologies (
    name, category, applicable_metals, applicable_soil_types,
    cost_low, cost_high, duration_months_low, duration_months_high,
    remediation_efficiency, environmental_impact_score, applicability_score, sustainability_score,
    description, advantages, limitations
) VALUES
(
    '超富集植物萃取修复',
    '植物修复',
    ARRAY['Pb', 'Zn', 'Cu', 'Cd'],
    ARRAY['壤土', '砂壤土', '粘土'],
    500.0, 2000.0, 12, 60,
    65.0, 9.0, 8.0, 9.5,
    '利用超富集植物（如印度芥菜、东南景天）吸收土壤中的重金属',
    '环境友好、成本低、不破坏土壤结构、可回收金属',
    '修复周期长、受气候影响大、对复合污染效果有限'
),
(
    '植物稳定修复',
    '植物修复',
    ARRAY['Pb', 'Zn', 'Cu', 'As', 'Cd', 'Hg'],
    ARRAY['壤土', '砂土', '粘土'],
    300.0, 1200.0, 6, 36,
    75.0, 9.5, 8.5, 9.0,
    '利用植物根系固定重金属，减少迁移扩散',
    '快速控制污染扩散、防止侵蚀、美化环境',
    '重金属仍留在土壤中，需要长期监测'
),
(
    '水泥基固化稳定化',
    '固化稳定化',
    ARRAY['Pb', 'Zn', 'Cu', 'As', 'Cd'],
    ARRAY['壤土', '砂土', '粘土', '粉土'],
    800.0, 2500.0, 1, 6,
    90.0, 7.0, 9.0, 7.5,
    '通过添加水泥、石灰等固化剂将重金属包裹固定',
    '处理周期短、技术成熟、处理费用较低、效果稳定',
    '增加土壤体积、破坏土壤结构、长期稳定性需监测'
),
(
    '螯合剂固化稳定化',
    '固化稳定化',
    ARRAY['Pb', 'Cu', 'Cd', 'Hg', 'Zn'],
    ARRAY['壤土', '砂土', '粘土'],
    1500.0, 4000.0, 1, 3,
    92.0, 7.5, 8.5, 7.0,
    '使用有机螯合剂与重金属形成稳定螯合物',
    '固化效果好、添加量少、适应范围广',
    '成本较高、部分螯合剂可能有环境风险'
),
(
    '化学淋洗修复',
    '土壤淋洗',
    ARRAY['Pb', 'Zn', 'Cu', 'Cd', 'As'],
    ARRAY['砂土', '砂壤土'],
    2000.0, 6000.0, 3, 12,
    85.0, 6.0, 7.0, 6.5,
    '使用淋洗剂（EDTA、柠檬酸等）提取土壤重金属',
    '去除效率高、可回收重金属、处理彻底',
    '可能破坏土壤结构、废水需处理、对粘壤土效果差'
),
(
    '电动修复',
    '电动修复',
    ARRAY['Pb', 'Zn', 'Cu', 'Cd', 'As', 'Hg'],
    ARRAY['粘土', '粉土'],
    3000.0, 8000.0, 2, 12,
    80.0, 7.0, 6.5, 7.0,
    '施加电场驱动重金属离子迁移富集',
    '适用于低渗透土壤、可原位修复、不破坏土壤',
    '能耗高、电极腐蚀、可能产生副反应'
),
(
    '热脱附修复',
    '热脱附',
    ARRAY['Hg'],
    ARRAY['壤土', '砂土', '粘土'],
    5000.0, 15000.0, 1, 6,
    95.0, 5.5, 6.0, 5.0,
    '通过加热使汞挥发并收集处理',
    '去除效率极高、处理速度快',
    '能耗高、仅适用于挥发性金属、可能破坏土壤有机质'
),
(
    '微生物修复',
    '生物修复',
    ARRAY['As', 'Hg', 'Pb', 'Cu', 'Zn', 'Cd'],
    ARRAY['壤土', '粉土'],
    800.0, 3000.0, 6, 24,
    70.0, 9.0, 7.0, 9.0,
    '利用微生物转化重金属形态或固定重金属',
    '环境友好、成本低、可与植物修复联用',
    '受环境条件影响大、修复周期长、效果波动大'
),
(
    '化学氧化还原',
    '化学氧化还原',
    ARRAY['As', 'Hg', 'Cr'],
    ARRAY['壤土', '砂土', '粘土'],
    1500.0, 5000.0, 1, 6,
    82.0, 6.5, 7.5, 6.0,
    '通过氧化剂或还原剂改变重金属价态降低毒性',
    '反应迅速、处理效率高、操作相对简单',
    '可能产生二次污染、药剂用量大、影响土壤性质'
),
(
    '客土/换土法',
    '客土法',
    ARRAY['Pb', 'Zn', 'Cu', 'As', 'Hg', 'Cd'],
    ARRAY['各种土壤'],
    3000.0, 10000.0, 1, 3,
    98.0, 4.0, 9.5, 3.5,
    '用清洁土壤替换污染土壤或覆盖清洁土壤',
    '效果立竿见影、彻底去除污染、适用范围广',
    '成本高、破坏原土壤生态、污染土壤需异地处置'
)
ON CONFLICT DO NOTHING;

-- ========================================
-- 插入30个全球古代金属冶炼遗址
-- ========================================
INSERT INTO sites (name, country, metal_type, scale, era, description, geom) VALUES
-- 铜冶炼遗址
('蒂尔曼遗址', '约旦', '铜', '超大型', '公元前4500-前1200年', '古代近东最重要的铜冶炼中心之一', ST_SetSRID(ST_MakePoint(35.45, 30.32), 4326)),
('提姆纳河谷', '以色列', '铜', '大型', '公元前5000-公元1世纪', '圣经时期著名的铜矿冶炼遗址', ST_SetSRID(ST_MakePoint(34.98, 29.73), 4326)),
('法尤姆遗址', '埃及', '铜', '中型', '公元前3000-前1000年', '古埃及铜器时代冶炼遗址', ST_SetSRID(ST_MakePoint(30.83, 29.31), 4326)),
('萨尔茨堡附近矿区', '奥地利', '铜', '大型', '公元前2500-前800年', '哈尔施塔特文化铜冶炼中心', ST_SetSRID(ST_MakePoint(13.05, 47.80), 4326)),
('法伦铜矿', '瑞典', '铜', '超大型', '公元9-20世纪', '中世纪欧洲最大的铜矿和冶炼中心', ST_SetSRID(ST_MakePoint(15.62, 60.60), 4326)),
('基律纳矿区', '瑞典', '铜', '大型', '公元17-20世纪', '北欧重要的铜铁冶炼遗址', ST_SetSRID(ST_MakePoint(20.22, 67.85), 4326)),
('大冶铜绿山', '中国', '铜', '超大型', '公元前1500-公元1900年', '中国古代规模最大的铜矿采冶遗址', ST_SetSRID(ST_MakePoint(114.93, 30.09), 4326)),
('瑞昌铜岭', '中国', '铜', '大型', '公元前1600-前1000年', '商代重要铜矿采冶遗址', ST_SetSRID(ST_MakePoint(115.62, 29.68), 4326)),
('中条山矿区', '中国', '铜', '大型', '公元前2000-前200年', '夏商周时期核心铜料来源', ST_SetSRID(ST_MakePoint(111.55, 35.41), 4326)),
-- 铁冶炼遗址
('科里亚遗址', '尼日利亚', '铁', '大型', '公元前500-公元1500年', '西非诺克文化铁冶炼中心', ST_SetSRID(ST_MakePoint(8.78, 9.83), 4326)),
('梅罗伊遗址', '苏丹', '铁', '超大型', '公元前300-公元400年', '古库什王国的铁冶炼中心，非洲古代钢铁之都', ST_SetSRID(ST_MakePoint(33.78, 16.93), 4326)),
('德尔菲遗址', '希腊', '铁', '中型', '公元前1000-公元200年', '古希腊铁冶炼遗址', ST_SetSRID(ST_MakePoint(22.50, 38.48), 4326)),
('鲁尔工业区遗址', '德国', '铁', '超大型', '公元12-20世纪', '欧洲工业革命时期最重要的钢铁基地', ST_SetSRID(ST_MakePoint(7.15, 51.43), 4326)),
('铁桥谷', '英国', '铁', '大型', '公元17-19世纪', '工业革命发源地，世界最早的铸铁桥梁所在地', ST_SetSRID(ST_MakePoint(-2.48, 52.62), 4326)),
('赫兰钢铁遗址', '德国', '铁', '大型', '公元16-19世纪', '中欧早期近代铁冶炼遗址', ST_SetSRID(ST_MakePoint(7.57, 51.26), 4326)),
('徐州利国驿', '中国', '铁', '超大型', '公元前500-公元1900年', '汉代以来最重要的官营铁冶基地', ST_SetSRID(ST_MakePoint(117.46, 34.37), 4326)),
('巩义铁生沟', '中国', '铁', '大型', '公元前200-公元200年', '汉代大型官营铁冶遗址', ST_SetSRID(ST_MakePoint(113.02, 34.70), 4326)),
('南阳宛城冶铁', '中国', '铁', '大型', '公元前300-公元200年', '战国汉代著名的铁冶手工业中心', ST_SetSRID(ST_MakePoint(112.55, 33.01), 4326)),
-- 银冶炼遗址
('拉乌里科查遗址', '秘鲁', '银', '超大型', '公元1000-1900年', '印加帝国及殖民时期世界最大的银矿产地', ST_SetSRID(ST_MakePoint(-76.58, -14.72), 4326)),
('萨卡特卡斯', '墨西哥', '银', '大型', '公元1546-20世纪', '西班牙殖民时期新世界最重要的银产地', ST_SetSRID(ST_MakePoint(-102.58, 22.77), 4326)),
('波托西', '玻利维亚', '银', '超大型', '公元1545-20世纪', '世界历史上产银最多的矿山，支撑了西班牙帝国的财政', ST_SetSRID(ST_MakePoint(-65.76, -19.59), 4326)),
('弗莱贝格', '德国', '银', '大型', '公元12-20世纪', '中世纪欧洲最重要的银矿冶炼中心', ST_SetSRID(ST_MakePoint(13.34, 50.92), 4326)),
('库特纳霍拉', '捷克', '银', '大型', '公元13-16世纪', '中世纪欧洲最富有的银矿城市，铸造著名的格罗申银币', ST_SetSRID(ST_MakePoint(15.26, 49.95), 4326)),
-- 铅冶炼遗址
('萨德伯里', '英国', '铅', '中型', '公元前100-公元1900年', '古罗马至近代英格兰铅冶炼中心', ST_SetSRID(ST_MakePoint(-1.98, 52.89), 4326)),
('门迪普山区', '英国', '铅', '大型', '公元前50-公元1900年', '古罗马时期不列颠岛主要铅矿产地', ST_SetSRID(ST_MakePoint(-2.75, 51.28), 4326)),
('洛林矿区', '法国', '铅', '大型', '公元前200-公元1900年', '西欧重要的铅锌矿冶炼区域', ST_SetSRID(ST_MakePoint(6.18, 49.11), 4326)),
-- 汞（辰砂）冶炼遗址
('阿尔马登', '西班牙', '汞', '超大型', '公元前400-20世纪', '世界最大的汞矿产地，古罗马至近代全球汞供应的主要来源', ST_SetSRID(ST_MakePoint(-4.84, 38.77), 4326)),
('伊德里亚', '斯洛文尼亚', '汞', '大型', '公元1490-20世纪', '欧洲第二大汞矿，与阿尔马登齐名', ST_SetSRID(ST_MakePoint(14.03, 46.00), 4326)),
('新阿尔马登', '美国', '汞', '大型', '公元1845-1976年', '北美最大的汞矿，加州淘金热时期重要汞供应地', ST_SetSRID(ST_MakePoint(-121.05, 37.13), 4326)),
('万山汞矿', '中国', '汞', '超大型', '公元1400-2000年', '"中国汞都"，世界罕见的超大型汞矿田', ST_SetSRID(ST_MakePoint(109.20, 27.53), 4326))
ON CONFLICT DO NOTHING;

-- ========================================
-- 插入初始污染指纹库（标准指纹特征）
-- ========================================
INSERT INTO pollution_fingerprints (
    fingerprint_name, metal_type, process_type,
    pb_zn_ratio, cu_pb_ratio, as_hg_ratio, cd_zn_ratio, cu_as_ratio,
    pb206_pb207, pb208_pb207,
    pca_pc1, pca_pc2, pca_pc3, cluster_id, description
) VALUES
('青铜冶炼-典型I型', '铜', '还原焙烧-还原熔炼', 0.45, 2.80, 0.12, 0.004, 18.5, 1.1780, 2.4560, 2.345, 1.234, 0.567, 1, '以Cu为主，伴随少量Pb、Zn、As，铜铅比高'),
('青铜冶炼-含砷型', '铜', '还原熔炼-砷合金', 0.35, 3.20, 0.58, 0.003, 5.2, 1.1720, 2.4480, 1.892, 2.156, 1.234, 1, '高As特征，与早期青铜冶炼加入砷矿有关'),
('青铜冶炼-铅青铜型', '铜', '铅青铜合金冶炼', 1.85, 0.65, 0.08, 0.006, 25.0, 1.1850, 2.4720, 1.567, -0.234, 0.890, 2, 'Pb含量显著升高，铜铅比低'),
('古代块炼铁', '铁', '固态还原法', 0.15, 0.25, 0.10, 0.002, 1.5, 1.1650, 2.4420, -1.234, 2.456, 0.345, 3, '以Fe为主，重金属杂质含量整体较低'),
('近代高炉炼铁', '铁', '高炉熔炼法', 0.30, 0.35, 0.25, 0.005, 1.2, 1.1800, 2.4600, -2.345, 1.567, -0.890, 3, '使用焦炭冶炼，重金属伴生元素增加'),
('古代银-铅冶炼', '银', '灰吹法', 8.50, 0.12, 0.25, 0.008, 0.8, 1.1920, 2.4850, 2.789, -1.234, 1.567, 4, '极高Pb/Zn比，Pb主导，含Ag相关痕量元素'),
('殖民时期银冶炼', '银', '汞齐法', 6.80, 0.18, 5.50, 0.006, 0.3, 1.1950, 2.4920, 3.123, -1.567, 2.890, 4, '高Hg伴生，As/Hg比特征明显'),
('古代银-铅锌矿', '银', '混汞法', 4.20, 0.25, 2.80, 0.012, 1.1, 1.1880, 2.4780, 2.456, -0.890, 1.890, 5, 'Zn含量较高，Pb/Zn比中等'),
('罗马时期铅冶炼', '铅', '还原熔炼-精炼', 12.50, 0.05, 0.08, 0.003, 0.5, 1.1680, 2.4450, 3.567, -2.345, 0.567, 6, '极高Pb含量，其他元素较低，铅同位素特征典型'),
('中世纪铅银冶炼', '铅', '铅银共生冶炼', 9.80, 0.10, 0.15, 0.005, 0.6, 1.1760, 2.4580, 3.234, -1.890, 0.890, 6, 'Pb高，伴生Ag相关元素'),
('古代辰砂炼汞', '汞', '焙烧-冷凝法', 0.20, 0.08, 15.0, 0.002, 0.2, 1.1700, 2.4460, -0.567, 3.234, 2.567, 7, '极高Hg特征，As/Hg比值低'),
('近代辰砂冶炼', '汞', '机械化焙烧', 0.28, 0.12, 12.5, 0.003, 0.3, 1.1740, 2.4520, -0.890, 2.890, 2.123, 7, 'Hg高，伴生痕量元素较古代丰富'),
('铜铁混合冶炼', '混合', '铜铁共生矿处理', 0.65, 1.45, 0.35, 0.005, 8.5, 1.1750, 2.4550, 0.567, 1.234, 0.234, 8, 'Cu和Fe特征并存'),
('铅锌混合冶炼', '混合', '铅锌矿处理', 3.50, 0.22, 0.45, 0.015, 1.2, 1.1820, 2.4650, 1.234, -0.567, 0.456, 8, 'Pb和Zn双高特征'),
('多金属复杂冶炼', '混合', '多金属矿综合冶炼', 1.80, 0.85, 1.20, 0.010, 2.5, 1.1800, 2.4600, 0.234, 0.567, 1.234, 8, '多种重金属浓度均较高，指纹复杂')
ON CONFLICT DO NOTHING;

-- ============================================
-- Feature v2.1: 冶炼工艺反演 & 农田安全 & 矿渣资源化 & 时间线对比
-- 新增4张核心表 + 索引 + 种子数据
-- ============================================

-- ====== 表1: slag_compositions 矿渣矿物组成 ======
CREATE TABLE IF NOT EXISTS slag_compositions (
    id SERIAL PRIMARY KEY,
    site_id INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    measurement_year INTEGER NOT NULL,
    sample_depth VARCHAR(50),
    -- 主要氧化物（重量百分比 %）
    sio2 NUMERIC(8,4) DEFAULT 0,      -- SiO2 二氧化硅
    al2o3 NUMERIC(8,4) DEFAULT 0,     -- Al2O3 氧化铝
    cao NUMERIC(8,4) DEFAULT 0,       -- CaO 氧化钙
    feo NUMERIC(8,4) DEFAULT 0,       -- FeO 氧化亚铁
    fe2o3 NUMERIC(8,4) DEFAULT 0,     -- Fe2O3 三氧化二铁
    mgo NUMERIC(8,4) DEFAULT 0,       -- MgO 氧化镁
    mno NUMERIC(8,4) DEFAULT 0,       -- MnO 氧化锰
    p2o5 NUMERIC(8,4) DEFAULT 0,      -- P2O5 五氧化二磷
    so3 NUMERIC(8,4) DEFAULT 0,       -- SO3 三氧化硫
    k2o NUMERIC(8,4) DEFAULT 0,       -- K2O 氧化钾
    na2o NUMERIC(8,4) DEFAULT 0,      -- Na2O 氧化钠
    tio2 NUMERIC(8,4) DEFAULT 0,      -- TiO2 二氧化钛
    -- 矿物相（%）
    fayalite NUMERIC(6,2) DEFAULT 0,   -- 铁橄榄石 2FeO·SiO2
    wollastonite NUMERIC(6,2) DEFAULT 0,-- 硅灰石 CaSiO3
    anorthite NUMERIC(6,2) DEFAULT 0,   -- 钙长石 CaAl2Si2O8
    diopside NUMERIC(6,2) DEFAULT 0,    -- 透辉石 CaMgSi2O6
    magnetite NUMERIC(6,2) DEFAULT 0,   -- 磁铁矿 Fe3O4
    hematite NUMERIC(6,2) DEFAULT 0,    -- 赤铁矿 Fe2O3
    wuestite NUMERIC(6,2) DEFAULT 0,    -- 方铁矿 FeO
    glass_phase NUMERIC(6,2) DEFAULT 0, -- 玻璃相
    other_minerals NUMERIC(6,2) DEFAULT 0,
    -- 重金属残留 (mg/kg)
    pb_leaching NUMERIC(10,4) DEFAULT 0,  -- Pb 浸出浓度
    cd_leaching NUMERIC(10,4) DEFAULT 0,  -- Cd 浸出浓度
    as_leaching NUMERIC(10,4) DEFAULT 0,  -- As 浸出浓度
    cr_leaching NUMERIC(10,4) DEFAULT 0,  -- Cr 浸出浓度
    ni_leaching NUMERIC(10,4) DEFAULT 0,  -- Ni 浸出浓度
    -- 物理性质
    density NUMERIC(6,3) DEFAULT 0,       -- 密度 g/cm³
    specific_surface NUMERIC(8,2) DEFAULT 0,  -- 比表面积 m²/kg
    loss_on_ignition NUMERIC(6,2) DEFAULT 0,  -- 烧失量 %
    remark TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, measurement_year, sample_depth)
);

-- BRIN时序索引 + 复合索引
CREATE INDEX IF NOT EXISTS idx_slag_year_brin ON slag_compositions USING BRIN (measurement_year);
CREATE INDEX IF NOT EXISTS idx_slag_site_year ON slag_compositions (site_id, measurement_year DESC);

-- ====== 表2: farmland_soils 遗址周边农田土壤 ======
CREATE TABLE IF NOT EXISTS farmland_soils (
    id SERIAL PRIMARY KEY,
    site_id INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    measurement_year INTEGER NOT NULL,
    distance_from_site INTEGER NOT NULL,  -- 距遗址距离（米）：0-500, 500-1000, 1000-2000, 2000-5000
    direction VARCHAR(20),                -- 方向：N/S/E/W/NE/NW/SE/SW
    land_use_type VARCHAR(50) NOT NULL,   -- 土地利用：水田/旱地/菜地/果园/茶园
    -- 重金属 (mg/kg)
    pb NUMERIC(12,4) DEFAULT 0,
    zn NUMERIC(12,4) DEFAULT 0,
    cu NUMERIC(12,4) DEFAULT 0,
    as_ NUMERIC(12,4) DEFAULT 0,
    hg NUMERIC(12,4) DEFAULT 0,
    cd NUMERIC(12,4) DEFAULT 0,
    cr NUMERIC(12,4) DEFAULT 0,
    ni NUMERIC(12,4) DEFAULT 0,
    -- 土壤理化
    ph NUMERIC(5,2),
    organic_matter NUMERIC(8,4),
    cec NUMERIC(8,4),
    soil_type VARCHAR(50),
    -- 主要种植作物
    main_crops TEXT[] DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, measurement_year, distance_from_site, land_use_type)
);

CREATE INDEX IF NOT EXISTS idx_farmland_site_year ON farmland_soils (site_id, measurement_year DESC);
CREATE INDEX IF NOT EXISTS idx_farmland_year_brin ON farmland_soils USING BRIN (measurement_year);
CREATE INDEX IF NOT EXISTS idx_farmland_distance ON farmland_soils (distance_from_site);

-- ====== 表3: smelting_process_inversions 冶炼工艺反演结果 ======
CREATE TABLE IF NOT EXISTS smelting_process_inversions (
    id SERIAL PRIMARY KEY,
    site_id INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    measurement_year INTEGER NOT NULL,
    -- BPNN神经网络反演结果
    estimated_temperature NUMERIC(8,2) NOT NULL,  -- 估算冶炼温度（℃）
    temperature_confidence NUMERIC(5,3) NOT NULL, -- 温度估算置信度
    reducing_agent VARCHAR(50) NOT NULL,           -- 还原剂类型：木炭/焦炭/煤/混合
    reducing_agent_confidence NUMERIC(5,3) NOT NULL,
    -- 贝叶斯后验概率
    bpnn_posterior JSONB,          -- 温度后验分布: {mean, std, p5, p95}
    bayes_posterior JSONB,         -- 还原剂后验: {木炭: 0.7, 焦炭: 0.2, ...}
    -- 工艺特征
    process_type_detailed VARCHAR(100),  -- 详细工艺：块炼法/坩埚法/高炉法/灰吹法/焙烧法...
    process_era_estimate VARCHAR(100),   -- 年代估计：公元前/公元早期/中世纪/近代
    -- 输入特征摘要
    input_features JSONB,          -- 输入特征向量（浓度+比率+同位素）
    -- 模型质量
    bpnn_mse NUMERIC(8,6),         -- BPNN均方误差
    bayes_kld NUMERIC(8,6),        -- KL散度
    quality_level VARCHAR(20),     -- 反演质量：高/中/低
    remark TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, measurement_year)
);

CREATE INDEX IF NOT EXISTS idx_smelting_inv_site ON smelting_process_inversions (site_id);
CREATE INDEX IF NOT EXISTS idx_smelting_inv_temp ON smelting_process_inversions (estimated_temperature);
CREATE INDEX IF NOT EXISTS idx_smelting_inv_reducing ON smelting_process_inversions (reducing_agent);

-- ====== 表4: resource_utilization_assessments 矿渣资源化评估 ======
CREATE TABLE IF NOT EXISTS resource_utilization_assessments (
    id SERIAL PRIMARY KEY,
    site_id INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    measurement_year INTEGER NOT NULL,
    -- 水泥混合材可行性
    cement_blended_feasibility VARCHAR(20) NOT NULL,  -- 可行/条件可行/不可行
    cement_blended_score NUMERIC(5,2) NOT NULL,        -- 综合评分 0-100
    cement_blended_grade VARCHAR(20),                  -- 等级：S95/S75/S105
    cement_details JSONB,                              -- 详细指标: 活性指数/流动度比/含水率/烧失量
    -- 路基材料可行性
    road_base_feasibility VARCHAR(20) NOT NULL,
    road_base_score NUMERIC(5,2) NOT NULL,
    road_base_grade VARCHAR(20),                       -- 等级：一级/二级/三级
    road_details JSONB,                                -- CBR值/压碎值/塑性指数/冻融稳定性
    -- 其他利用途径
    other_uses JSONB,                                  -- 混凝土骨料/微晶玻璃/土壤改良剂...
    -- 环境风险
    leaching_risk_level VARCHAR(20),                   -- 环境浸出风险：低/中/高
    leaching_risk_details JSONB,                       -- 各重金属浸出浓度对比标准
    -- 综合推荐
    recommended_use VARCHAR(100) NOT NULL,             -- 最佳推荐用途
    utilization_plan JSONB,                            -- 推荐方案：处理工艺/成本估算/环境效益
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, measurement_year)
);

CREATE INDEX IF NOT EXISTS idx_resource_site ON resource_utilization_assessments (site_id);
CREATE INDEX IF NOT EXISTS idx_resource_cement ON resource_utilization_assessments (cement_blended_feasibility);
CREATE INDEX IF NOT EXISTS idx_resource_road ON resource_utilization_assessments (road_base_feasibility);

-- ====== 矿渣成分种子数据（30个遗址各1条） ======
INSERT INTO slag_compositions (site_id, measurement_year, sample_depth,
    sio2, al2o3, cao, feo, fe2o3, mgo, mno, p2o5, so3, k2o, na2o, tio2,
    fayalite, wollastonite, anorthite, diopside, magnetite, hematite, wuestite, glass_phase, other_minerals,
    pb_leaching, cd_leaching, as_leaching, cr_leaching, ni_leaching,
    density, specific_surface, loss_on_ignition, remark) VALUES
-- 铜冶炼矿渣（高FeO, SiO2, 典型铁橄榄石相）
(1, 2024, '0-30cm', 32.5, 8.2, 5.8, 42.0, 3.5, 2.1, 0.8, 0.15, 0.3, 0.8, 0.5, 0.45, 58, 4, 5, 3, 12, 4, 2, 10, 2, 1.8, 0.05, 0.25, 0.1, 0.15, 3.2, 420, 1.8, '青铜冶炼铁橄榄石渣'),
(2, 2024, '0-30cm', 30.2, 7.5, 6.2, 45.5, 3.0, 2.5, 0.9, 0.12, 0.25, 0.7, 0.4, 0.40, 62, 3, 4, 4, 10, 3, 3, 9, 2, 1.5, 0.04, 0.20, 0.08, 0.12, 3.3, 380, 1.5, '典型炼铜水淬渣'),
(3, 2024, '0-30cm', 28.8, 9.1, 7.5, 40.2, 4.8, 3.2, 1.0, 0.18, 0.35, 0.9, 0.6, 0.50, 52, 6, 7, 5, 8, 5, 4, 11, 2, 2.2, 0.06, 0.30, 0.12, 0.18, 3.1, 450, 2.1, '铜矿鼓风炉渣'),
-- 铁冶炼矿渣（高CaO, Al2O3, 钙长石/透辉石相）
(10, 2024, '0-30cm', 28.0, 15.5, 38.0, 6.5, 2.0, 8.2, 1.2, 0.25, 0.5, 0.6, 0.8, 1.2, 5, 25, 28, 18, 4, 2, 1, 15, 2, 0.5, 0.02, 0.08, 0.15, 0.08, 2.9, 320, 0.8, '高炉矿渣，高活性'),
(11, 2024, '0-30cm', 32.0, 12.8, 32.0, 8.5, 2.5, 6.5, 1.5, 0.30, 0.45, 0.7, 0.7, 1.5, 8, 20, 22, 15, 6, 3, 1, 22, 3, 0.8, 0.03, 0.10, 0.18, 0.10, 2.8, 350, 1.0, '块炼铁渣，玻璃相高'),
(12, 2024, '0-30cm', 25.5, 18.2, 42.0, 4.0, 1.5, 9.8, 0.8, 0.20, 0.55, 0.5, 0.9, 1.0, 2, 30, 35, 20, 2, 1, 0.5, 8, 1.5, 0.3, 0.01, 0.05, 0.10, 0.05, 2.7, 280, 0.5, '碱性高炉渣，潜在水硬性高'),
-- 银/铅冶炼矿渣（高Pb, Cd重金属残留）
(19, 2024, '0-30cm', 22.0, 6.5, 12.0, 18.0, 6.5, 3.5, 0.6, 0.08, 8.5, 1.2, 0.4, 0.35, 30, 10, 6, 4, 15, 8, 6, 18, 3, 15.0, 2.5, 5.8, 0.3, 0.5, 3.5, 520, 3.5, '炼铅炉渣，重金属浸出风险高'),
(20, 2024, '0-30cm', 20.5, 7.2, 10.5, 15.5, 8.0, 2.8, 0.5, 0.10, 12.0, 1.5, 0.5, 0.40, 25, 8, 5, 3, 18, 10, 7, 20, 4, 12.5, 1.8, 8.2, 0.25, 0.4, 3.6, 550, 4.2, '灰吹法银渣，SO3含量高'),
-- 汞冶炼矿渣
(25, 2024, '0-30cm', 35.0, 10.5, 4.5, 8.0, 3.0, 2.0, 0.3, 0.05, 2.5, 1.0, 0.3, 0.60, 18, 5, 10, 6, 8, 5, 2, 40, 6, 0.8, 0.02, 35.0, 0.10, 0.05, 2.6, 620, 2.8, '辰砂焙烧渣，Hg浸出极高'),
(26, 2024, '0-30cm', 38.0, 11.2, 5.0, 6.5, 2.5, 1.8, 0.25, 0.03, 1.8, 0.8, 0.25, 0.55, 15, 4, 8, 5, 6, 4, 1.5, 52, 3.5, 0.5, 0.01, 22.5, 0.08, 0.03, 2.5, 680, 2.2, '机械化炼汞渣，玻璃相占主导'),
-- 混合冶炼矿渣
(28, 2024, '0-30cm', 30.0, 10.0, 12.0, 25.0, 5.0, 4.5, 0.7, 0.15, 2.0, 1.0, 0.6, 0.80, 35, 12, 10, 8, 10, 6, 4, 12, 3, 5.5, 0.35, 1.5, 0.5, 0.25, 3.2, 480, 2.0, '铜铅混合冶炼渣')
ON CONFLICT DO NOTHING;

-- ====== 农田土壤种子数据（距遗址不同距离梯度） ======
INSERT INTO farmland_soils (site_id, measurement_year, distance_from_site, direction, land_use_type,
    pb, zn, cu, as_, hg, cd, cr, ni, ph, organic_matter, cec, soil_type, main_crops) VALUES
-- 遗址1周边
(1, 2024, 300, 'NE', '旱地', 420, 680, 850, 65, 2.8, 8.5, 75, 45, 6.8, 2.5, 18.5, '壤土', ARRAY['小麦','玉米']),
(1, 2024, 800, 'E', '水田', 185, 420, 320, 32, 0.85, 3.2, 65, 38, 5.5, 3.2, 22.0, '水稻土', ARRAY['水稻']),
(1, 2024, 1500, 'SE', '果园', 95, 280, 180, 18, 0.35, 1.5, 58, 32, 6.2, 2.0, 15.0, '黄壤', ARRAY['苹果','梨']),
(1, 2024, 3500, 'S', '旱地', 42, 120, 55, 8.5, 0.08, 0.28, 52, 28, 6.5, 1.8, 12.5, '褐土', ARRAY['小麦','大豆']),
-- 遗址10周边（铁冶炼）
(10, 2024, 400, 'W', '菜地', 280, 1250, 480, 78, 1.8, 6.2, 85, 55, 7.2, 3.0, 20.5, '潮土', ARRAY['白菜','萝卜','番茄']),
(10, 2024, 1200, 'NW', '旱地', 125, 680, 220, 38, 0.45, 2.0, 72, 42, 6.8, 2.2, 16.0, '褐土', ARRAY['玉米','花生']),
(10, 2024, 2800, 'N', '果园', 58, 320, 95, 15, 0.12, 0.65, 60, 35, 6.5, 1.5, 12.0, '棕壤', ARRAY['桃','葡萄']),
-- 遗址19周边（铅冶炼 - 严重）
(19, 2024, 200, 'S', '水田', 2850, 980, 280, 62, 3.5, 18.5, 95, 65, 5.8, 2.8, 19.0, '水稻土', ARRAY['水稻']),
(19, 2024, 600, 'SW', '旱地', 1560, 620, 180, 35, 1.6, 10.2, 82, 52, 6.0, 2.2, 15.5, '红壤', ARRAY['玉米','红薯']),
(19, 2024, 1800, 'W', '茶园', 580, 380, 120, 22, 0.45, 3.5, 70, 40, 4.8, 2.5, 14.0, '黄壤', ARRAY['茶叶']),
(19, 2024, 4500, 'NW', '果园', 165, 180, 65, 10, 0.10, 0.85, 58, 30, 5.5, 1.8, 10.5, '黄棕壤', ARRAY['柑橘']),
-- 遗址25周边（汞冶炼 - Hg超标严重）
(25, 2024, 350, 'E', '水田', 380, 340, 160, 185, 25.5, 4.2, 68, 38, 5.2, 2.0, 16.5, '水稻土', ARRAY['水稻']),
(25, 2024, 900, 'SE', '旱地', 185, 220, 95, 95, 8.5, 1.8, 60, 32, 5.8, 1.5, 12.0, '黄壤', ARRAY['玉米','烟草']),
(25, 2024, 2200, 'S', '林地', 65, 130, 48, 38, 0.95, 0.55, 55, 28, 6.0, 4.5, 18.0, '黄棕壤', ARRAY['杉木','松树'])
ON CONFLICT DO NOTHING;

-- ====== 冶炼工艺反演种子数据 ======
INSERT INTO smelting_process_inversions (site_id, measurement_year,
    estimated_temperature, temperature_confidence, reducing_agent, reducing_agent_confidence,
    bpnn_posterior, bayes_posterior, process_type_detailed, process_era_estimate,
    input_features, bpnn_mse, bayes_kld, quality_level, remark) VALUES
-- 铜冶炼 - 青铜时代
(1, 2024, 1080.0, 0.88, '木炭', 0.92,
    '{"mean":1080,"std":45,"p5":1010,"p95":1160}', '{"木炭":0.92,"焦炭":0.05,"煤":0.02,"混合":0.01}',
    '坩埚还原熔炼法', '公元前2000-1000年 青铜时代早期',
    '{"pb_zn":0.56,"cu_pb":26.7,"as_hg":40,"feo_total":45.5}', 0.0125, 0.156, '高', '高温高置信度，典型青铜冶炼'),
(2, 2024, 1150.0, 0.85, '木炭', 0.88,
    '{"mean":1150,"std":52,"p5":1070,"p95":1240}', '{"木炭":0.88,"焦炭":0.08,"煤":0.03,"混合":0.01}',
    '竖炉熔炼法', '公元前1500-500年 青铜时代晚期',
    '{"pb_zn":0.38,"cu_pb":20.0,"as_hg":36,"feo_total":48.5}', 0.0182, 0.178, '高', '温度较高，已出现竖炉技术'),
-- 铁冶炼 - 中世纪
(10, 2024, 1450.0, 0.92, '焦炭', 0.95,
    '{"mean":1450,"std":38,"p5":1390,"p95":1520}', '{"木炭":0.03,"焦炭":0.95,"煤":0.01,"混合":0.01}',
    '近代高炉法', '公元1700-1900年 工业革命时期',
    '{"cao_sio2":1.36,"al2o3":15.5,"feo_total":8.5,"mgo":8.2}', 0.0085, 0.085, '高', '典型高温高炉，焦炭还原剂'),
(11, 2024, 1180.0, 0.80, '木炭', 0.78,
    '{"mean":1180,"std":68,"p5":1075,"p95":1295}', '{"木炭":0.78,"焦炭":0.10,"煤":0.07,"混合":0.05}',
    '古代块炼法', '公元前500-公元500年 铁器时代',
    '{"cao_sio2":1.0,"al2o3":12.8,"feo_total":11.0,"glass":22}', 0.0325, 0.285, '中', '温度区间跨度大，块炼铁特征'),
-- 银/铅冶炼
(19, 2024, 950.0, 0.82, '木炭', 0.90,
    '{"mean":950,"std":55,"p5":865,"p95":1040}', '{"木炭":0.90,"焦炭":0.06,"煤":0.03,"混合":0.01}',
    '灰吹法-铅置换', '公元前300-公元500年 罗马时期',
    '{"pb_zn":8.67,"so3":8.5,"ag_trace":1,"cu_pb":0.08}', 0.0156, 0.125, '高', '温度较低，典型灰吹法温度区间'),
(20, 2024, 650.0, 0.75, '混合', 0.65,
    '{"mean":650,"std":85,"p5":510,"p95":790}', '{"木炭":0.30,"焦炭":0.20,"煤":0.15,"混合":0.35}',
    '混汞法-低温焙烧', '公元1500-1800年 殖民时期',
    '{"pb_zn":6.80,"as_hg":0.51,"hg_total":45,"so3":12}', 0.0482, 0.365, '中', '多峰后验分布，低温区间不明确'),
-- 汞冶炼
(25, 2024, 680.0, 0.90, '木炭', 0.88,
    '{"mean":680,"std":42,"p5":615,"p95":750}', '{"木炭":0.88,"焦炭":0.08,"煤":0.03,"混合":0.01}',
    '辰砂焙烧-冷凝法', '公元前500-公元1500年',
    '{"as_hg":0.0078,"hg_total":320,"glass_phase":40,"sio2":35}', 0.0112, 0.098, '高', 'Hg沸点357℃，焙烧温度600-700特征'),
(26, 2024, 750.0, 0.88, '煤', 0.82,
    '{"mean":750,"std":48,"p5":675,"p95":830}', '{"木炭":0.10,"焦炭":0.07,"煤":0.82,"混合":0.01}',
    '机械化回转窑焙烧', '公元1900-2000年 近现代',
    '{"as_hg":0.011,"hg_total":250,"glass_phase":52,"so3":1.8}', 0.0158, 0.135, '高', '温度略高，工业规模煤燃烧'),
-- 混合冶炼
(28, 2024, 1250.0, 0.78, '木炭', 0.72,
    '{"mean":1250,"std":78,"p5":1125,"p95":1380}', '{"木炭":0.72,"焦炭":0.15,"煤":0.08,"混合":0.05}',
    '铜铅共生矿处理', '中世纪时期',
    '{"pb_zn":1.80,"feo_total":30,"cu_fe_ratio":0.48,"glass":12}', 0.0385, 0.312, '中', '温度区间宽，混合工艺特征')
ON CONFLICT DO NOTHING;

-- ====== 资源化利用评估种子数据 ======
INSERT INTO resource_utilization_assessments (site_id, measurement_year,
    cement_blended_feasibility, cement_blended_score, cement_blended_grade, cement_details,
    road_base_feasibility, road_base_score, road_base_grade, road_details,
    other_uses, leaching_risk_level, leaching_risk_details,
    recommended_use, utilization_plan) VALUES
-- 高炉铁渣：最佳水泥混合材
(10, 2024, '可行', 92.5, 'S95',
    '{"activity_index_7d":78,"activity_index_28d":98,"flow_ratio":102,"water_content":0.3,"loss_on_ignition":0.8,"fineness":420}',
    '可行', 85.0, '一级',
    '{"cbr":185,"crush_value":12,"plasticity_index":5.5,"freeze_thaw_loss":1.8,"abrasion":8.5}',
    '{"concrete_aggregate":{"score":75,"grade":"II类"},"soil_conditioner":{"score":60,"note":"需检测重金属"}}',
    '低',
    '{"pb":0.5,"cd":0.02,"as":0.08,"cr":0.15,"standard":"GB5085.3-2007","pass":true}',
    '优先作为S95级粒化高炉矿渣粉（水泥混合材）',
    '{"process":["水淬粒化","辊压粉磨","比表面积420m²/kg检测"],"cost_estimation":{"processing":85,"transport":45,"total":130},"unit":"元/吨","benefits":{"co2_reduction_kg_per_ton":850,"cement_clinker_saving_pct":45}}'),
-- 块炼铁渣：水泥/路基两用
(11, 2024, '条件可行', 72.0, 'S75',
    '{"activity_index_7d":58,"activity_index_28d":78,"flow_ratio":98,"water_content":0.5,"loss_on_ignition":1.0,"note":"需细磨至500目提高活性"}',
    '可行', 82.0, '一级',
    '{"cbr":165,"crush_value":14,"plasticity_index":6.8,"freeze_thaw_loss":2.2,"abrasion":10}',
    '{"concrete_aggregate":{"score":70,"grade":"II类"},"microcrystalline_glass":{"score":55,"note":"SiO2+Al2O3>50%"}}',
    '低',
    '{"pb":0.8,"cd":0.03,"as":0.10,"cr":0.18,"pass":true}',
    '路基材料（优先）/ S75水泥混合材',
    '{"process":["破碎筛分","分级","固化稳定化预处理（可选）"],"cost_estimation":{"processing":35,"transport":40,"total":75},"unit":"元/吨","benefits":{"road_base_replacement_pct":100,"natural_aggregate_saving_ton":1.2}}'),
-- 铜冶炼渣：路基+微晶玻璃
(1, 2024, '条件可行', 58.0, NULL,
    '{"activity_index_7d":38,"activity_index_28d":52,"flow_ratio":92,"water_content":0.8,"note":"活性低，需碱性激发或掺量≤20%"}',
    '可行', 88.0, '一级',
    '{"cbr":195,"crush_value":10,"plasticity_index":3.2,"freeze_thaw_loss":1.2,"abrasion":6.5}',
    '{"microcrystalline_glass":{"score":85,"grade":"A","note":"FeO+SiO2高，耐磨铸石原料"},"concrete_aggregate":{"score":80,"grade":"I类"}}',
    '中',
    '{"pb":1.8,"cd":0.05,"as":0.25,"cr":0.10,"standard":"GB5085.3-2007","pass":true,"note":"需检测长期稳定性"}',
    '道路基层碎石骨料 + 耐磨微晶玻璃原料',
    '{"process":["破碎筛分","磁选回收铁","分级","微晶玻璃配料(可选)"],"cost_estimation":{"aggregate_processing":25,"glass_processing":280,"transport":45},"unit":"元/吨","benefits":{"iron_recovery_pct":8,"aggregate_value_pct":100}}'),
-- 炼铅渣：高风险需固化
(19, 2024, '不可行', 18.0, NULL,
    '{"activity_index_7d":25,"note":"重金属超标，直接用于水泥将造成环境污染"}',
    '条件可行', 38.0, NULL,
    '{"cbr":120,"crush_value":18,"note":"Pb/Cd浸出超标，必须先固化稳定化处理"}',
    '{"safe_landfill":{"score":80,"note":"稳定化后填埋"},"recover_metals":{"score":65,"note":"可浮选回收残余Pb/Ag"}}',
    '高',
    '{"pb":15.0,"cd":2.5,"as":5.8,"standard":"GB5085.3-2007","pass":false,"note":"Pb超标4倍，Cd超标12倍，As超标5倍"}',
    '稳定化处理后安全填埋 + 有价金属回收',
    '{"process":["浮选回收Pb/Ag(回收率65-75%)","螯合剂固化稳定化","养护28d检测","达标填埋/路基"],"cost_estimation":{"metal_recovery":220,"stabilization":180,"landfill":60,"total":460},"unit":"元/吨","benefits":{"metal_recovery_value_150元/吨":null,"leaching_risk_reduction_pct":95}}'),
-- 炼汞渣：极高风险 - 仅安全处置
(25, 2024, '不可行', 5.0, NULL,
    '{"note":"Hg浸出远超标准，绝对禁止进入建材"}',
    '不可行', 12.0, NULL,
    '{"note":"Hg浸出浓度35mg/L，超标准350倍，必须稳定化+安全填埋"}',
    '{"safe_landfill":{"score":90,"note":"唯一合规途径"},"mercury_recovery":{"score":40,"note":"热解回收残余Hg(70%)"}}',
    '极高',
    '{"pb":0.8,"cd":0.02,"as":35.0,"hg":35.0,"standard":"GB5085.3-2007","pass":false,"note":"As超标17倍，Hg超标700倍"}',
    '热解析回收Hg + 硫化钠稳定化 + 防渗安全填埋',
    '{"process":["600℃热解析(回收率70-85%)","Na2S·9H2O稳定化","养护检测","HDPE膜防渗填埋","监测30年"],"cost_estimation":{"thermal_desorption":580,"stabilization":250,"landfill":380,"total":1210},"unit":"元/吨","benefits":{"hg_recovery_500g/吨":null,"leaching_risk_reduction_pct":99.5}}'),
-- 铜渣另一例
(3, 2024, '条件可行', 55.0, NULL,
    '{"activity_index_7d":35,"activity_index_28d":50,"note":"碱性激发后可掺20%"}',
    '可行', 86.0, '一级',
    '{"cbr":188,"crush_value":11,"plasticity_index":4.0,"freeze_thaw_loss":1.5}',
    '{"concrete_aggregate":{"score":78,"grade":"II类"},"microcrystalline_glass":{"score":82,"grade":"A"}}',
    '中',
    '{"pb":2.2,"cd":0.06,"as":0.30,"pass":true,"note":"As接近限值"}',
    '道路基层骨料 + 可选微晶玻璃原料',
    '{"process":["破碎筛分","磁选回收铁","用于道路水稳层"],"cost_estimation":{"processing":28,"transport":42,"total":70},"unit":"元/吨","benefits":{"iron_recovery_pct":6,"aggregate_replacement":1.1}}')
ON CONFLICT DO NOTHING;
