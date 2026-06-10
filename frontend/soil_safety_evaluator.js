(function (global) {
    'use strict';

    const API_BASE = '/api';
    const METALS = ['Pb', 'Zn', 'Cu', 'As', 'Hg', 'Cd', 'Cr', 'Ni'];

    const RISK_MAP = {
        '低风险': { label: '低', cls: 'green' },
        '中风险': { label: '中', cls: 'yellow' },
        '中等风险': { label: '中', cls: 'yellow' },
        '较高风险': { label: '较高', cls: 'orange' },
        '高风险': { label: '高', cls: 'red' },
        '极高风险': { label: '高', cls: 'red' }
    };

    const RISK_THEME = {
        green:  { bg: 'rgba(63,185,80,0.15)',  text: '#3fb950', border: '#3fb950' },
        yellow: { bg: 'rgba(210,153,34,0.15)', text: '#d29922', border: '#d29922' },
        orange: { bg: 'rgba(255,152,0,0.15)',  text: '#ff9800', border: '#ff9800' },
        red:    { bg: 'rgba(248,81,73,0.15)',  text: '#f85149', border: '#f85149' }
    };

    const CROP_MAP = {
        '旱地': '小麦/玉米', '水田': '水稻', '菜地': '蔬菜',
        '果园': '果树', '林地': '林木', '草地': '牧草'
    };

    function getIgeoColor(v) {
        if (v <= 0) return '#4caf50';
        if (v <= 1) return '#8bc34a';
        if (v <= 2) return '#ffc107';
        if (v <= 3) return '#ff9800';
        if (v <= 4) return '#f44336';
        if (v <= 5) return '#d32f2f';
        return '#b71c1c';
    }

    function getIgeoTextColor(v) {
        if (v <= 1) return '#111';
        return '#fff';
    }

    function riskInfo(level) {
        return RISK_MAP[level] || { label: level, cls: 'yellow' };
    }

    function setupCanvas(canvas, w, h) {
        var dpr = window.devicePixelRatio || 1;
        canvas.width = w * dpr;
        canvas.height = h * dpr;
        canvas.style.width = w + 'px';
        canvas.style.height = h + 'px';
        var ctx = canvas.getContext('2d');
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        return ctx;
    }

    function el(tag, cls, html) {
        var e = document.createElement(tag);
        if (cls) e.className = cls;
        if (html !== undefined) e.innerHTML = html;
        return e;
    }

    var stylesInjected = false;
    function injectStyles() {
        if (stylesInjected) return;
        stylesInjected = true;
        var s = document.createElement('style');
        s.textContent = [
            '.fs-section{margin-bottom:24px}',
            '.fs-title{font-size:15px;font-weight:600;color:#e6edf3;margin-bottom:12px;padding-left:10px;border-left:3px solid #58a6ff}',
            '.fs-overview{background:#161b22;border-radius:8px;padding:20px;display:flex;align-items:center;gap:24px;flex-wrap:wrap}',
            '.fs-badge{font-size:28px;font-weight:800;padding:10px 28px;border-radius:8px;letter-spacing:2px}',
            '.fs-metrics{display:flex;gap:20px;flex-wrap:wrap}',
            '.fs-metric{background:#0d1117;border-radius:6px;padding:12px 18px;text-align:center;min-width:100px}',
            '.fs-metric-label{font-size:11px;color:#8b949e;display:block;margin-bottom:4px}',
            '.fs-metric-value{font-size:22px;font-weight:700;color:#58a6ff}',
            '.fs-summary{width:100%;font-size:13px;color:#8b949e;line-height:1.6;margin-top:8px;padding-top:8px;border-top:1px solid #21262d}',
            '.fs-canvas-wrap{background:#161b22;border-radius:8px;padding:16px;overflow-x:auto}',
            '.fs-canvas-wrap canvas{display:block}',
            '.fs-crop-list{display:flex;flex-direction:column;gap:12px}',
            '.fs-crop-card{background:#161b22;border-radius:8px;padding:16px;border:1px solid #21262d}',
            '.fs-crop-header{display:flex;align-items:center;gap:10px;margin-bottom:10px;flex-wrap:wrap}',
            '.fs-crop-name{font-size:14px;font-weight:600;color:#e6edf3}',
            '.fs-crop-risk{font-size:12px;font-weight:700;padding:3px 10px;border-radius:10px}',
            '.fs-crop-land{font-size:12px;color:#8b949e}',
            '.fs-exceed-metals{margin-bottom:8px}',
            '.fs-exceed-tag{display:inline-block;font-size:11px;font-weight:700;padding:2px 8px;border-radius:4px;margin-right:4px;margin-bottom:2px;background:rgba(248,81,73,0.2);color:#f85149}',
            '.fs-rec-item{font-size:12px;color:#8b949e;line-height:1.6;padding-left:12px;position:relative}',
            '.fs-rec-item::before{content:"";position:absolute;left:0;top:7px;width:6px;height:6px;border-radius:50%}',
            '.fs-rec-item.rec-ok::before{background:#3fb950}',
            '.fs-rec-item.rec-warn::before{background:#d29922}',
            '.fs-rec-item.rec-ban::before{background:#f85149}',
            '.fs-bcf-row{display:flex;align-items:center;gap:6px;margin-bottom:4px;font-size:11px}',
            '.fs-bcf-metal{width:24px;font-weight:600;text-align:right}',
            '.fs-bcf-track{flex:1;height:12px;background:#21262d;border-radius:3px;position:relative;overflow:hidden}',
            '.fs-bcf-fill{height:100%;border-radius:3px;transition:width .3s}',
            '.fs-bcf-limit{position:absolute;top:0;bottom:0;width:2px;background:#fff;z-index:1}',
            '.fs-bcf-val{width:60px;color:#8b949e;text-align:right}',
            '.fs-table-toggle{background:#161b22;border:1px solid #30363d;color:#8b949e;padding:8px 16px;border-radius:6px;cursor:pointer;font-size:13px;width:100%;text-align:left;margin-bottom:8px}',
            '.fs-table-toggle:hover{border-color:#58a6ff;color:#e6edf3}',
            '.fs-table-wrap{overflow-x:auto;max-height:0;overflow:hidden;transition:max-height .3s ease}',
            '.fs-table-wrap.open{max-height:2000px}',
            '.fs-table{width:100%;border-collapse:collapse;font-size:12px}',
            '.fs-table th{background:#21262d;color:#8b949e;padding:8px 10px;text-align:center;font-weight:600;position:sticky;top:0}',
            '.fs-table td{padding:6px 10px;text-align:center;color:#c9d1d9;border-bottom:1px solid #21262d}',
            '.fs-table tr:hover td{background:#161b22}',
            '.fs-val-danger{color:#f85149;font-weight:700}'
        ].join('\n');
        document.head.appendChild(s);
    }

    function renderRiskOverview(data, container) {
        var sec = el('div', 'fs-section');
        sec.appendChild(el('div', 'fs-title', '综合风险概览'));
        var wrap = el('div', 'fs-overview');
        var ri = riskInfo(data.overall_risk_level);
        var theme = RISK_THEME[ri.cls] || RISK_THEME.yellow;
        var badge = el('div', 'fs-badge');
        badge.textContent = ri.label + '风险';
        badge.style.background = theme.bg;
        badge.style.color = theme.text;
        badge.style.border = '2px solid ' + theme.border;
        wrap.appendChild(badge);
        var metrics = el('div', 'fs-metrics');
        [['最大Igeo', data.max_igeo, 4], ['最大Eri', data.max_eri, 2], ['总RI', data.total_ri, 2]].forEach(function (m) {
            var card = el('div', 'fs-metric');
            card.appendChild(el('span', 'fs-metric-label', m[0]));
            var v = el('span', 'fs-metric-value');
            v.textContent = typeof m[1] === 'number' ? m[1].toFixed(m[2]) : '--';
            card.appendChild(v);
            metrics.appendChild(card);
        });
        wrap.appendChild(metrics);
        if (data.summary) {
            wrap.appendChild(el('div', 'fs-summary', data.summary));
        }
        sec.appendChild(wrap);
        container.appendChild(sec);
    }

    function renderIgeoHeatmap(data, container) {
        var sec = el('div', 'fs-section');
        sec.appendChild(el('div', 'fs-title', '地积累指数(Igeo)热力图'));
        var cw = el('div', 'fs-canvas-wrap');
        var samples = data.sample_results || [];
        if (!samples.length) { cw.appendChild(el('div', '', '暂无数据')); sec.appendChild(cw); container.appendChild(sec); return; }

        var cellW = 60, cellH = 36, leftM = 90, topM = 28;
        var w = leftM + METALS.length * cellW + 10;
        var h = topM + samples.length * cellH + 10;
        var canvas = el('canvas');
        var ctx = setupCanvas(canvas, w, h);

        ctx.fillStyle = '#0d1117';
        ctx.fillRect(0, 0, w, h);
        ctx.font = 'bold 11px sans-serif';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'bottom';
        ctx.fillStyle = '#8b949e';
        METALS.forEach(function (m, i) {
            ctx.fillText(m, leftM + i * cellW + cellW / 2, topM - 4);
        });

        ctx.textAlign = 'right';
        ctx.textBaseline = 'middle';
        samples.forEach(function (sr, row) {
            ctx.fillStyle = '#8b949e';
            ctx.font = '11px sans-serif';
            ctx.fillText(sr.sample_name, leftM - 6, topM + row * cellH + cellH / 2);
            var lookup = {};
            sr.metal_results.forEach(function (mr) { lookup[mr.metal] = mr.igeo; });
            METALS.forEach(function (m, col) {
                var v = lookup[m];
                if (v === undefined || v === null) v = -Infinity;
                var x = leftM + col * cellW;
                var y = topM + row * cellH;
                ctx.fillStyle = getIgeoColor(v);
                ctx.fillRect(x + 1, y + 1, cellW - 2, cellH - 2);
                ctx.fillStyle = getIgeoTextColor(v);
                ctx.font = 'bold 10px sans-serif';
                ctx.textAlign = 'center';
                ctx.textBaseline = 'middle';
                var txt = isFinite(v) ? v.toFixed(2) : '--';
                ctx.fillText(txt, x + cellW / 2, y + cellH / 2);
                ctx.textAlign = 'right';
            });
        });

        cw.appendChild(canvas);
        sec.appendChild(cw);
        container.appendChild(sec);
    }

    function renderDistanceDecay(data, container) {
        var sec = el('div', 'fs-section');
        sec.appendChild(el('div', 'fs-title', '距离衰减曲线'));
        var cw = el('div', 'fs-canvas-wrap');
        var dd = data.distance_decay || [];
        if (!dd.length) { cw.appendChild(el('div', '', '暂无数据')); sec.appendChild(cw); container.appendChild(sec); return; }

        var margin = { top: 30, right: 60, bottom: 40, left: 60 };
        var chartW = 560, chartH = 300;
        var w = chartW, h = chartH;
        var canvas = el('canvas');
        var ctx = setupCanvas(canvas, w, h);

        ctx.fillStyle = '#0d1117';
        ctx.fillRect(0, 0, w, h);

        var pw = w - margin.left - margin.right;
        var ph = h - margin.top - margin.bottom;

        var distCenters = { '≤500米': 250, '500-1000米': 750, '1000-2000米': 1500, '≥2000米': 3500 };
        var points = dd.map(function (d) {
            return { label: d.distance_label, dist: distCenters[d.distance_label] || 0, avgIgeo: d.avg_igeo, avgRI: d.avg_ri };
        });
        points.sort(function (a, b) { return a.dist - b.dist; });

        var maxIgeo = Math.max(1, Math.max.apply(null, points.map(function (p) { return p.avgIgeo; }))) * 1.2;
        var maxRI = Math.max(150, Math.max.apply(null, points.map(function (p) { return p.avgRI; }))) * 1.2;
        var maxDist = 5000;

        function xScale(d) { return margin.left + (d / maxDist) * pw; }
        function yIgeo(v) { return margin.top + ph - (v / maxIgeo) * ph; }
        function yRI(v) { return margin.top + ph - (v / maxRI) * ph; }

        ctx.strokeStyle = '#21262d';
        ctx.lineWidth = 1;
        for (var i = 0; i <= 5; i++) {
            var gy = margin.top + ph * i / 5;
            ctx.beginPath(); ctx.moveTo(margin.left, gy); ctx.lineTo(margin.left + pw, gy); ctx.stroke();
        }

        ctx.setLineDash([4, 4]);
        ctx.strokeStyle = '#3fb950';
        ctx.lineWidth = 1;
        ctx.beginPath(); ctx.moveTo(margin.left, yIgeo(1)); ctx.lineTo(margin.left + pw, yIgeo(1)); ctx.stroke();
        ctx.strokeStyle = '#f85149';
        ctx.beginPath(); ctx.moveTo(margin.left, yRI(150)); ctx.lineTo(margin.left + pw, yRI(150)); ctx.stroke();
        ctx.setLineDash([]);

        ctx.font = '9px sans-serif';
        ctx.fillStyle = '#3fb950';
        ctx.textAlign = 'left';
        ctx.fillText('Igeo=1', margin.left + pw + 3, yIgeo(1) + 3);
        ctx.fillStyle = '#f85149';
        ctx.fillText('RI=150', margin.left + pw + 3, yRI(150) + 3);

        ctx.strokeStyle = '#58a6ff';
        ctx.lineWidth = 2.5;
        ctx.beginPath();
        points.forEach(function (p, idx) {
            var x = xScale(p.dist), y = yIgeo(p.avgIgeo);
            if (idx === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
        });
        ctx.stroke();

        ctx.strokeStyle = '#f778ba';
        ctx.lineWidth = 2.5;
        ctx.setLineDash([6, 4]);
        ctx.beginPath();
        points.forEach(function (p, idx) {
            var x = xScale(p.dist), y = yRI(p.avgRI);
            if (idx === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
        });
        ctx.stroke();
        ctx.setLineDash([]);

        ctx.fillStyle = '#58a6ff';
        points.forEach(function (p) {
            ctx.beginPath(); ctx.arc(xScale(p.dist), yIgeo(p.avgIgeo), 4, 0, Math.PI * 2); ctx.fill();
        });
        ctx.fillStyle = '#f778ba';
        points.forEach(function (p) {
            ctx.beginPath(); ctx.arc(xScale(p.dist), yRI(p.avgRI), 4, 0, Math.PI * 2); ctx.fill();
        });

        ctx.font = '11px sans-serif';
        ctx.fillStyle = '#8b949e';
        ctx.textAlign = 'center';
        points.forEach(function (p) {
            ctx.fillText(p.label, xScale(p.dist), margin.top + ph + 14);
        });
        ctx.fillText('距离(米)', margin.left + pw / 2, h - 4);

        ctx.save();
        ctx.translate(14, margin.top + ph / 2);
        ctx.rotate(-Math.PI / 2);
        ctx.fillStyle = '#58a6ff';
        ctx.textAlign = 'center';
        ctx.fillText('平均Igeo', 0, 0);
        ctx.restore();

        ctx.save();
        ctx.translate(w - 6, margin.top + ph / 2);
        ctx.rotate(Math.PI / 2);
        ctx.fillStyle = '#f778ba';
        ctx.textAlign = 'center';
        ctx.fillText('平均RI', 0, 0);
        ctx.restore();

        ctx.font = '10px sans-serif';
        ctx.textAlign = 'right';
        ctx.fillStyle = '#58a6ff';
        for (var i = 0; i <= 5; i++) {
            var val = maxIgeo * (5 - i) / 5;
            ctx.fillText(val.toFixed(1), margin.left - 5, yIgeo(val) + 3);
        }
        ctx.fillStyle = '#f778ba';
        ctx.textAlign = 'left';
        for (var i = 0; i <= 5; i++) {
            var val = maxRI * (5 - i) / 5;
            ctx.fillText(val.toFixed(0), margin.left + pw + 5, yRI(val) + 3);
        }

        var legX = margin.left + 10, legY = margin.top + 8;
        ctx.font = 'bold 10px sans-serif';
        ctx.textAlign = 'left';
        ctx.strokeStyle = '#58a6ff'; ctx.lineWidth = 2; ctx.setLineDash([]);
        ctx.beginPath(); ctx.moveTo(legX, legY); ctx.lineTo(legX + 18, legY); ctx.stroke();
        ctx.fillStyle = '#58a6ff'; ctx.fillText('平均Igeo(实线)', legX + 22, legY + 3);
        ctx.strokeStyle = '#f778ba'; ctx.setLineDash([6, 4]);
        ctx.beginPath(); ctx.moveTo(legX, legY + 14); ctx.lineTo(legX + 18, legY + 14); ctx.stroke();
        ctx.setLineDash([]);
        ctx.fillStyle = '#f778ba'; ctx.fillText('平均RI(虚线)', legX + 22, legY + 17);

        cw.appendChild(canvas);
        sec.appendChild(cw);
        container.appendChild(sec);
    }

    function renderCropRecommendations(data, container) {
        var sec = el('div', 'fs-section');
        sec.appendChild(el('div', 'fs-title', '农作物种植建议'));
        var list = el('div', 'fs-crop-list');
        var crops = data.crop_recommendations || [];
        var samples = data.sample_results || [];
        if (!crops.length) { list.appendChild(el('div', '', '暂无数据')); sec.appendChild(list); container.appendChild(sec); return; }

        crops.forEach(function (cr, idx) {
            var card = el('div', 'fs-crop-card');
            var header = el('div', 'fs-crop-header');
            var sName = samples[idx] ? samples[idx].sample_name : ('样点' + (idx + 1));
            header.appendChild(el('span', 'fs-crop-name', sName));
            header.appendChild(el('span', 'fs-crop-land', (cr.land_use_type || '--') + ' · ' + (CROP_MAP[cr.land_use_type] || '通用作物')));
            var ri = riskInfo(cr.risk_level);
            var theme = RISK_THEME[ri.cls] || RISK_THEME.yellow;
            var badge = el('span', 'fs-crop-risk');
            badge.textContent = ri.label + '风险';
            badge.style.background = theme.bg;
            badge.style.color = theme.text;
            header.appendChild(badge);
            card.appendChild(header);

            var exceedPreds = (cr.predictions || []).filter(function (p) { return p.is_exceed; });
            if (exceedPreds.length > 0) {
                var exDiv = el('div', 'fs-exceed-metals');
                exceedPreds.forEach(function (p) {
                    exDiv.appendChild(el('span', 'fs-exceed-tag', p.metal + ' 超标(' + (p.exceed_ratio * 100).toFixed(0) + '%)'));
                });
                card.appendChild(exDiv);
            }

            var recCls = cr.risk_level === '高风险' || cr.risk_level === '极高风险' ? 'rec-ban' :
                         cr.risk_level === '中等风险' ? 'rec-warn' : 'rec-ok';
            (cr.recommendations || []).forEach(function (r) {
                card.appendChild(el('div', 'fs-rec-item ' + recCls, r));
            });

            var bcfTitle = el('div', '', '');
            bcfTitle.style.cssText = 'font-size:11px;color:#8b949e;margin:10px 0 6px;border-top:1px solid #21262d;padding-top:8px';
            bcfTitle.textContent = 'BCF预测: 作物浓度 vs 食品标准';
            card.appendChild(bcfTitle);

            (cr.predictions || []).forEach(function (p) {
                var row = el('div', 'fs-bcf-row');
                var metalSpan = el('span', 'fs-bcf-metal');
                metalSpan.textContent = p.metal;
                if (p.is_exceed) metalSpan.style.color = '#f85149';
                else if (p.is_close) metalSpan.style.color = '#d29922';
                row.appendChild(metalSpan);

                var track = el('div', 'fs-bcf-track');
                var maxRatio = 2.0;
                var ratio = p.food_limit > 0 ? p.predicted_crop_conc / p.food_limit : 0;
                var pct = Math.min(ratio / maxRatio, 1) * 100;
                var fill = el('div', 'fs-bcf-fill');
                fill.style.width = pct + '%';
                fill.style.background = p.is_exceed ? '#f85149' : p.is_close ? '#d29922' : '#3fb950';
                track.appendChild(fill);

                var limitLine = el('div', 'fs-bcf-limit');
                limitLine.style.left = (50 / maxRatio * 100) + '%';
                track.appendChild(limitLine);
                row.appendChild(track);

                var valSpan = el('span', 'fs-bcf-val');
                valSpan.textContent = (p.predicted_crop_conc * 1000).toFixed(2) + '/' + (p.food_limit * 1000).toFixed(1);
                row.appendChild(valSpan);
                card.appendChild(row);
            });

            list.appendChild(card);
        });

        sec.appendChild(list);
        container.appendChild(sec);
    }

    function renderDetailTable(data, container) {
        var sec = el('div', 'fs-section');
        var toggle = el('button', 'fs-table-toggle', '▸ 详细数据表（点击展开）');
        var wrap = el('div', 'fs-table-wrap');
        var samples = data.sample_results || [];
        var eco = data.eco_risk_results || [];
        var crops = data.crop_recommendations || [];

        var table = el('table', 'fs-table');
        var thead = el('thead');
        var hrow = el('tr');
        ['采样点', '土地类型', '风险等级', 'RI'].concat(METALS.map(function (m) { return m + '(mg/kg)'; })).concat(['PH', '有机质']).forEach(function (h) {
            hrow.appendChild(el('th', '', h));
        });
        thead.appendChild(hrow);
        table.appendChild(thead);

        var tbody = el('tbody');
        samples.forEach(function (sr, idx) {
            var tr = el('tr');
            tr.appendChild(el('td', '', sr.sample_name));
            tr.appendChild(el('td', '', crops[idx] ? crops[idx].land_use_type : '--'));
            var riInfo = eco[idx] ? riskInfo(eco[idx].risk_level) : { label: '--', cls: 'yellow' };
            var td = el('td', '');
            td.innerHTML = '<span style="color:' + (RISK_THEME[riInfo.cls] || RISK_THEME.yellow).text + ';font-weight:600">' + riInfo.label + '</span>';
            tr.appendChild(td);
            tr.appendChild(el('td', '', eco[idx] ? eco[idx].ri.toFixed(1) : '--'));

            var lookup = {};
            sr.metal_results.forEach(function (mr) { lookup[mr.metal] = mr.concentration; });
            METALS.forEach(function (m) {
                var c = lookup[m];
                var td2 = el('td', '');
                td2.textContent = c !== undefined ? c.toFixed(2) : '--';
                var crPred = crops[idx] && crops[idx].predictions;
                if (crPred) {
                    var pred = crPred.find(function (p) { return p.metal === m; });
                    if (pred && pred.is_exceed) td2.className = 'fs-val-danger';
                }
                tr.appendChild(td2);
            });

            tr.appendChild(el('td', '', '--'));
            tr.appendChild(el('td', '', '--'));
            tbody.appendChild(tr);
        });
        table.appendChild(tbody);
        wrap.appendChild(table);
        sec.appendChild(toggle);
        sec.appendChild(wrap);

        toggle.addEventListener('click', function () {
            var open = wrap.classList.toggle('open');
            toggle.textContent = (open ? '▾' : '▸') + ' 详细数据表（点击' + (open ? '收起' : '展开') + '）';
        });

        container.appendChild(sec);
    }

    var SoilSafetyEvaluator = {
        setAPIBase: function (url) { API_BASE = url; },
        load: function (siteId) {
            return fetch(API_BASE + '/sites/' + siteId + '/farm-safety')
                .then(function (r) {
                    if (!r.ok) throw new Error('HTTP ' + r.status);
                    return r.json();
                })
                .then(function (json) { return json.data; });
        },

        render: function (data, container) {
            if (typeof container === 'string') container = document.querySelector(container);
            if (!container || !data) return;
            injectStyles();
            container.innerHTML = '';
            renderRiskOverview(data, container);
            renderIgeoHeatmap(data, container);
            renderDistanceDecay(data, container);
            renderCropRecommendations(data, container);
            renderDetailTable(data, container);
        }
    };

    global.SoilSafetyEvaluator = SoilSafetyEvaluator;

})(typeof window !== 'undefined' ? window : this);
