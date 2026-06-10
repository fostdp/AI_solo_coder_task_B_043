// ============================================
// app.js - 应用入口 & 胶水代码
// ============================================

const API_BASE = '/api';
let allSites = [];
let activeFilters = { metals: {}, scales: {}, pollutions: {} };

const POLLUTION_LEVELS = [
    { key: '清洁', range: [0, 1], color: '#4caf50' },
    { key: '轻度', range: [1, 2], color: '#8bc34a' },
    { key: '中度', range: [2, 3], color: '#ffc107' },
    { key: '重度', range: [3, 5], color: '#ff9800' },
    { key: '严重', range: [5, Infinity], color: '#f44336' }
];

document.addEventListener('DOMContentLoaded', async () => {
    PollutionMap.init({
        mapContainerId: 'map',
        onClick: site => SiteDetail.show(site),
        filters: () => activeFilters
    });

    SiteDetail.setAPIBase(API_BASE);
    if (typeof SmeltingProcess !== 'undefined' && SmeltingProcess.setAPIBase) SmeltingProcess.setAPIBase(API_BASE);
    if (typeof FarmSafety !== 'undefined' && FarmSafety.setAPIBase) FarmSafety.setAPIBase(API_BASE);
    if (typeof SlagRecycle !== 'undefined' && SlagRecycle.setAPIBase) SlagRecycle.setAPIBase(API_BASE);
    if (typeof TimelineCompare !== 'undefined' && TimelineCompare.setAPIBase) TimelineCompare.setAPIBase(API_BASE);

    await loadSites();
    refreshStats();

    initSidebarFilters();
    initTimelineModal();

    setInterval(() => { loadSites().catch(() => {}); refreshStats().catch(() => {}); }, 15000);
});

async function loadSites() {
    try {
        const r = await fetch(API_BASE + '/sites');
        const d = await r.json();
        allSites = d.data || [];
        PollutionMap.updateSites(allSites);
    } catch (e) {
        console.error('加载遗址失败:', e);
    }
}

async function refreshStats() {
    try {
        const r = await fetch(API_BASE + '/stats');
        const d = await r.json();
        const c = (d.data && d.data.counts) || {};
        const sb = (d.data && d.data.severity_breakdown) || {};
        document.getElementById('totalSites').textContent = c.sites_total || 0;
        document.getElementById('sitesWithData').textContent = c.measurements || 0;
        document.getElementById('severeCount').textContent = sb['严重'] || 0;
        document.getElementById('alertCount').textContent = d.data?.pending_alerts || 0;
    } catch (e) { /* 静默失败 */ }
}

function resetMapView() {
    if (PollutionMap.resetView) PollutionMap.resetView();
}

function refreshData() {
    loadSites();
    refreshStats();
}

function closePanel() {
    if (SiteDetail) SiteDetail.close();
}

function switchTab(name) {
    var fn = window._siteDetailSwitchTab;
    if (typeof fn === 'function') fn(name);
}

// ====== 侧栏筛选器 ======
function initSidebarFilters() {
    var checks = document.querySelectorAll('#showAll,#filterCu,#filterFe,#filterAg,#filterPb,#filterHg');
    checks.forEach(function(chk) {
        chk.addEventListener('change', onSidebarFilterChange);
    });
}

function onSidebarFilterChange() {
    var showAll = document.getElementById('showAll');
    if (showAll && showAll.checked) {
        Object.keys(activeFilters.metals).forEach(function(k) { activeFilters.metals[k] = true; });
    } else {
        activeFilters.metals = {
            '铜': document.getElementById('filterCu')?.checked || false,
            '铁': document.getElementById('filterFe')?.checked || false,
            '银': document.getElementById('filterAg')?.checked || false,
            '铅': document.getElementById('filterPb')?.checked || false,
            '汞': document.getElementById('filterHg')?.checked || false
        };
    }
    PollutionMap.updateFilters(function() { return activeFilters; });
}

// ====== 时间线模态框 ======
function initTimelineModal() {
    if (typeof TimelineCompare !== 'undefined' && TimelineCompare.init) {
        TimelineCompare.init(allSites);
    }
}

function openTimelineModal() {
    var modal = document.getElementById('timelineModal');
    if (modal) {
        modal.style.display = 'flex';
        if (typeof TimelineCompare !== 'undefined' && TimelineCompare.init) {
            TimelineCompare.init(allSites);
        }
    }
}

function closeTimelineModal() {
    var modal = document.getElementById('timelineModal');
    if (modal) modal.style.display = 'none';
    if (typeof TimelineCompare !== 'undefined' && TimelineCompare.stopAnimation) {
        TimelineCompare.stopAnimation();
    }
}

function loadTimelineData() {
    var select = document.getElementById('timelineSiteSelect');
    var ids = [];
    if (select) {
        for (var i = 0; i < select.options.length; i++) {
            if (select.options[i].selected) {
                ids.push(parseInt(select.options[i].value));
            }
        }
    }
    if (typeof TimelineCompare !== 'undefined' && TimelineCompare.load) {
        TimelineCompare.load(ids.length > 0 ? ids : null);
    }
}

function toggleTimelineAnimation() {
    if (typeof TimelineCompare !== 'undefined' && TimelineCompare.toggleAnimation) {
        TimelineCompare.toggleAnimation();
    }
}
