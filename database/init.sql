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
-- 污染指数计算视图
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
