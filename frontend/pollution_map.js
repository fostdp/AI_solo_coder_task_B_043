// ============================================
// pollution_map.js - 全球遗址污染地图模块
// 职责：Leaflet地图初始化 + Canvas遗址绘制 + 网格聚合 + 事件交互
// 对外API：window.PollutionMap.init(), window.PollutionMap.render()
//          window.PollutionMap.onSiteClick(callback)
// ============================================

(function (global) {
    'use strict';

    // ====== 常量配置 ======
    const SCALE_RADIUS = { small: 6, medium: 10, large: 15, mega: 22 };

    const METAL_ICONS = {
        '铜': 'Cu', '铁': 'Fe', '银': 'Ag', '铅': 'Pb', '汞': 'Hg',
        '青铜': 'Br', '混合': 'Mx'
    };

    const METAL_COLORS = {
        '铜': '#c87533', '铁': '#7a7a7a', '银': '#d4af37',
        '铅': '#5a4a6a', '汞': '#7a8a8a', '青铜': '#cd7f32', '混合': '#556b7b'
    };

    let map, canvas, canvasCtx;
    let sites = [];
    let clusters = [];
    let isClusteringEnabled = false;
    let siteClickCallback = null;
    let activeFilters = null;

    // ====== 对外接口 ======
    const PollutionMap = {
        init(config) {
            map = L.map(config.mapContainerId, {
                zoomControl: true, worldCopyJump: true,
                preferCanvas: true, scrollWheelZoom: true
            }).setView([25, 15], 3);

            L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
                maxZoom: 19, attribution: '© OSM contributors'
            }).addTo(map);

            canvas = L.DomUtil.create('canvas', 'sites-canvas');
            canvas.style.position = 'absolute';
            canvas.style.top = '0'; canvas.style.left = '0';
            canvas.style.pointerEvents = 'auto';
            canvas.style.zIndex = 500;
            map.getPanes().overlayPane.appendChild(canvas);

            canvasCtx = canvas.getContext('2d');

            if (config.sites) sites = config.sites;
            if (config.onClick) siteClickCallback = config.onClick;
            if (config.filters) activeFilters = config.filters;

            map.on('move', () => draw());
            map.on('zoom', () => draw());
            map.on('resize', () => resizeCanvas());

            canvas.addEventListener('click', handleClick);
            canvas.addEventListener('mousemove', handleHover);

            resizeCanvas();
            draw();
        },

        updateSites(newSites) {
            sites = newSites;
            draw();
        },

        updateFilters(filters) {
            activeFilters = filters;
            draw();
        },

        onSiteClick(cb) { siteClickCallback = cb; },

        getMap() { return map; },
        getSites() { return sites; }
    };

    // ====== 核心绘制 ======
    function resizeCanvas() {
        const size = map.getSize();
        const dpr = window.devicePixelRatio || 1;
        canvas.width = size.x * dpr;
        canvas.height = size.y * dpr;
        canvas.style.width = size.x + 'px';
        canvas.style.height = size.y + 'px';
        canvasCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
    }

    function draw() {
        if (!canvasCtx) return;
        resizeCanvas();
        canvasCtx.clearRect(0, 0, canvas.width, canvas.height);

        const filters = activeFilters && typeof activeFilters === 'function'
            ? activeFilters()
            : { metals: {}, scales: {}, pollutions: {} };

        const zoom = map.getZoom();
        const filtered = sites.filter(s => shouldShow(s, filters));

        isClusteringEnabled = zoom <= 5;

        if (isClusteringEnabled) {
            clusters = buildClusters(filtered, getGridSize(zoom));
            clusters.forEach(drawCluster);
        } else {
            clusters = [];
            filtered.forEach(site => {
                const p = map.latLngToContainerPoint([site.latitude, site.longitude]);
                if (p) {
                    site._canvasX = p.x;
                    site._canvasY = p.y;
                    drawSiteMarker(site, zoom);
                }
            });
        }
    }

    // ====== 网格聚合算法 ======
    function getGridSize(zoom) {
        if (zoom <= 3) return 120;
        if (zoom <= 4) return 100;
        if (zoom <= 5) return 80;
        return 60;
    }

    function buildClusters(filteredSites, gridSize) {
        const mapClusters = new Map();
        const result = [];

        filteredSites.forEach(site => {
            const p = map.latLngToContainerPoint([site.latitude, site.longitude]);
            if (!p) return;
            if (p.x < -100 || p.x > canvas.width + 100 ||
                p.y < -100 || p.y > canvas.height + 100) return;

            const gx = Math.floor(p.x / gridSize);
            const gy = Math.floor(p.y / gridSize);
            const key = gx + ',' + gy;

            if (!mapClusters.has(key)) {
                const c = { x: 0, y: 0, sites: [], maxPollutionIndex: 0, count: 0 };
                mapClusters.set(key, c);
                result.push(c);
            }
            const c = mapClusters.get(key);
            c.sites.push(site);
            c.x += p.x; c.y += p.y; c.count++;
            if (site.pollution_index > c.maxPollutionIndex) {
                c.maxPollutionIndex = site.pollution_index;
            }
            site._canvasX = p.x;
            site._canvasY = p.y;
        });

        result.forEach(c => {
            c.x /= c.count; c.y /= c.count;
            c.radius = getClusterRadius(c.count);
        });

        return result;
    }

    function getClusterRadius(count) {
        if (count <= 2) return 18;
        if (count <= 5) return 24;
        if (count <= 10) return 30;
        if (count <= 20) return 36;
        return 42;
    }

    // ====== 绘制 ======
    function drawCluster(cluster) {
        const c = getPollutionColor(cluster.maxPollutionIndex);
        const r = cluster.radius;
        const x = cluster.x, y = cluster.y;

        canvasCtx.beginPath();
        canvasCtx.arc(x + 2, y + 2, r, 0, Math.PI * 2);
        canvasCtx.fillStyle = 'rgba(0,0,0,0.3)';
        canvasCtx.fill();

        const g = canvasCtx.createRadialGradient(x, y, 0, x, y, r);
        g.addColorStop(0, c); g.addColorStop(0.6, c);
        g.addColorStop(1, adjustColor(c, -40));
        canvasCtx.beginPath();
        canvasCtx.arc(x, y, r, 0, Math.PI * 2);
        canvasCtx.fillStyle = g;
        canvasCtx.fill();

        canvasCtx.strokeStyle = 'rgba(255,255,255,0.9)';
        canvasCtx.lineWidth = 2;
        canvasCtx.stroke();

        canvasCtx.fillStyle = '#fff';
        canvasCtx.font = 'bold 13px sans-serif';
        canvasCtx.textAlign = 'center';
        canvasCtx.textBaseline = 'middle';
        canvasCtx.fillText(String(cluster.count), x, y - 1);

        canvasCtx.fillStyle = 'rgba(255,255,255,0.8)';
        canvasCtx.font = '9px sans-serif';
        canvasCtx.fillText('处遗址', x, y + 12);

        cluster._canvasX = x;
        cluster._canvasY = y;
        cluster._canvasRadius = r;
    }

    function drawSiteMarker(site, zoom) {
        const p = { x: site._canvasX, y: site._canvasY };
        if (!p.x) return;
        if (p.x < -50 || p.x > canvas.width + 50 || p.y < -50 || p.y > canvas.height + 50) return;

        const baseR = SCALE_RADIUS[site.scale] || 8;
        const factor = Math.max(0.5, Math.min(1.5, (zoom - 2) / 4 + 0.5));
        const r = baseR * factor;
        const c = getPollutionColor(site.pollution_index);

        canvasCtx.beginPath();
        canvasCtx.arc(p.x + 2, p.y + 2, r, 0, Math.PI * 2);
        canvasCtx.fillStyle = 'rgba(0,0,0,0.3)';
        canvasCtx.fill();

        const g = canvasCtx.createRadialGradient(p.x, p.y, 0, p.x, p.y, r);
        g.addColorStop(0, c); g.addColorStop(0.7, c);
        g.addColorStop(1, adjustColor(c, -30));
        canvasCtx.beginPath();
        canvasCtx.arc(p.x, p.y, r, 0, Math.PI * 2);
        canvasCtx.fillStyle = g;
        canvasCtx.fill();

        canvasCtx.strokeStyle = 'rgba(255,255,255,0.8)';
        canvasCtx.lineWidth = 1.5;
        canvasCtx.stroke();

        if (r >= 10) {
            canvasCtx.fillStyle = 'rgba(255,255,255,0.9)';
            canvasCtx.font = `bold ${Math.max(8, r * 0.5)}px sans-serif`;
            canvasCtx.textAlign = 'center';
            canvasCtx.textBaseline = 'middle';
            canvasCtx.fillText(getMetalIcon(site.metal_type), p.x, p.y);
        }
        site._canvasRadius = r;
    }

    // ====== 点击 & 悬停 ======
    function handleClick(e) {
        const r = canvas.getBoundingClientRect();
        const x = e.clientX - r.left;
        const y = e.clientY - r.top;

        if (isClusteringEnabled && clusters.length > 0) {
            let target = null, minD = Infinity;
            clusters.forEach(c => {
                if (c._canvasX === undefined) return;
                const d = Math.hypot(x - c._canvasX, y - c._canvasY);
                if (d <= c._canvasRadius + 5 && d < minD) {
                    minD = d; target = c;
                }
            });
            if (target) {
                if (target.sites.length === 1) {
                    if (siteClickCallback) siteClickCallback(target.sites[0]);
                } else {
                    const lats = target.sites.reduce((s, x) => s + x.latitude, 0) / target.count;
                    const lngs = target.sites.reduce((s, x) => s + x.longitude, 0) / target.count;
                    map.setView([lats, lngs], Math.min(map.getZoom() + 2, 18));
                }
                return;
            }
        }

        let target = null, minD = Infinity;
        sites.forEach(s => {
            if (s._canvasX === undefined) return;
            const d = Math.hypot(x - s._canvasX, y - s._canvasY);
            if (d <= (s._canvasRadius || 10) + 5 && d < minD) {
                minD = d; target = s;
            }
        });
        if (target && siteClickCallback) siteClickCallback(target);
    }

    function handleHover(e) {
        const r = canvas.getBoundingClientRect();
        const x = e.clientX - r.left;
        const y = e.clientY - r.top;
        let hover = false;

        if (isClusteringEnabled) {
            clusters.forEach(c => {
                if (c._canvasX === undefined) return;
                if (Math.hypot(x - c._canvasX, y - c._canvasY) <= c._canvasRadius + 5) {
                    hover = true;
                }
            });
        } else {
            sites.forEach(s => {
                if (s._canvasX === undefined) return;
                if (Math.hypot(x - s._canvasX, y - s._canvasY) <= (s._canvasRadius || 10) + 5) {
                    hover = true;
                }
            });
        }
        canvas.style.cursor = hover ? 'pointer' : 'default';
    }

    // ====== 工具函数 ======
    function shouldShow(site, filters) {
        if (!filters) return true;
        if (filters.metals && Object.keys(filters.metals).length > 0 &&
            filters.metals[site.metal_type] === false) return false;
        if (filters.scales && Object.keys(filters.scales).length > 0 &&
            filters.scales[site.scale] === false) return false;
        if (filters.pollutions) {
            const p = site.pollution_index;
            let category;
            if (p < 1) category = '清洁';
            else if (p < 2) category = '轻度';
            else if (p < 3) category = '中度';
            else if (p < 5) category = '重度';
            else category = '严重';
            const anyEnabled = Object.values(filters.pollutions).some(v => v === true);
            if (anyEnabled && !filters.pollutions[category]) return false;
        }
        return true;
    }

    function getPollutionColor(pi) {
        if (pi < 1) return '#4caf50';
        if (pi < 2) return '#8bc34a';
        if (pi < 3) return '#ffc107';
        if (pi < 5) return '#ff9800';
        return '#f44336';
    }

    function getMetalIcon(type) {
        return METAL_ICONS[type] || (type ? type[0] : '?');
    }

    function adjustColor(hex, amount) {
        const c = hex.replace('#', '');
        let r = parseInt(c.substring(0, 2), 16) + amount;
        let g = parseInt(c.substring(2, 4), 16) + amount;
        let b = parseInt(c.substring(4, 6), 16) + amount;
        r = Math.max(0, Math.min(255, r));
        g = Math.max(0, Math.min(255, g));
        b = Math.max(0, Math.min(255, b));
        return '#' + [r, g, b].map(x => x.toString(16).padStart(2, '0')).join('');
    }

    // 兼容旧代码全局变量
    global.PollutionMap = PollutionMap;
    global._mapLegacy = {
        SCALE_RADIUS, METAL_ICONS, METAL_COLORS,
        getPollutionColor, getMetalIcon, adjustColor
    };

})(typeof window !== 'undefined' ? window : this);
