// ============================================
// app.js - 应用入口 & 胶水代码
//
// 功能划分：
//   pollution_map.js - 地图 / Canvas / 聚合 / 绘制
//   site_detail.js   - 详情面板 / 趋势图 / 指纹 / 修复推荐
//   app.js           - 初始化、筛选器、API调用、模块调度
// ============================================

const API_BASE = '/api';
let allSites = [];
let activeFilters = { metals: {}, scales: {}, pollutions: {} };

// 污染等级分类配置（前端筛选用）
const POLLUTION_LEVELS = [
    { key: '清洁', range: [0, 1], color: '#4caf50' },
    { key: '轻度', range: [1, 2], color: '#8bc34a' },
    { key: '中度', range: [2, 3], color: '#ffc107' },
    { key: '重度', range: [3, 5], color: '#ff9800' },
    { key: '严重', range: [5, Infinity], color: '#f44336' }
];

// ============================================
// 初始化
// ============================================
document.addEventListener('DOMContentLoaded', async () => {
    initFilters();
    initFilterButtons();

    PollutionMap.init({
        mapContainerId: 'map',
        onClick: site => SiteDetail.show(site),
        filters: () => activeFilters
    });

    SiteDetail.setAPIBase(API_BASE);

    await loadSites();
    refreshStats();

    // 定期刷新（用于模拟器上报后自动更新）
    setInterval(() => { loadSites().catch(() => {}); refreshStats().catch(() => {}); }, 15000);
});

// ============================================
// 数据加载
// ============================================
async function loadSites() {
    try {
        const r = await fetch(API_BASE + '/sites');
        const d = await r.json();
        allSites = d.data || [];
        PollutionMap.updateSites(allSites);
        updateFiltersBadge();
    } catch (e) {
        console.error('加载遗址失败:', e);
    }
}

async function refreshStats() {
    try {
        const r = await fetch(API_BASE + '/stats');
        const d = await r.json();
        const c = d.data?.counts || {};
        const sb = d.data?.severity_breakdown || {};
        document.getElementById('statSites').textContent = c.sites_total || 0;
        document.getElementById('statAlerts').textContent = c.alerts_total || 0;
        document.getElementById('statSevere').textContent = sb['严重'] || 0;
        document.getElementById('statPending').textContent = d.data?.pending_alerts || 0;
    } catch (e) { /* 静默失败 */ }
}

// ============================================
// 筛选器
// ============================================
function initFilters() {
    const metals = ['铜', '铁', '银', '铅', '汞', '青铜', '混合'];
    const scales = ['small', 'medium', 'large', 'mega'];
    const scaleNames = { small: '小型', medium: '中型', large: '大型', mega: '巨型' };

    const mf = document.getElementById('metalFilters');
    metals.forEach(m => {
        const chk = mkFilter('metal', m, m);
        mf.appendChild(chk);
        activeFilters.metals[m] = true;
    });

    const sf = document.getElementById('scaleFilters');
    scales.forEach(s => {
        const chk = mkFilter('scale', s, scaleNames[s]);
        sf.appendChild(chk);
        activeFilters.scales[s] = true;
    });

    const pf = document.getElementById('pollutionFilters');
    POLLUTION_LEVELS.forEach(lv => {
        const chk = mkFilter('pollution', lv.key, lv.key, lv.color);
        pf.appendChild(chk);
        activeFilters.pollutions[lv.key] = true;
    });
}

function mkFilter(group, key, label, color) {
    const item = document.createElement('label');
    item.className = 'filter-item';
    const input = document.createElement('input');
    input.type = 'checkbox';
    input.checked = true;
    input.dataset.group = group;
    input.dataset.key = key;
    input.addEventListener('change', onFilterChange);
    const dot = document.createElement('span');
    dot.className = 'filter-dot';
    if (color) dot.style.background = color;
    const text = document.createElement('span');
    text.className = 'filter-label';
    text.textContent = label;
    item.appendChild(input);
    item.appendChild(dot);
    item.appendChild(text);
    return item;
}

function onFilterChange(e) {
    const { group, key } = e.target.dataset;
    if (group === 'metal') activeFilters.metals[key] = e.target.checked;
    if (group === 'scale') activeFilters.scales[key] = e.target.checked;
    if (group === 'pollution') activeFilters.pollutions[key] = e.target.checked;
    PollutionMap.updateFilters(() => activeFilters);
}

function initFilterButtons() {
    document.getElementById('resetFilters').addEventListener('click', () => {
        document.querySelectorAll('.filter-item input').forEach(i => i.checked = true);
        Object.keys(activeFilters.metals).forEach(k => activeFilters.metals[k] = true);
        Object.keys(activeFilters.scales).forEach(k => activeFilters.scales[k] = true);
        Object.keys(activeFilters.pollutions).forEach(k => activeFilters.pollutions[k] = true);
        PollutionMap.updateFilters(() => activeFilters);
    });
    document.getElementById('clearFilters').addEventListener('click', () => {
        document.querySelectorAll('.filter-item input').forEach(i => i.checked = false);
        Object.keys(activeFilters.metals).forEach(k => activeFilters.metals[k] = false);
        Object.keys(activeFilters.scales).forEach(k => activeFilters.scales[k] = false);
        Object.keys(activeFilters.pollutions).forEach(k => activeFilters.pollutions[k] = false);
        PollutionMap.updateFilters(() => activeFilters);
    });
}

function updateFiltersBadge() {
    const counts = { '铜': 0, '铁': 0, '银': 0, '铅': 0, '汞': 0, '青铜': 0, '混合': 0 };
    allSites.forEach(s => { if (counts[s.metal_type] !== undefined) counts[s.metal_type]++; });
    document.querySelectorAll('.filter-item').forEach(fi => {
        const label = fi.querySelector('.filter-label');
        const key = fi.querySelector('input')?.dataset?.key;
        if (key && counts[key]) {
            if (!fi.querySelector('.filter-badge')) {
                const bd = document.createElement('span');
                bd.className = 'filter-badge';
                fi.appendChild(bd);
            }
            const bd = fi.querySelector('.filter-badge');
            bd.textContent = counts[key];
        }
    });
}

// 兼容legacy代码（全局变量引用）
window.getActiveFiltersLegacy = function () { return activeFilters; };
