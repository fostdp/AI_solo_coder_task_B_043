(function (global) {
    'use strict';

    var API_BASE = '/api';

    var METAL_COLORS = {
        Cu: '#f0883e',
        Fe: '#8b949e',
        Ag: '#ffd33d',
        Pb: '#58a6ff',
        Hg: '#f85149'
    };

    var METAL_LABELS = {
        Cu: '铜', Fe: '铁', Ag: '银', Pb: '铅', Hg: '汞',
        '铜': 'Cu', '铁': 'Fe', '银': 'Ag', '铅': 'Pb', '汞': 'Hg'
    };

    var EPOCH_COLORS = [
        'rgba(139,92,246,0.12)',
        'rgba(59,130,246,0.12)',
        'rgba(16,185,129,0.12)',
        'rgba(245,158,11,0.12)',
        'rgba(239,68,68,0.12)',
        'rgba(168,85,247,0.12)',
        'rgba(236,72,153,0.12)',
        'rgba(34,211,238,0.12)'
    ];

    var DEFAULT_EPOCHS = [
        { epoch_name: '铜石并用时代', year_start: -5000, year_end: -3300, key_technology: '天然铜冷锻/退火', description: '最早的铜器出现，低温加工天然铜' },
        { epoch_name: '青铜时代早期', year_start: -3300, year_end: -2000, key_technology: '砷青铜/锡青铜铸造', description: '铜-砷/铜-锡合金冶炼，范铸法' },
        { epoch_name: '青铜时代鼎盛期', year_start: -2000, year_end: -1200, key_technology: '失蜡法/大型范铸', description: '大型青铜器出现，锡料贸易网形成' },
        { epoch_name: '铁器时代早期', year_start: -1200, year_end: -500, key_technology: '块炼法炼铁', description: '铁的固态还原，铁器逐渐普及' },
        { epoch_name: '铁器时代鼎盛期', year_start: -500, year_end: 500, key_technology: '生铁铸造/炼钢', description: '液态生铁，炒钢法/百炼钢' },
        { epoch_name: '中世纪冶炼', year_start: 500, year_end: 1500, key_technology: '水力鼓风/竖炉大型化', description: '水力风箱，银铅矿灰吹法普及' },
        { epoch_name: '殖民时代', year_start: 1500, year_end: 1800, key_technology: '混汞法提金/大规模开采', description: '汞齐法贵金属提取，跨大西洋金属贸易' },
        { epoch_name: '工业革命', year_start: 1800, year_end: 1900, key_technology: '焦炭高炉/转炉炼钢', description: '现代冶金工业诞生，产量指数级增长' }
    ];

    var YEAR_MIN = -3000;
    var YEAR_MAX = 2000;

    var CHART_MARGIN = { top: 50, right: 30, bottom: 50, left: 120 };

    var allSites = [];
    var timelineData = null;
    var animState = {
        running: false,
        currentYear: YEAR_MIN,
        speed: 2,
        rafId: null,
        lastTime: 0
    };

    var siteRowColors = [
        '#f0883e', '#58a6ff', '#ffd33d', '#3fb950',
        '#f85149', '#bc8c4e', '#a371f7', '#d29922'
    ];

    var TimelineCompare = {
        setAPIBase: function (url) { API_BASE = url; },
        init: function (sites) {
            allSites = sites || [];
            var sel = document.getElementById('timelineSiteSelect');
            if (!sel) return;
            sel.innerHTML = '';
            allSites.forEach(function (s) {
                var opt = document.createElement('option');
                opt.value = s.id;
                opt.textContent = '#' + s.id + ' ' + s.name + ' (' + (s.metal_type || '') + ')';
                sel.appendChild(opt);
            });
        },

        load: function (siteIDs) {
            var url = API_BASE + '/timeline/compare';
            if (siteIDs && siteIDs.length > 0) {
                url += '?site_ids=' + siteIDs.join(',');
            }
            return fetch(url)
                .then(function (r) { return r.json(); })
                .then(function (d) {
                    timelineData = d.data || null;
                    return timelineData;
                })
                .catch(function (e) {
                    console.error('加载时间线数据失败:', e);
                    return null;
                });
        },

        render: function (data) {
            if (!data) data = timelineData;
            if (!data) return;
            drawTimeline(data);
            updateEpochInfo(data, animState.currentYear);
        },

        toggleAnimation: function () {
            if (animState.running) {
                pauseAnimation();
            } else {
                startAnimation();
            }
            updateAnimButton();
        }
    };

    function formatYear(year) {
        if (year < 0) return '公元前' + Math.abs(year) + '年';
        return '公元' + year + '年';
    }

    function metalColor(type) {
        if (!type) return '#8b949e';
        var t = type.replace(/[铜铁银铅汞]/g, function (m) {
            return { '铜': 'Cu', '铁': 'Fe', '银': 'Ag', '铅': 'Pb', '汞': 'Hg' }[m] || m;
        });
        if (t.indexOf('Cu') >= 0 || t.indexOf('铜') >= 0) return METAL_COLORS.Cu;
        if (t.indexOf('Fe') >= 0 || t.indexOf('铁') >= 0) return METAL_COLORS.Fe;
        if (t.indexOf('Ag') >= 0 || t.indexOf('银') >= 0) return METAL_COLORS.Ag;
        if (t.indexOf('Pb') >= 0 || t.indexOf('铅') >= 0) return METAL_COLORS.Pb;
        if (t.indexOf('Hg') >= 0 || t.indexOf('汞') >= 0) return METAL_COLORS.Hg;
        return '#8b949e';
    }

    function getMetalKey(type) {
        if (!type) return 'Cu';
        if (type.indexOf('Cu') >= 0 || type.indexOf('铜') >= 0) return 'Cu';
        if (type.indexOf('Fe') >= 0 || type.indexOf('铁') >= 0) return 'Fe';
        if (type.indexOf('Ag') >= 0 || type.indexOf('银') >= 0) return 'Ag';
        if (type.indexOf('Pb') >= 0 || type.indexOf('铅') >= 0) return 'Pb';
        if (type.indexOf('Hg') >= 0 || type.indexOf('汞') >= 0) return 'Hg';
        return 'Cu';
    }

    function drawTimeline(data) {
        var canvas = document.getElementById('timelineCanvas');
        if (!canvas) return;

        var dpr = window.devicePixelRatio || 1;
        var w = canvas.clientWidth || 900;
        var h = canvas.clientHeight || 400;
        canvas.width = w * dpr;
        canvas.height = h * dpr;
        var ctx = canvas.getContext('2d');
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx.clearRect(0, 0, w, h);

        var sites = data.sites || [];
        var peaks = data.peaks || [];
        var epochs = data.civilization_epochs && data.civilization_epochs.length > 0
            ? data.civilization_epochs
            : DEFAULT_EPOCHS;
        var globalTrend = data.global_trend || {};

        var displaySites = sites.slice(0, 8);
        var trendRows = 1;
        var totalRows = displaySites.length + trendRows;
        var rowH = Math.max(20, (h - CHART_MARGIN.top - CHART_MARGIN.bottom) / totalRows);
        var plotW = w - CHART_MARGIN.left - CHART_MARGIN.right;
        var plotH = h - CHART_MARGIN.top - CHART_MARGIN.bottom;

        var xScale = function (year) {
            return CHART_MARGIN.left + ((year - YEAR_MIN) / (YEAR_MAX - YEAR_MIN)) * plotW;
        };

        drawBackground(ctx, w, h);
        drawEpochBands(ctx, epochs, xScale, h);
        drawGrid(ctx, xScale, w, h);
        drawYearAxis(ctx, xScale, w, h);

        displaySites.forEach(function (site, idx) {
            var yTop = CHART_MARGIN.top + idx * rowH;
            drawSiteRow(ctx, site, idx, yTop, rowH, xScale, peaks, plotW);
        });

        var trendY = CHART_MARGIN.top + displaySites.length * rowH;
        drawGlobalTrendRow(ctx, globalTrend, trendY, rowH, xScale, plotW);

        if (animState.running || animState.currentYear > YEAR_MIN) {
            drawScanLine(ctx, xScale, h);
            drawPeakBubbles(ctx, peaks, xScale, displaySites, rowH);
        }

        drawSiteLabels(ctx, displaySites, rowH, peaks);
        drawLegend(ctx, w, h);
    }

    function drawBackground(ctx, w, h) {
        ctx.fillStyle = '#0d1117';
        ctx.fillRect(0, 0, w, h);
    }

    function drawEpochBands(ctx, epochs, xScale, h) {
        epochs.forEach(function (ep, idx) {
            var x1 = xScale(Math.max(ep.year_start, YEAR_MIN));
            var x2 = xScale(Math.min(ep.year_end, YEAR_MAX));
            if (x2 <= x1) return;

            ctx.fillStyle = EPOCH_COLORS[idx % EPOCH_COLORS.length];
            ctx.fillRect(x1, CHART_MARGIN.top, x2 - x1, h - CHART_MARGIN.top - CHART_MARGIN.bottom);

            ctx.save();
            ctx.fillStyle = 'rgba(201,209,217,0.35)';
            ctx.font = '10px sans-serif';
            ctx.textAlign = 'center';
            var labelX = (x1 + x2) / 2;
            if (x2 - x1 > 60) {
                ctx.fillText(ep.epoch_name, labelX, CHART_MARGIN.top - 20);
            } else {
                ctx.save();
                ctx.translate(labelX, CHART_MARGIN.top - 8);
                ctx.rotate(-0.5);
                ctx.fillText(ep.epoch_name, 0, 0);
                ctx.restore();
            }

            if (x2 - x1 > 80 && ep.key_technology) {
                ctx.fillStyle = 'rgba(139,148,158,0.4)';
                ctx.font = '8px sans-serif';
                ctx.fillText(ep.key_technology, labelX, CHART_MARGIN.top - 6);
            }
            ctx.restore();
        });
    }

    function drawGrid(ctx, xScale, w, h) {
        ctx.strokeStyle = 'rgba(48,54,61,0.5)';
        ctx.lineWidth = 0.5;

        for (var year = -3000; year <= 2000; year += 500) {
            var x = xScale(year);
            ctx.beginPath();
            ctx.moveTo(x, CHART_MARGIN.top);
            ctx.lineTo(x, h - CHART_MARGIN.bottom);
            ctx.stroke();
        }
    }

    function drawYearAxis(ctx, xScale, w, h) {
        ctx.fillStyle = '#8b949e';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'center';

        for (var year = -3000; year <= 2000; year += 500) {
            var x = xScale(year);
            var label = year < 0 ? '前' + Math.abs(year) : String(year);
            ctx.fillText(label, x, h - CHART_MARGIN.bottom + 18);
        }

        ctx.fillStyle = '#6e7681';
        ctx.font = '9px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('年份', CHART_MARGIN.left + (w - CHART_MARGIN.left - CHART_MARGIN.right) / 2, h - CHART_MARGIN.bottom + 38);
    }

    function drawSiteRow(ctx, site, idx, yTop, rowH, xScale, peaks, plotW) {
        var sitePeaks = peaks.filter(function (p) { return p.site_id === site.id; });
        var color = siteRowColors[idx % siteRowColors.length];
        var metalC = metalColor(site.metal_type);

        ctx.fillStyle = 'rgba(22,27,34,0.6)';
        ctx.fillRect(CHART_MARGIN.left, yTop, plotW, rowH);

        ctx.strokeStyle = 'rgba(48,54,61,0.4)';
        ctx.lineWidth = 0.5;
        ctx.beginPath();
        ctx.moveTo(CHART_MARGIN.left, yTop + rowH);
        ctx.lineTo(CHART_MARGIN.left + plotW, yTop + rowH);
        ctx.stroke();

        if (sitePeaks.length < 2) {
            drawPeakDots(ctx, sitePeaks, xScale, yTop, rowH, metalC);
            return;
        }

        sitePeaks.sort(function (a, b) { return a.peak_year - b.peak_year; });

        var firstYear = sitePeaks[0].peak_year;
        var lastYear = sitePeaks[sitePeaks.length - 1].peak_year;
        if (firstYear > YEAR_MIN) {
            sitePeaks.unshift({ peak_year: YEAR_MIN, peak_value: 0, site_id: site.id });
        }
        if (lastYear < YEAR_MAX) {
            sitePeaks.push({ peak_year: YEAR_MAX, peak_value: 0, site_id: site.id });
        }

        var maxVal = 0;
        sitePeaks.forEach(function (p) { if (p.peak_value > maxVal) maxVal = p.peak_value; });
        if (maxVal === 0) maxVal = 1;

        var yMid = yTop + rowH * 0.5;
        var amplitude = rowH * 0.4;
        var yScale = function (val) {
            return yMid - (val / maxVal) * amplitude;
        };

        ctx.beginPath();
        ctx.moveTo(xScale(sitePeaks[0].peak_year), yScale(sitePeaks[0].peak_value));
        for (var i = 1; i < sitePeaks.length; i++) {
            var px = xScale(sitePeaks[i - 1].peak_year);
            var py = yScale(sitePeaks[i - 1].peak_value);
            var nx = xScale(sitePeaks[i].peak_year);
            var ny = yScale(sitePeaks[i].peak_value);
            var cpx = (px + nx) / 2;
            ctx.bezierCurveTo(cpx, py, cpx, ny, nx, ny);
        }

        ctx.strokeStyle = metalC;
        ctx.lineWidth = 1.5;
        ctx.stroke();

        ctx.lineTo(xScale(sitePeaks[sitePeaks.length - 1].peak_year), yMid);
        ctx.lineTo(xScale(sitePeaks[0].peak_year), yMid);
        ctx.closePath();

        var grad = ctx.createLinearGradient(0, yTop, 0, yTop + rowH);
        var baseColor = metalC.replace('#', '');
        var r = parseInt(baseColor.substring(0, 2), 16);
        var g = parseInt(baseColor.substring(2, 4), 16);
        var b = parseInt(baseColor.substring(4, 6), 16);
        grad.addColorStop(0, 'rgba(' + r + ',' + g + ',' + b + ',0.25)');
        grad.addColorStop(1, 'rgba(' + r + ',' + g + ',' + b + ',0.02)');
        ctx.fillStyle = grad;
        ctx.fill();

        drawPeakDots(ctx, peaks.filter(function (p) { return p.site_id === site.id; }), xScale, yTop, rowH, metalC, yScale, maxVal);
    }

    function drawPeakDots(ctx, sitePeaks, xScale, yTop, rowH, color, yScaleFn, maxVal) {
        sitePeaks.forEach(function (p) {
            if (p.peak_year < YEAR_MIN || p.peak_year > YEAR_MAX) return;

            var x = xScale(p.peak_year);
            var y;
            if (yScaleFn && maxVal > 0) {
                y = yScaleFn(p.peak_value);
            } else {
                y = yTop + rowH * 0.5 - p.peak_value * rowH * 0.35;
            }

            ctx.beginPath();
            ctx.arc(x, y, 4, 0, Math.PI * 2);
            ctx.fillStyle = '#f85149';
            ctx.fill();
            ctx.strokeStyle = 'rgba(255,255,255,0.6)';
            ctx.lineWidth = 1;
            ctx.stroke();

            ctx.fillStyle = 'rgba(248,81,73,0.9)';
            ctx.font = 'bold 8px sans-serif';
            ctx.textAlign = 'center';
            var label = p.peak_year < 0 ? '前' + Math.abs(p.peak_year) : String(p.peak_year);
            ctx.fillText(label, x, y - 8);
        });
    }

    function drawGlobalTrendRow(ctx, globalTrend, yTop, rowH, xScale, plotW) {
        ctx.fillStyle = 'rgba(13,17,23,0.8)';
        ctx.fillRect(CHART_MARGIN.left, yTop, plotW, rowH);

        ctx.strokeStyle = 'rgba(88,166,255,0.3)';
        ctx.lineWidth = 1;
        ctx.setLineDash([4, 4]);
        ctx.beginPath();
        ctx.moveTo(CHART_MARGIN.left, yTop);
        ctx.lineTo(CHART_MARGIN.left + plotW, yTop);
        ctx.stroke();
        ctx.setLineDash([]);

        ctx.fillStyle = '#6e7681';
        ctx.font = '9px sans-serif';
        ctx.textAlign = 'left';
        ctx.fillText('全球趋势', CHART_MARGIN.left + 4, yTop + 10);

        var years = globalTrend.years || [];
        if (years.length === 0) return;

        var yMid = yTop + rowH * 0.55;
        var amplitude = rowH * 0.35;

        var trendLines = [
            { key: 'all_sites_avg', color: '#e6edf3', width: 2.5, label: '平均' },
            { key: 'copper_sites', color: METAL_COLORS.Cu, width: 1.2, label: 'Cu' },
            { key: 'iron_sites', color: METAL_COLORS.Fe, width: 1.2, label: 'Fe' },
            { key: 'silver_sites', color: METAL_COLORS.Ag, width: 1.2, label: 'Ag' },
            { key: 'lead_sites', color: METAL_COLORS.Pb, width: 1.2, label: 'Pb' },
            { key: 'mercury_sites', color: METAL_COLORS.Hg, width: 1.2, label: 'Hg' }
        ];

        var globalMax = 0;
        trendLines.forEach(function (line) {
            var vals = globalTrend[line.key] || [];
            vals.forEach(function (v) { if (v > globalMax) globalMax = v; });
        });
        if (globalMax === 0) globalMax = 1;

        trendLines.forEach(function (line) {
            var vals = globalTrend[line.key] || [];
            if (vals.length === 0) return;

            ctx.beginPath();
            ctx.strokeStyle = line.color;
            ctx.lineWidth = line.width;
            if (line.key !== 'all_sites_avg') {
                ctx.globalAlpha = 0.7;
            }
            var started = false;
            for (var i = 0; i < years.length; i++) {
                var x = xScale(years[i]);
                var y = yMid - (vals[i] / globalMax) * amplitude;
                if (!started) { ctx.moveTo(x, y); started = true; }
                else ctx.lineTo(x, y);
            }
            ctx.stroke();
            ctx.globalAlpha = 1;
        });
    }

    function drawScanLine(ctx, xScale, h) {
        var x = xScale(animState.currentYear);
        if (x < CHART_MARGIN.left || x > h) return;

        ctx.strokeStyle = 'rgba(56,211,159,0.8)';
        ctx.lineWidth = 2;
        ctx.setLineDash([6, 3]);
        ctx.beginPath();
        ctx.moveTo(x, CHART_MARGIN.top);
        ctx.lineTo(x, h - CHART_MARGIN.bottom);
        ctx.stroke();
        ctx.setLineDash([]);

        ctx.beginPath();
        ctx.arc(x, CHART_MARGIN.top, 4, 0, Math.PI * 2);
        ctx.fillStyle = '#38d39f';
        ctx.fill();

        ctx.fillStyle = '#38d39f';
        ctx.font = 'bold 10px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(formatYear(Math.round(animState.currentYear)), x, CHART_MARGIN.top - 28);
    }

    function drawPeakBubbles(ctx, peaks, xScale, displaySites, rowH) {
        var scanYear = Math.round(animState.currentYear);
        var scanX = xScale(animState.currentYear);

        peaks.forEach(function (p) {
            if (Math.abs(p.peak_year - scanYear) > 80) return;

            var siteIdx = -1;
            for (var i = 0; i < displaySites.length; i++) {
                if (displaySites[i].id === p.site_id) { siteIdx = i; break; }
            }
            if (siteIdx < 0) return;

            var px = xScale(p.peak_year);
            var py = CHART_MARGIN.top + siteIdx * rowH + rowH * 0.5;
            var dist = Math.abs(px - scanX);
            if (dist > 30) return;

            var opacity = 1 - dist / 30;
            ctx.save();
            ctx.globalAlpha = opacity;

            var bw = 120;
            var bh = 36;
            var bx = px - bw / 2;
            var by = py - bh - 12;

            ctx.fillStyle = 'rgba(22,27,34,0.92)';
            ctx.strokeStyle = 'rgba(56,211,159,0.6)';
            ctx.lineWidth = 1;
            roundRect(ctx, bx, by, bw, bh, 6);
            ctx.fill();
            ctx.stroke();

            ctx.fillStyle = '#e6edf3';
            ctx.font = 'bold 9px sans-serif';
            ctx.textAlign = 'left';
            var yearLabel = p.peak_year < 0 ? '前' + Math.abs(p.peak_year) + '年' : p.peak_year + '年';
            ctx.fillText(p.site_name + ' ' + yearLabel, bx + 8, by + 14);

            ctx.fillStyle = '#8b949e';
            ctx.font = '8px sans-serif';
            ctx.fillText('峰值: ' + p.peak_value.toFixed(2) + '  ' + (p.metal_type || ''), bx + 8, by + 27);

            ctx.restore();
        });
    }

    function roundRect(ctx, x, y, w, h, r) {
        ctx.beginPath();
        ctx.moveTo(x + r, y);
        ctx.lineTo(x + w - r, y);
        ctx.quadraticCurveTo(x + w, y, x + w, y + r);
        ctx.lineTo(x + w, y + h - r);
        ctx.quadraticCurveTo(x + w, y + h, x + w - r, y + h);
        ctx.lineTo(x + r, y + h);
        ctx.quadraticCurveTo(x, y + h, x, y + h - r);
        ctx.lineTo(x, y + r);
        ctx.quadraticCurveTo(x, y, x + r, y);
        ctx.closePath();
    }

    function drawSiteLabels(ctx, displaySites, rowH, peaks) {
        ctx.fillStyle = '#c9d1d9';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'right';

        displaySites.forEach(function (site, idx) {
            var yMid = CHART_MARGIN.top + idx * rowH + rowH * 0.5;
            var name = site.name || ('遗址#' + site.id);
            if (name.length > 10) name = name.substring(0, 10) + '…';
            ctx.fillStyle = siteRowColors[idx % siteRowColors.length];
            ctx.fillText(name, CHART_MARGIN.left - 8, yMid - 3);

            var mLabel = site.metal_type || '';
            ctx.fillStyle = metalColor(site.metal_type);
            ctx.font = '9px sans-serif';
            ctx.fillText(mLabel, CHART_MARGIN.left - 8, yMid + 9);
            ctx.font = '10px sans-serif';
        });

        var trendY = CHART_MARGIN.top + displaySites.length * rowH + rowH * 0.5;
        ctx.fillStyle = '#e6edf3';
        ctx.font = '10px sans-serif';
        ctx.fillText('全球趋势', CHART_MARGIN.left - 8, trendY - 3);
        ctx.fillStyle = '#6e7681';
        ctx.font = '9px sans-serif';
        ctx.fillText('归一化平均', CHART_MARGIN.left - 8, trendY + 9);
    }

    function drawLegend(ctx, w, h) {
        var x = CHART_MARGIN.left + 8;
        var y = h - CHART_MARGIN.bottom + 28;

        var items = [
            { label: 'Cu 铜', color: METAL_COLORS.Cu },
            { label: 'Fe 铁', color: METAL_COLORS.Fe },
            { label: 'Ag 银', color: METAL_COLORS.Ag },
            { label: 'Pb 铅', color: METAL_COLORS.Pb },
            { label: 'Hg 汞', color: METAL_COLORS.Hg }
        ];

        ctx.font = '9px sans-serif';
        ctx.textAlign = 'left';
        items.forEach(function (item) {
            ctx.fillStyle = item.color;
            ctx.fillRect(x, y, 12, 8);
            ctx.fillStyle = '#8b949e';
            ctx.fillText(item.label, x + 16, y + 7);
            x += 70;
        });
    }

    function startAnimation() {
        animState.running = true;
        animState.lastTime = performance.now();
        animState.rafId = requestAnimationFrame(animStep);
    }

    function pauseAnimation() {
        animState.running = false;
        if (animState.rafId) {
            cancelAnimationFrame(animState.rafId);
            animState.rafId = null;
        }
    }

    function resetAnimation() {
        pauseAnimation();
        animState.currentYear = YEAR_MIN;
        if (timelineData) {
            drawTimeline(timelineData);
            updateEpochInfo(timelineData, animState.currentYear);
        }
        updateAnimButton();
    }

    function animStep(timestamp) {
        if (!animState.running) return;

        var dt = (timestamp - animState.lastTime) / 1000;
        animState.lastTime = timestamp;

        var yearsPerSec = 1000 / animState.speed;
        animState.currentYear += dt * yearsPerSec;

        if (animState.currentYear >= YEAR_MAX) {
            animState.currentYear = YEAR_MAX;
            pauseAnimation();
            updateAnimButton();
        }

        if (timelineData) {
            drawTimeline(timelineData);
            updateEpochInfo(timelineData, animState.currentYear);
        }

        if (animState.running) {
            animState.rafId = requestAnimationFrame(animStep);
        }
    }

    function updateAnimButton() {
        var btn = document.querySelector('[onclick="toggleTimelineAnimation()"]');
        if (!btn) return;
        if (animState.running) {
            btn.textContent = '⏸ 暂停动画';
        } else {
            btn.textContent = '▶ 播放动画';
        }
    }

    function updateEpochInfo(data, currentYear) {
        var el = document.getElementById('timelineEpochInfo');
        if (!el) return;

        var epochs = data.civilization_epochs && data.civilization_epochs.length > 0
            ? data.civilization_epochs
            : DEFAULT_EPOCHS;
        var currentEpoch = null;
        for (var i = 0; i < epochs.length; i++) {
            if (currentYear >= epochs[i].year_start && currentYear < epochs[i].year_end) {
                currentEpoch = epochs[i];
                break;
            }
        }

        if (!currentEpoch) {
            el.innerHTML = '<div style="color:#6e7681;font-size:12px;">当前年份: ' + formatYear(Math.round(currentYear)) + '</div>';
            return;
        }

        var sites = data.sites || [];
        var peaks = data.peaks || [];
        var relatedPeaks = peaks.filter(function (p) {
            return p.peak_year >= currentEpoch.year_start && p.peak_year < currentEpoch.year_end;
        });
        var relatedSites = [];
        var seen = {};
        relatedPeaks.forEach(function (p) {
            if (!seen[p.site_name]) {
                relatedSites.push(p.site_name);
                seen[p.site_name] = true;
            }
        });

        var siteList = relatedSites.length > 0 ? relatedSites.join('、') : '暂无';
        var html = '<div style="background:#161b22;border:1px solid #30363d;border-radius:8px;padding:14px;">';
        html += '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px;">';
        html += '<span style="font-size:15px;font-weight:700;color:#e6edf3;">' + currentEpoch.epoch_name + '</span>';
        html += '<span style="font-size:11px;color:#8b949e;">' + formatYear(Math.round(currentYear)) + '</span>';
        html += '</div>';
        html += '<div style="display:grid;grid-template-columns:1fr 1fr;gap:8px;font-size:12px;">';
        html += '<div><span style="color:#6e7681;">年代范围：</span><span style="color:#c9d1d9;">' + currentEpoch.year_range + '</span></div>';
        html += '<div><span style="color:#6e7681;">关键技术：</span><span style="color:#58a6ff;">' + (currentEpoch.key_technology || '-') + '</span></div>';
        html += '</div>';
        html += '<div style="margin-top:8px;font-size:12px;"><span style="color:#6e7681;">涉及遗址：</span><span style="color:#3fb950;">' + siteList + '</span></div>';
        if (currentEpoch.description) {
            html += '<div style="margin-top:8px;font-size:11px;color:#8b949e;line-height:1.5;border-top:1px solid #21262d;padding-top:8px;">' + currentEpoch.description + '</div>';
        }
        html += '</div>';
        el.innerHTML = html;
    }

    global.openTimelineModal = function () {
        var modal = document.getElementById('timelineModal');
        if (modal) {
            modal.style.display = 'flex';
            resetAnimation();
            if (allSites.length === 0 && typeof fetch === 'function') {
                fetch(API_BASE + '/sites')
                    .then(function (r) { return r.json(); })
                    .then(function (d) {
                        var sites = d.data || [];
                        TimelineCompare.init(sites);
                    })
                    .catch(function () {});
            }
        }
    };

    global.closeTimelineModal = function () {
        var modal = document.getElementById('timelineModal');
        if (modal) {
            modal.style.display = 'none';
        }
        pauseAnimation();
    };

    global.loadTimelineData = function () {
        var sel = document.getElementById('timelineSiteSelect');
        var siteIDs = [];
        if (sel) {
            for (var i = 0; i < sel.options.length; i++) {
                if (sel.options[i].selected) {
                    siteIDs.push(parseInt(sel.options[i].value, 10));
                }
            }
        }

        TimelineCompare.load(siteIDs).then(function (data) {
            if (data) {
                resetAnimation();
                TimelineCompare.render(data);
            }
        });
    };

    global.toggleTimelineAnimation = function () {
        TimelineCompare.toggleAnimation();
    };

    global.TimelineCompare = TimelineCompare;

})(typeof window !== 'undefined' ? window : this);
