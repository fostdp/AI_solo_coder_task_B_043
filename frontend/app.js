const API_BASE = '/api';
let map;
let canvas;
let canvasCtx;
let sites = [];
let currentSite = null;
let allSites = [];
let clusters = [];
let isClusteringEnabled = false;

const METAL_COLORS = {
    'Pb': '#6ea8fe',
    'Zn': '#a371f7',
    'Cu': '#3fb950',
    'As': '#d29922',
    'Hg': '#f85149',
    'Cd': '#f778ba'
};

const SCALE_RADIUS = {
    '小型': 6,
    '中型': 10,
    '大型': 15,
    '超大型': 22
};

function getPollutionColor(pi) {
    if (!pi || pi <= 0) return '#9e9e9e';
    if (pi < 1) return '#1b7a34';
    if (pi < 2) return '#8bc34a';
    if (pi < 3) return '#ffeb3b';
    if (pi < 5) return '#ff9800';
    return '#f44336';
}

function getPollutionClass(pi) {
    if (!pi || pi <= 0) return 'none';
    if (pi < 1) return 'low';
    if (pi < 2) return 'medium-low';
    if (pi < 3) return 'medium';
    if (pi < 5) return 'high';
    return 'severe';
}

function getPollutionText(pi) {
    if (!pi || pi <= 0) return '暂无数据';
    if (pi < 1) return '清洁';
    if (pi < 2) return '轻度污染';
    if (pi < 3) return '中度污染';
    if (pi < 5) return '重度污染';
    return '严重污染';
}

function initMap() {
    map = L.map('map', {
        center: [25, 15],
        zoom: 2,
        minZoom: 2,
        maxZoom: 10,
        worldCopyJump: true
    });

    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '© OpenStreetMap contributors',
        maxZoom: 19
    }).addTo(map);

    canvas = document.getElementById('siteCanvas');
    canvasCtx = canvas.getContext('2d');
    resizeCanvas();

    map.on('move', drawSitesOnCanvas);
    map.on('moveend', drawSitesOnCanvas);
    map.on('zoom', drawSitesOnCanvas);
    map.on('zoomend', drawSitesOnCanvas);
    map.on('resize', () => {
        resizeCanvas();
        drawSitesOnCanvas();
    });

    canvas.addEventListener('click', handleCanvasClick);
    canvas.addEventListener('mousemove', handleCanvasHover);

    window.addEventListener('resize', () => {
        resizeCanvas();
        drawSitesOnCanvas();
    });
}

function resizeCanvas() {
    const container = document.getElementById('map');
    canvas.width = container.clientWidth;
    canvas.height = container.clientHeight;
    canvas.style.width = container.clientWidth + 'px';
    canvas.style.height = container.clientHeight + 'px';
}

function getClusterGridSize(zoom) {
    if (zoom <= 3) return 120;
    if (zoom <= 4) return 100;
    if (zoom <= 5) return 80;
    return 60;
}

function shouldUseClustering(zoom) {
    return zoom <= 5;
}

function buildClusters(filteredSites, gridSize) {
    const clusterMap = new Map();
    const clusters = [];

    filteredSites.forEach(site => {
        const point = map.latLngToContainerPoint([site.latitude, site.longitude]);
        if (!point) return;

        if (point.x < -100 || point.x > canvas.width + 100 || 
            point.y < -100 || point.y > canvas.height + 100) {
            return;
        }

        const gridX = Math.floor(point.x / gridSize);
        const gridY = Math.floor(point.y / gridSize);
        const key = `${gridX},${gridY}`;

        if (!clusterMap.has(key)) {
            const cluster = {
                x: 0,
                y: 0,
                sites: [],
                maxPollutionIndex: 0,
                gridX: gridX,
                gridY: gridY,
                count: 0
            };
            clusterMap.set(key, cluster);
            clusters.push(cluster);
        }

        const cluster = clusterMap.get(key);
        cluster.sites.push(site);
        cluster.x += point.x;
        cluster.y += point.y;
        cluster.count++;
        if (site.pollution_index > cluster.maxPollutionIndex) {
            cluster.maxPollutionIndex = site.pollution_index;
        }

        site._canvasX = point.x;
        site._canvasY = point.y;
    });

    clusters.forEach(cluster => {
        cluster.x /= cluster.count;
        cluster.y /= cluster.count;
        cluster.radius = getClusterRadius(cluster.count);
    });

    return clusters;
}

function getClusterRadius(count) {
    if (count <= 2) return 18;
    if (count <= 5) return 24;
    if (count <= 10) return 30;
    if (count <= 20) return 36;
    return 42;
}

function drawCluster(cluster) {
    const color = getPollutionColor(cluster.maxPollutionIndex);
    const radius = cluster.radius;
    const x = cluster.x;
    const y = cluster.y;

    canvasCtx.beginPath();
    canvasCtx.arc(x + 2, y + 2, radius, 0, Math.PI * 2);
    canvasCtx.fillStyle = 'rgba(0, 0, 0, 0.3)';
    canvasCtx.fill();

    const gradient = canvasCtx.createRadialGradient(x, y, 0, x, y, radius);
    gradient.addColorStop(0, color);
    gradient.addColorStop(0.6, color);
    gradient.addColorStop(1, adjustColor(color, -40));
    canvasCtx.beginPath();
    canvasCtx.arc(x, y, radius, 0, Math.PI * 2);
    canvasCtx.fillStyle = gradient;
    canvasCtx.fill();

    canvasCtx.strokeStyle = 'rgba(255, 255, 255, 0.9)';
    canvasCtx.lineWidth = 2;
    canvasCtx.stroke();

    canvasCtx.fillStyle = '#fff';
    canvasCtx.font = 'bold 13px sans-serif';
    canvasCtx.textAlign = 'center';
    canvasCtx.textBaseline = 'middle';
    canvasCtx.fillText(cluster.count.toString(), x, y - 1);

    canvasCtx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    canvasCtx.font = '9px sans-serif';
    canvasCtx.fillText('处遗址', x, y + 12);

    cluster._canvasX = x;
    cluster._canvasY = y;
    cluster._canvasRadius = radius;
}

function drawSiteMarker(site, zoom) {
    const point = { x: site._canvasX, y: site._canvasY };
    if (!point.x || !point.y) return;

    if (point.x < -50 || point.x > canvas.width + 50 || 
        point.y < -50 || point.y > canvas.height + 50) {
        return;
    }

    const baseRadius = SCALE_RADIUS[site.scale] || 8;
    const scaleFactor = Math.max(0.5, Math.min(1.5, (zoom - 2) / 4 + 0.5));
    const radius = baseRadius * scaleFactor;
    const color = getPollutionColor(site.pollution_index);

    canvasCtx.beginPath();
    canvasCtx.arc(point.x + 2, point.y + 2, radius, 0, Math.PI * 2);
    canvasCtx.fillStyle = 'rgba(0, 0, 0, 0.3)';
    canvasCtx.fill();

    const gradient = canvasCtx.createRadialGradient(point.x, point.y, 0, point.x, point.y, radius);
    gradient.addColorStop(0, color);
    gradient.addColorStop(0.7, color);
    gradient.addColorStop(1, adjustColor(color, -30));
    canvasCtx.beginPath();
    canvasCtx.arc(point.x, point.y, radius, 0, Math.PI * 2);
    canvasCtx.fillStyle = gradient;
    canvasCtx.fill();

    canvasCtx.strokeStyle = 'rgba(255, 255, 255, 0.8)';
    canvasCtx.lineWidth = 1.5;
    canvasCtx.stroke();

    if (radius >= 10) {
        canvasCtx.fillStyle = 'rgba(255, 255, 255, 0.9)';
        canvasCtx.font = `bold ${Math.max(8, radius * 0.5)}px sans-serif`;
        canvasCtx.textAlign = 'center';
        canvasCtx.textBaseline = 'middle';
        canvasCtx.fillText(getMetalIcon(site.metal_type), point.x, point.y);
    }

    site._canvasRadius = radius;
}

function drawSitesOnCanvas() {
    if (!canvasCtx || sites.length === 0) return;

    canvasCtx.clearRect(0, 0, canvas.width, canvas.height);

    const filters = getActiveFilters();
    const zoom = map.getZoom();
    const filteredSites = sites.filter(site => shouldShowSite(site, filters));

    isClusteringEnabled = shouldUseClustering(zoom);

    if (isClusteringEnabled) {
        const gridSize = getClusterGridSize(zoom);
        clusters = buildClusters(filteredSites, gridSize);
        clusters.forEach(cluster => drawCluster(cluster));
    } else {
        clusters = [];
        filteredSites.forEach(site => {
            const point = map.latLngToContainerPoint([site.latitude, site.longitude]);
            if (point) {
                site._canvasX = point.x;
                site._canvasY = point.y;
                drawSiteMarker(site, zoom);
            }
        });
    }
}

function getMetalIcon(metalType) {
    const icons = { '铜': 'Cu', '铁': 'Fe', '银': 'Ag', '铅': 'Pb', '汞': 'Hg' };
    return icons[metalType] || '●';
}

function adjustColor(hex, amount) {
    const num = parseInt(hex.replace('#', ''), 16);
    const r = Math.min(255, Math.max(0, (num >> 16) + amount));
    const g = Math.min(255, Math.max(0, ((num >> 8) & 0x00FF) + amount));
    const b = Math.min(255, Math.max(0, (num & 0x0000FF) + amount));
    return `#${(1 << 24 | r << 16 | g << 8 | b).toString(16).slice(1)}`;
}

function getActiveFilters() {
    return {
        cu: document.getElementById('filterCu').checked,
        fe: document.getElementById('filterFe').checked,
        ag: document.getElementById('filterAg').checked,
        pb: document.getElementById('filterPb').checked,
        hg: document.getElementById('filterHg').checked,
        showAll: document.getElementById('showAll').checked
    };
}

function shouldShowSite(site, filters) {
    if (filters.showAll) return true;
    const metalMap = { '铜': 'cu', '铁': 'fe', '银': 'ag', '铅': 'pb', '汞': 'hg' };
    return filters[metalMap[site.metal_type]];
}

function handleCanvasClick(e) {
    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;

    if (isClusteringEnabled && clusters.length > 0) {
        let clickedCluster = null;
        let minDist = Infinity;

        clusters.forEach(cluster => {
            if (cluster._canvasX === undefined) return;
            const dx = x - cluster._canvasX;
            const dy = y - cluster._canvasY;
            const dist = Math.sqrt(dx * dx + dy * dy);
            if (dist <= (cluster._canvasRadius + 5) && dist < minDist) {
                minDist = dist;
                clickedCluster = cluster;
            }
        });

        if (clickedCluster) {
            const currentZoom = map.getZoom();
            const newZoom = Math.min(currentZoom + 2, map.getMaxZoom());
            
            if (clickedCluster.sites.length === 1) {
                showSiteDetail(clickedCluster.sites[0]);
            } else {
                map.setView([
                    (clickedCluster.sites.reduce((sum, s) => sum + s.latitude, 0) / clickedCluster.sites.length),
                    (clickedCluster.sites.reduce((sum, s) => sum + s.longitude, 0) / clickedCluster.sites.length)
                ], newZoom);
            }
            return;
        }
    }

    let clickedSite = null;
    let minDist = Infinity;

    sites.forEach(site => {
        if (site._canvasX === undefined) return;
        const dx = x - site._canvasX;
        const dy = y - site._canvasY;
        const dist = Math.sqrt(dx * dx + dy * dy);
        if (dist <= (site._canvasRadius + 5) && dist < minDist) {
            minDist = dist;
            clickedSite = site;
        }
    });

    if (clickedSite) {
        showSiteDetail(clickedSite);
    }
}

function handleCanvasHover(e) {
    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;

    let isHovering = false;

    if (isClusteringEnabled && clusters.length > 0) {
        clusters.forEach(cluster => {
            if (cluster._canvasX === undefined) return;
            const dx = x - cluster._canvasX;
            const dy = y - cluster._canvasY;
            const dist = Math.sqrt(dx * dx + dy * dy);
            if (dist <= (cluster._canvasRadius + 5)) {
                isHovering = true;
            }
        });
    } else {
        sites.forEach(site => {
            if (site._canvasX === undefined) return;
            const dx = x - site._canvasX;
            const dy = y - site._canvasY;
            const dist = Math.sqrt(dx * dx + dy * dy);
            if (dist <= (site._canvasRadius + 5)) {
                isHovering = true;
            }
        });
    }

    canvas.style.cursor = isHovering ? 'pointer' : 'default';
}

async function loadSites() {
    try {
        const res = await fetch(`${API_BASE}/sites`);
        allSites = await res.json();
        sites = [...allSites];
        drawSitesOnCanvas();
        updateStats();
    } catch (err) {
        console.error('Failed to load sites:', err);
    }
}

async function loadStats() {
    try {
        const res = await fetch(`${API_BASE}/stats`);
        const stats = await res.json();
        document.getElementById('totalSites').textContent = stats.total_sites;
        document.getElementById('sitesWithData').textContent = stats.sites_with_data;
        document.getElementById('severeCount').textContent = stats.pollution_levels.severe + stats.pollution_levels.high;
        document.getElementById('alertCount').textContent = stats.total_alerts;
    } catch (err) {
        console.error('Failed to load stats:', err);
    }
}

function updateStats() {
    loadStats();
}

async function loadAlerts() {
    try {
        const res = await fetch(`${API_BASE}/alerts?limit=10`);
        const alerts = await res.json();
        const container = document.getElementById('alertList');

        if (alerts.length === 0) {
            container.innerHTML = '<div class="loading">暂无告警</div>';
            return;
        }

        container.innerHTML = alerts.map(a => `
            <div class="alert-item ${a.severity === '严重' ? 'severe' : a.severity === '高' ? 'high' : a.severity === '中' ? 'medium' : 'low'}">
                <div class="alert-type">${a.alert_type} ${a.metal_type ? '- ' + a.metal_type : ''}</div>
                <div class="alert-msg">${a.message.substring(0, 80)}...</div>
            </div>
        `).join('');
    } catch (err) {
        console.error('Failed to load alerts:', err);
    }
}

function showSiteDetail(site) {
    currentSite = site;
    document.getElementById('panelPlaceholder').style.display = 'none';
    document.getElementById('panelContent').style.display = 'flex';

    const pollutionText = getPollutionText(site.pollution_index);
    const pollutionClass = getPollutionClass(site.pollution_index);

    document.getElementById('siteHeader').innerHTML = `
        <div class="site-name">${site.name}</div>
        <div class="site-meta">
            <span class="meta-tag">📍 ${site.country}</span>
            <span class="meta-tag metal">⚒️ ${site.metal_type}冶炼</span>
            <span class="meta-tag scale">📏 ${site.scale}</span>
            <span class="meta-tag">📜 ${site.era}</span>
        </div>
        <div class="site-desc">${site.description || '暂无描述'}</div>
        <div class="pollution-badge ${pollutionClass}">
            ${site.pollution_index > 0 ? `污染指数: ${site.pollution_index.toFixed(4)} · ${pollutionText}` : '暂无监测数据'}
        </div>
    `;

    switchTab('trend');
    loadTrendData(site.id);
}

function closePanel() {
    currentSite = null;
    document.getElementById('panelPlaceholder').style.display = 'flex';
    document.getElementById('panelContent').style.display = 'none';
}

function switchTab(tabName) {
    document.querySelectorAll('.tab').forEach(t => t.classList.toggle('active', t.dataset.tab === tabName));
    document.querySelectorAll('.tab-pane').forEach(p => p.classList.toggle('active', p.id === 'tab-' + tabName));

    if (currentSite) {
        if (tabName === 'fingerprint') loadFingerprintData(currentSite.id);
        if (tabName === 'remediation') loadRemediationData(currentSite.id);
    }
}

async function loadTrendData(siteId) {
    try {
        const res = await fetch(`${API_BASE}/sites/${siteId}/trend`);
        const data = await res.json();

        if (data.length === 0) {
            document.getElementById('tab-trend').innerHTML = '<div class="loading">暂无趋势数据</div>';
            return;
        }

        const latest = data[data.length - 1];
        const metals = ['Pb', 'Zn', 'Cu', 'As', 'Hg', 'Cd'];

        document.getElementById('trendSummary').innerHTML = metals.slice(0, 6).map(m => `
            <div class="summary-card ${m.toLowerCase()}">
                <div class="label">${m} (mg/kg)</div>
                <div class="value">${latest.metals[m] ? latest.metals[m].toFixed(1) : '-'}</div>
            </div>
        `).join('');

        drawTrendChart(data);

        document.getElementById('metalLegend').innerHTML = metals.map(m => `
            <div class="metal-legend-item">
                <span class="metal-legend-color" style="background:${METAL_COLORS[m]}"></span>
                ${m}
            </div>
        `).join('');

        if (!document.getElementById('metalLegend').querySelector('.pi-legend')) {
            const piLegend = document.createElement('div');
            piLegend.className = 'metal-legend-item pi-legend';
            piLegend.innerHTML = '<span class="metal-legend-color" style="background:#ffd700"></span> 污染指数';
            document.getElementById('metalLegend').appendChild(piLegend);
        }

    } catch (err) {
        console.error('Failed to load trend data:', err);
    }
}

function drawTrendChart(data) {
    const canvas = document.getElementById('trendChart');
    const ctx = canvas.getContext('2d');
    const dpr = window.devicePixelRatio || 1;
    const rect = canvas.getBoundingClientRect();
    canvas.width = rect.width * dpr;
    canvas.height = 250 * dpr;
    canvas.style.width = rect.width + 'px';
    canvas.style.height = '250px';
    ctx.scale(dpr, dpr);

    const W = rect.width;
    const H = 250;
    const padding = { top: 20, right: 20, bottom: 40, left: 50 };
    const chartW = W - padding.left - padding.right;
    const chartH = H - padding.top - padding.bottom;

    ctx.fillStyle = '#161b22';
    ctx.fillRect(0, 0, W, H);

    const metals = ['Pb', 'Zn', 'Cu', 'As', 'Hg', 'Cd'];
    const years = data.map(d => d.year);

    let maxVal = 0;
    data.forEach(d => {
        metals.forEach(m => {
            if (d.metals[m]) maxVal = Math.max(maxVal, d.metals[m]);
        });
    });
    if (maxVal === 0) maxVal = 100;
    maxVal = maxVal * 1.15;

    ctx.strokeStyle = '#21262d';
    ctx.lineWidth = 1;
    for (let i = 0; i <= 5; i++) {
        const y = padding.top + (chartH / 5) * i;
        ctx.beginPath();
        ctx.moveTo(padding.left, y);
        ctx.lineTo(W - padding.right, y);
        ctx.stroke();

        ctx.fillStyle = '#8b949e';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'right';
        ctx.textBaseline = 'middle';
        ctx.fillText((maxVal - (maxVal / 5) * i).toFixed(0), padding.left - 8, y);
    }

    ctx.textAlign = 'center';
    ctx.textBaseline = 'top';
    const stepX = chartW / Math.max(1, years.length - 1);
    years.forEach((year, i) => {
        const x = padding.left + stepX * i;
        ctx.fillStyle = '#8b949e';
        ctx.fillText(year, x, H - padding.bottom + 10);
    });

    metals.forEach(metal => {
        ctx.beginPath();
        ctx.strokeStyle = METAL_COLORS[metal];
        ctx.lineWidth = 2;

        let hasData = false;
        data.forEach((d, i) => {
            const val = d.metals[metal];
            if (val && val > 0) {
                const x = padding.left + stepX * i;
                const y = padding.top + chartH - (val / maxVal) * chartH;
                if (!hasData) {
                    ctx.moveTo(x, y);
                    hasData = true;
                } else {
                    ctx.lineTo(x, y);
                }
            }
        });
        ctx.stroke();

        data.forEach((d, i) => {
            const val = d.metals[metal];
            if (val && val > 0) {
                const x = padding.left + stepX * i;
                const y = padding.top + chartH - (val / maxVal) * chartH;
                ctx.beginPath();
                ctx.arc(x, y, 3, 0, Math.PI * 2);
                ctx.fillStyle = METAL_COLORS[metal];
                ctx.fill();
            }
        });
    });

    let maxPI = Math.max(...data.map(d => d.pollution_index));
    if (maxPI === 0) maxPI = 1;
    ctx.beginPath();
    ctx.strokeStyle = '#ffd700';
    ctx.lineWidth = 2;
    ctx.setLineDash([5, 5]);

    data.forEach((d, i) => {
        if (d.pollution_index > 0) {
            const x = padding.left + stepX * i;
            const y = padding.top + chartH - (d.pollution_index / Math.max(maxPI, 3)) * chartH;
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        }
    });
    ctx.stroke();
    ctx.setLineDash([]);
}

async function loadFingerprintData(siteId) {
    const container = document.getElementById('fingerprintContent');
    container.innerHTML = '<div class="loading">正在分析污染指纹...</div>';

    try {
        const res = await fetch(`${API_BASE}/sites/${siteId}/fingerprint`);
        const data = await res.json();

        if (!data.matched_fingerprint) {
            container.innerHTML = '<div class="loading">指纹数据不足，无法匹配</div>';
            return;
        }

        const simClass = data.similarity >= 0.8 ? 'high' : data.similarity >= 0.6 ? 'medium' : 'low';
        const ratios = data.site_ratios || {};

        container.innerHTML = `
            <div class="fp-match">
                <div class="fp-match-header">
                    <div class="fp-name">${data.matched_fingerprint.fingerprint_name}</div>
                    <div class="fp-similarity ${simClass}">相似度: ${(data.similarity * 100).toFixed(1)}%</div>
                </div>
                <div class="fp-info">
                    冶炼类型: <span>${data.matched_fingerprint.metal_type}</span><br>
                    工艺类型: <span>${data.matched_fingerprint.process_type || '未知'}</span><br>
                    特征簇编号: <span>Cluster #${data.matched_fingerprint.cluster_id || data.cluster_id || '-'}</span>
                </div>
                <div class="fp-ratios">
                    <h4>重金属比率特征</h4>
                    <div class="ratio-grid">
                        <div class="ratio-item">
                            <div class="ratio-label">Pb/Zn 比</div>
                            <div class="ratio-value">${ratios.pb_zn_ratio ? ratios.pb_zn_ratio.toFixed(4) : '-'}</div>
                        </div>
                        <div class="ratio-item">
                            <div class="ratio-label">Cu/Pb 比</div>
                            <div class="ratio-value">${ratios.cu_pb_ratio ? ratios.cu_pb_ratio.toFixed(4) : '-'}</div>
                        </div>
                        <div class="ratio-item">
                            <div class="ratio-label">As/Hg 比</div>
                            <div class="ratio-value">${ratios.as_hg_ratio ? ratios.as_hg_ratio.toFixed(4) : '-'}</div>
                        </div>
                        <div class="ratio-item">
                            <div class="ratio-label">Cd/Zn 比</div>
                            <div class="ratio-value">${ratios.cd_zn_ratio ? ratios.cd_zn_ratio.toFixed(4) : '-'}</div>
                        </div>
                        <div class="ratio-item">
                            <div class="ratio-label">Cu/As 比</div>
                            <div class="ratio-value">${ratios.cu_as_ratio ? ratios.cu_as_ratio.toFixed(4) : '-'}</div>
                        </div>
                        <div class="ratio-item">
                            <div class="ratio-label">欧氏距离</div>
                            <div class="ratio-value">${data.distance ? data.distance.toFixed(4) : '-'}</div>
                        </div>
                    </div>
                </div>
                <div class="fp-desc">💡 ${data.matched_fingerprint.description}</div>
            </div>
        `;
    } catch (err) {
        console.error('Failed to load fingerprint data:', err);
        container.innerHTML = '<div class="loading">加载失败</div>';
    }
}

async function loadRemediationData(siteId) {
    const container = document.getElementById('remediationContent');
    container.innerHTML = '<div class="loading">正在评估修复方案...</div>';

    try {
        const res = await fetch(`${API_BASE}/sites/${siteId}/remediation`);
        const data = await res.json();

        if (!data.top_technologies || data.top_technologies.length === 0) {
            container.innerHTML = '<div class="loading">数据不足，无法生成修复方案</div>';
            return;
        }

        const piClass = data.pollution_index >= 2 ? 'pi-high' : data.pollution_index >= 1 ? 'pi-medium' : 'pi-low';

        container.innerHTML = `
            <div class="assess-header">
                <h3>修复评估指标</h3>
                <div class="assess-metrics">
                    <div class="metric">
                        <span class="metric-value ${piClass}">${data.pollution_index.toFixed(3)}</span>
                        <span class="metric-label">综合污染指数</span>
                    </div>
                    <div class="metric">
                        <span class="metric-value ${data.eco_risk_index >= 150 ? 'pi-high' : data.eco_risk_index >= 40 ? 'pi-medium' : 'pi-low'}">${data.eco_risk_index.toFixed(1)}</span>
                        <span class="metric-label">潜在生态风险指数</span>
                    </div>
                    <div class="metric">
                        <span class="metric-value">${data.detected_metals.length}</span>
                        <span class="metric-label">超标重金属种数</span>
                    </div>
                </div>
                <div style="margin-top:10px;font-size:12px;color:#8b949e;">
                    检测到的重金属: ${data.detected_metals.map(m => 
                        `<span style="color:${METAL_COLORS[m]};font-weight:600;">${m}</span>`
                    ).join('、') || '无'}
                </div>
            </div>
            <h3 style="font-size:13px;color:#8b949e;margin-bottom:10px;">🏆 推荐修复技术 (按综合评分排序)</h3>
            <div class="tech-list">
                ${data.top_technologies.map(tech => renderTechCard(tech)).join('')}
            </div>
        `;
    } catch (err) {
        console.error('Failed to load remediation data:', err);
        container.innerHTML = '<div class="loading">加载失败</div>';
    }
}

function renderTechCard(tech) {
    const subScores = tech.sub_scores || {};
    const scoreLabels = {
        metal_coverage: '重金属覆盖度',
        efficiency: '修复效率',
        soil_applicability: '土壤适应性',
        cost: '经济性',
        duration: '修复周期',
        environmental: '环境影响',
        sustainability: '可持续性'
    };

    return `
        <div class="tech-card">
            <div class="tech-header">
                <div class="tech-name">${tech.name}</div>
                <div class="tech-score">${tech.total_score.toFixed(1)}分</div>
            </div>
            <span class="tech-category">${tech.category}</span>
            <div class="tech-desc">${tech.description}</div>
            <div class="tech-meta">
                <div class="tech-meta-item">
                    <span class="tech-meta-label">💰 成本</span>
                    <span class="tech-meta-value">¥${tech.cost_low}~${tech.cost_high}/m³</span>
                </div>
                <div class="tech-meta-item">
                    <span class="tech-meta-label">⏱️ 周期</span>
                    <span class="tech-meta-value">${tech.duration_months_low}~${tech.duration_months_high}月</span>
                </div>
                <div class="tech-meta-item">
                    <span class="tech-meta-label">🎯 效率</span>
                    <span class="tech-meta-value">${tech.remediation_efficiency}%</span>
                </div>
                <div class="tech-meta-item">
                    <span class="tech-meta-label">♻️ 可持续</span>
                    <span class="tech-meta-value">${tech.sustainability_score}/10</span>
                </div>
            </div>
            <div class="tech-subscores">
                ${Object.entries(subScores).map(([key, val]) => `
                    <div class="score-bar-container">
                        <div class="score-bar-label">
                            <span>${scoreLabels[key] || key}</span>
                            <span>${val.toFixed(1)}</span>
                        </div>
                        <div class="score-bar">
                            <div class="score-bar-fill" style="width:${val}%"></div>
                        </div>
                    </div>
                `).join('')}
            </div>
        </div>
    `;
}

function resetMapView() {
    map.setView([25, 15], 2);
}

function refreshData() {
    loadSites();
    loadStats();
    loadAlerts();
    if (currentSite) {
        showSiteDetail(currentSite);
    }
}

document.addEventListener('DOMContentLoaded', () => {
    initMap();
    loadSites();
    loadStats();
    loadAlerts();

    ['showAll', 'filterCu', 'filterFe', 'filterAg', 'filterPb', 'filterHg'].forEach(id => {
        document.getElementById(id).addEventListener('change', drawSitesOnCanvas);
    });

    setInterval(() => {
        loadAlerts();
        loadStats();
    }, 30000);
});
