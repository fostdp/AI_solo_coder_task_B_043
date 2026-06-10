// ============================================
// site_detail.js - 遗址详情面板模块
// 职责：三Tab详情面板（趋势/指纹/修复）+ 纯Canvas趋势图
// 对外API：window.SiteDetail.show(site)
//          window.SiteDetail.setAPIBase(url)
// ============================================

(function (global) {
    'use strict';

    const METALS = ['Pb', 'Zn', 'Cu', 'As', 'Hg', 'Cd'];
    const METAL_COLORS = {
        Pb: '#f44336', Zn: '#2196f3', Cu: '#ff9800',
        As: '#9c27b0', Hg: '#00bcd4', Cd: '#8bc34a'
    };
    const CHART_MARGIN = { top: 30, right: 20, bottom: 40, left: 60 };

    let API_BASE = '/api';
    let currentSite = null;
    let trendData = null;
    let fingerprintResult = null;
    let remediationResult = null;

    // ====== 对外接口 ======
    const SiteDetail = {
        setAPIBase(url) { API_BASE = url; },

        show(site) {
            currentSite = site;
            var header = document.getElementById('siteHeader');
            if (header) {
                var pi = site.pollution_index || 0;
                var cls = piClass(pi);
                var cat = piCategory(pi);
                header.innerHTML = '<h2 class="site-name">' + site.name + '</h2>' +
                    '<div class="site-meta">遗址#' + site.id + ' · ' + site.metal_type + '冶炼 · ' + (site.era || '') + ' · ' + site.country + '</div>' +
                    '<div class="site-pi"><span class="pi-value ' + cls + '">' + pi.toFixed(2) + '</span> <span class="pi-category ' + cls + '">' + cat + '</span></div>';
            }

            document.getElementById('panelPlaceholder').style.display = 'none';
            document.getElementById('panelContent').style.display = '';

            activateTab('trend');
            loadAll(site.id);
        },

        close() {
            document.getElementById('panelContent').style.display = 'none';
            document.getElementById('panelPlaceholder').style.display = '';
            currentSite = null;
            slagData = null;
        },

        getCurrentSite() { return currentSite; }
    };

    let slagData = null;

    async function loadSlagData(siteId) {
        var container = document.getElementById('slagContent');
        if (!container) return;
        if (slagData && slagData.site_id === siteId) return;
        container.innerHTML = '<div style="color:#8b949e;font-size:12px;text-align:center;padding:20px;">加载中...</div>';
        try {
            var data = await global.SlagReuseAdvisor.load(siteId);
            if (data) {
                slagData = data;
                global.SlagReuseAdvisor.render(data, container);
            } else {
                container.innerHTML = '<div style="color:#8b949e;font-size:12px;text-align:center;padding:20px;">暂无矿渣数据</div>';
            }
        } catch (e) {
            container.innerHTML = '<div style="color:#f85149;font-size:12px;text-align:center;padding:20px;">加载失败</div>';
        }
    }

    // ====== Tab 切换 ======
    function activateTab(name) {
        document.querySelectorAll('.tab').forEach(b => b.classList.remove('active'));
        document.querySelectorAll('.tab-pane').forEach(c => c.classList.remove('active'));
        const btn = document.querySelector(`.tab[data-tab="${name}"]`);
        if (btn) btn.classList.add('active');
        const panel = document.getElementById('tab-' + name);
        if (panel) panel.classList.add('active');
        if (name === 'smelting' && currentSite) loadSmeltingData(currentSite.id);
        if (name === 'farmsafety' && currentSite) loadSoilSafetyEvaluatorData(currentSite.id);
        if (name === 'slag' && currentSite) loadSlagData(currentSite.id);
    }

    async function loadSmeltingData(siteId) {
        const container = document.getElementById('smeltingContent');
        if (!container) return;
        container.innerHTML = '<div style="color:#8b949e;text-align:center;padding:20px;">加载中...</div>';
        try {
            const data = await SmeltingProcess.load(siteId);
            SmeltingProcess.render(data, container);
        } catch (e) {
            container.innerHTML = '<div style="color:#f85149;font-size:12px;text-align:center;padding:20px;">冶炼工艺反演数据加载失败</div>';
        }
    }

    async function loadSoilSafetyEvaluatorData(siteId) {
        const container = document.getElementById('farmsafetyContent');
        if (!container) return;
        container.innerHTML = '<div style="color:#8b949e;text-align:center;padding:20px;">加载中...</div>';
        try {
            const data = await SoilSafetyEvaluator.load(siteId);
            SoilSafetyEvaluator.render(data, container);
        } catch (e) {
            container.innerHTML = '<div style="color:#f85149;font-size:12px;text-align:center;padding:20px;">农田安全评估数据加载失败</div>';
        }
    }

    // ====== 加载所有数据 ======
    async function loadAll(siteId) {
        const [trend, fp, rem] = await Promise.all([
            fetch(`${API_BASE}/sites/${siteId}/trend`).then(r => r.json()),
            fetch(`${API_BASE}/sites/${siteId}/fingerprint`).then(r => r.json()),
            fetch(`${API_BASE}/sites/${siteId}/remediation`).then(r => r.json())
        ]).catch(() => [null, null, null]);

        if (trend && trend.data) {
            trendData = trend.data;
            drawTrendChart(trend.data);
            fillMetalTable(trend.data);
        }
        if (fp && fp.data) {
            fingerprintResult = fp.data;
            fillFingerprint(fp.data);
        }
        if (rem && rem.data) {
            remediationResult = rem.data;
            fillRemediation(rem.data);
        }
    }

    // ====== 重金属浓度趋势图（纯Canvas） ======
    function drawTrendChart(data) {
        const c = document.getElementById('trendCanvas');
        if (!c || !data || data.length === 0) return;

        const dpr = window.devicePixelRatio || 1;
        const w = c.clientWidth, h = c.clientHeight;
        c.width = w * dpr; c.height = h * dpr;
        const ctx = c.getContext('2d');
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

        ctx.clearRect(0, 0, w, h);
        const pw = w - CHART_MARGIN.left - CHART_MARGIN.right;
        const ph = h - CHART_MARGIN.top - CHART_MARGIN.bottom;

        let maxVal = 0;
        data.forEach(d => {
            METALS.forEach(m => { if (d[m] > maxVal) maxVal = d[m]; });
            if (d.PollutionIndex * 100 > maxVal) maxVal = d.PollutionIndex * 100;
        });
        maxVal = maxVal * 1.1;

        const xScale = i => CHART_MARGIN.left + i * pw / Math.max(1, data.length - 1);
        const yScale = v => CHART_MARGIN.top + ph - (v / maxVal) * ph;

        drawGrid(ctx, data, xScale, yScale, maxVal, pw, ph);

        METALS.forEach(m => {
            ctx.beginPath();
            ctx.strokeStyle = METAL_COLORS[m];
            ctx.lineWidth = 2;
            data.forEach((d, i) => {
                const x = xScale(i), y = yScale(d[m]);
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            });
            ctx.stroke();
            ctx.fillStyle = METAL_COLORS[m];
            data.forEach((d, i) => {
                ctx.beginPath();
                ctx.arc(xScale(i), yScale(d[m]), 3, 0, Math.PI * 2);
                ctx.fill();
            });
        });

        ctx.beginPath();
        ctx.strokeStyle = '#333';
        ctx.lineWidth = 2.5;
        ctx.setLineDash([5, 4]);
        data.forEach((d, i) => {
            const x = xScale(i), y = yScale(d.PollutionIndex * 100);
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        });
        ctx.stroke();
        ctx.setLineDash([]);

        drawLegend(ctx, w);
    }

    function drawGrid(ctx, data, xScale, yScale, maxVal, pw, ph) {
        ctx.strokeStyle = '#f0f0f0';
        ctx.fillStyle = '#999';
        ctx.lineWidth = 1;
        ctx.font = '10px sans-serif';
        const steps = 5;
        for (let i = 0; i <= steps; i++) {
            const y = yScale(maxVal * i / steps);
            ctx.beginPath();
            ctx.moveTo(CHART_MARGIN.left, y);
            ctx.lineTo(CHART_MARGIN.left + pw, y);
            ctx.stroke();
            ctx.textAlign = 'right';
            ctx.fillText((maxVal * i / steps).toFixed(0), CHART_MARGIN.left - 5, y + 3);
        }
        ctx.textAlign = 'center';
        ctx.fillStyle = '#666';
        data.forEach((d, i) => {
            const x = xScale(i);
            ctx.fillText(d.Year, x, CHART_MARGIN.top + ph + 15);
        });
        ctx.save();
        ctx.translate(12, CHART_MARGIN.top + ph / 2);
        ctx.rotate(-Math.PI / 2);
        ctx.fillText('浓度 (mg/kg)', 0, 0);
        ctx.restore();
    }

    function drawLegend(ctx, w) {
        const items = [...METALS.map(m => ({ name: m, color: METAL_COLORS[m] })),
            { name: 'PI×100(虚线)', color: '#333', dash: true }];
        let x = w - 100, y = 15;
        ctx.font = 'bold 10px sans-serif';
        items.forEach(it => {
            ctx.beginPath();
            ctx.strokeStyle = it.color;
            ctx.fillStyle = it.color;
            ctx.lineWidth = 2;
            if (it.dash) ctx.setLineDash([4, 3]);
            ctx.moveTo(x, y);
            ctx.lineTo(x + 20, y);
            ctx.stroke();
            ctx.setLineDash([]);
            ctx.textAlign = 'left';
            ctx.fillText(it.name, x + 24, y + 3);
            y += 14;
        });
    }

    // ====== 金属浓度表 ======
    function fillMetalTable(data) {
        const tbody = document.getElementById('metalTableBody');
        if (!tbody || !data || !data.length) return;
        tbody.innerHTML = '';
        const latest = data[data.length - 1];
        const prev = data.length >= 2 ? data[data.length - 2] : latest;
        const standards = { Pb: 800, Zn: 5000, Cu: 18000, As: 250, Hg: 38, Cd: 47 };
        METALS.forEach(m => {
            const cur = latest[m] || 0;
            const pr = prev[m] || 0;
            const ch = pr > 0 ? ((cur - pr) / pr * 100) : 0;
            const ratio = standards[m] > 0 ? cur / standards[m] : 0;
            const status = ratio >= 1 ? '严重' : ratio >= 0.6 ? '预警' : '正常';
            const cls = ratio >= 1 ? 'danger' : ratio >= 0.6 ? 'warning' : 'ok';
            const tr = document.createElement('tr');
            tr.innerHTML = `<td style="font-weight:bold;color:${METAL_COLORS[m]}">${m}</td>
                <td>${cur.toFixed(2)}</td>
                <td>${standards[m].toFixed(0)}</td>
                <td>${(ratio * 100).toFixed(1)}%</td>
                <td class="${cls}-text">${status}</td>
                <td>${ch >= 0 ? '+' : ''}${ch.toFixed(1)}%</td>`;
            tbody.appendChild(tr);
        });
    }

    // ====== 指纹识别 ======
    function fillFingerprint(data) {
        if (data && data.best_match) {
            const bm = data.best_match;
            document.getElementById('bestFingerprintName').textContent =
                `${bm.metal_type || ''} · ${bm.process_type || ''}`;
            document.getElementById('bestFingerprintSim').textContent =
                ((bm.similarity || 0) * 100).toFixed(1) + '%';
            document.getElementById('bestFingerprintDesc').textContent =
                bm.description || '无';
        }
        const tbody = document.getElementById('fingerprintTable');
        if (!tbody || !data.matches) return;
        tbody.innerHTML = '';
        data.matches.forEach(m => {
            const tr = document.createElement('tr');
            tr.innerHTML = `<td style="font-weight:bold">${m.metal_type}</td>
                <td>${m.process_type}</td><td>${m.region}</td>
                <td style="color:${similarityColor(m.similarity)};font-weight:bold">${((m.similarity || 0) * 100).toFixed(1)}%</td>
                <td>${(m.distance || 0).toFixed(3)}</td>`;
            tbody.appendChild(tr);
        });
    }

    // ====== 修复技术推荐 ======
    function fillRemediation(data) {
        if (!data) return;
        document.getElementById('ecoRI').textContent =
            (data.eco_risk_index || 0).toFixed(1);
        document.getElementById('mobility').textContent = data.mobility_level || '低';
        const detected = data.detected_metals || [];
        document.getElementById('detectedMetals').textContent =
            detected.length > 0 ? detected.join(', ') : '无';
        const tbody = document.getElementById('remediationTable');
        if (!tbody || !data.recommended_techs) return;
        tbody.innerHTML = '';
        data.recommended_techs.forEach(t => {
            const scoreCls = t.final_score >= 70 ? 'ok' : t.final_score >= 50 ? 'warning' : 'danger';
            const tr = document.createElement('tr');
            tr.innerHTML = `<td style="font-weight:bold">#${t.rank}</td>
                <td style="font-weight:bold">${t.tech_name}</td><td>${t.tech_type || ''}</td>
                <td class="${scoreCls}-text" style="font-weight:bold">${t.final_score.toFixed(1)}</td>
                <td>${(t.efficiency_score || 0).toFixed(0)}</td>
                <td>${(t.cost_score || 0).toFixed(0)}</td>
                <td>${(t.duration_score || 0).toFixed(0)}</td>
                <td>${(t.sustain_score || 0).toFixed(0)}</td>`;
            tbody.appendChild(tr);
        });
        const weights = data.recommended_techs[0]?.weights_used || {};
        fillWeightsChart(weights, data.recommended_techs[0]?.alpha_used);
    }

    function fillWeightsChart(w, alpha) {
        const c = document.getElementById('weightsCanvas');
        if (!c) return;
        const dpr = window.devicePixelRatio || 1;
        const wd = c.clientWidth, hd = c.clientHeight;
        c.width = wd * dpr; c.height = hd * dpr;
        const ctx = c.getContext('2d');
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx.clearRect(0, 0, wd, hd);
        const labels = { metal_coverage: '金属覆盖', efficiency: '效率', soil_applicability: '土壤',
            cost: '成本', duration: '周期', environmental: '环境', sustainability: '可持续' };
        const entries = Object.entries(w || {});
        if (!entries.length) return;
        const max = Math.max(...entries.map(([, v]) => v));
        const barH = 18, gap = 8;
        const left = 70;
        entries.forEach(([k, v], i) => {
            const y = 20 + i * (barH + gap);
            ctx.fillStyle = '#333';
            ctx.font = '11px sans-serif';
            ctx.textAlign = 'right';
            ctx.fillText(labels[k] || k, left - 5, y + barH / 2 + 3);
            ctx.fillStyle = '#eee';
            ctx.fillRect(left, y, wd - left - 40, barH);
            const ratio = max > 0 ? v / max : 0;
            const color = ratio > 0.7 ? '#2196f3' : ratio > 0.4 ? '#4caf50' : '#ff9800';
            ctx.fillStyle = color;
            ctx.fillRect(left, y, (wd - left - 40) * ratio, barH);
            ctx.fillStyle = '#fff';
            ctx.textAlign = 'right';
            ctx.font = 'bold 10px sans-serif';
            ctx.fillText((v * 100).toFixed(1) + '%', left + (wd - left - 40) - 3, y + barH / 2 + 3);
        });
        if (alpha !== undefined) {
            ctx.fillStyle = '#666';
            ctx.font = '10px sans-serif';
            ctx.textAlign = 'left';
            ctx.fillText(`α(主观权重占比) = ${alpha.toFixed(2)}`, 10, hd - 8);
        }
    }

    // ====== 工具函数 ======
    function piClass(pi) {
        if (pi < 1) return 'ok';
        if (pi < 2) return 'mild';
        if (pi < 3) return 'warn';
        if (pi < 5) return 'danger';
        return 'critical';
    }
    function piCategory(pi) {
        if (pi < 1) return '清洁';
        if (pi < 2) return '轻度污染';
        if (pi < 3) return '中度污染';
        if (pi < 5) return '重度污染';
        return '严重污染';
    }
    function similarityColor(s) {
        s = s || 0;
        if (s >= 0.8) return '#4caf50';
        if (s >= 0.5) return '#ff9800';
        return '#f44336';
    }

    // ====== Tab事件绑定（页面加载后） ======
    function bindTabEvents() {
        document.querySelectorAll('.tab').forEach(btn => {
            btn.addEventListener('click', () => activateTab(btn.dataset.tab));
        });
        const close = document.getElementById('closeDetail');
        if (close) close.addEventListener('click', () => SiteDetail.close());
    }
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', bindTabEvents);
    } else {
        bindTabEvents();
    }

    global.SiteDetail = SiteDetail;
    global._siteDetailSwitchTab = function (name) { activateTab(name); };

})(typeof window !== 'undefined' ? window : this);
