(function (global) {
    'use strict';

    var API_BASE = '/api';

    var INPUT_LABELS = ['Pb/Zn', 'Cu/Pb', 'As/Hg', 'Cd/Zn', 'Cu/As', 'CaO/SiO\u2082', 'Fe_total', 'SO\u2083'];
    var INPUT_KEYS = ['pb_zn_ratio', 'cu_pb_ratio', 'as_hg_ratio', 'cd_zn_ratio', 'cu_as_ratio', 'cao_sio2_ratio', 'feo_total', 'so3_content'];
    var AGENT_KEYS = ['木炭', '焦炭', '煤', '混合'];
    var AGENT_COLORS = { '木炭': '#8B4513', '焦炭': '#333333', '煤': '#1a1a2e', '混合': '#6c757d' };

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

    function tempColor(t) {
        if (t < 700) return '#5dade2';
        if (t <= 1000) return '#e67e22';
        return '#e74c3c';
    }

    function qualityStyle(level) {
        if (level === '高') return { bg: 'rgba(76,175,80,0.15)', color: '#4caf50', border: '#4caf50' };
        if (level === '中') return { bg: 'rgba(255,193,7,0.15)', color: '#ffc107', border: '#ffc107' };
        return { bg: 'rgba(244,67,54,0.15)', color: '#f44336', border: '#f44336' };
    }

    function drawHistogram(canvas, distribution, estTemp) {
        var w = canvas.clientWidth || 320;
        var h = canvas.clientHeight || 160;
        var ctx = setupCanvas(canvas, w, h);
        ctx.clearRect(0, 0, w, h);

        var pad = { top: 20, right: 15, bottom: 30, left: 40 };
        var pw = w - pad.left - pad.right;
        var ph = h - pad.top - pad.bottom;
        var nBins = distribution.length || 10;
        var barW = pw / nBins - 4;
        var maxVal = 0;
        for (var i = 0; i < nBins; i++) {
            if (distribution[i] > maxVal) maxVal = distribution[i];
        }
        if (maxVal === 0) maxVal = 1;

        ctx.fillStyle = '#8b949e';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'center';
        for (var i = 0; i < nBins; i++) {
            var start = 500 + i * 110;
            var x = pad.left + i * (pw / nBins) + (pw / nBins) / 2;
            ctx.fillText(start + '', x, h - 8);
        }
        ctx.fillText('\u2103', w - 8, h - 8);

        for (var i = 0; i < nBins; i++) {
            var barH = (distribution[i] / maxVal) * ph;
            var x = pad.left + i * (pw / nBins) + 2;
            var y = pad.top + ph - barH;
            var grad = ctx.createLinearGradient(x, y, x, y + barH);
            var binCenter = 500 + i * 110 + 55;
            if (Math.abs(binCenter - estTemp) < 165) {
                grad.addColorStop(0, '#5dade2');
                grad.addColorStop(1, 'rgba(93,173,226,0.3)');
            } else {
                grad.addColorStop(0, 'rgba(201,209,217,0.5)');
                grad.addColorStop(1, 'rgba(201,209,217,0.1)');
            }
            ctx.fillStyle = grad;
            ctx.fillRect(x, y, barW, barH);
            ctx.strokeStyle = 'rgba(201,209,217,0.3)';
            ctx.lineWidth = 0.5;
            ctx.strokeRect(x, y, barW, barH);
        }

        ctx.strokeStyle = 'rgba(201,209,217,0.15)';
        ctx.lineWidth = 0.5;
        for (var i = 0; i <= 4; i++) {
            var y = pad.top + (ph / 4) * i;
            ctx.beginPath();
            ctx.moveTo(pad.left, y);
            ctx.lineTo(pad.left + pw, y);
            ctx.stroke();
        }
    }

    function drawNetwork(canvas, data) {
        var w = canvas.clientWidth || 560;
        var h = canvas.clientHeight || 320;
        var ctx = setupCanvas(canvas, w, h);
        ctx.clearRect(0, 0, w, h);

        var features = (data && data.inversion && data.inversion.input_features) || {};
        var layers = [8, 32, 16, 5];
        var layerX = [60, 190, 320, 470];
        var layerLabels = [
            INPUT_LABELS,
            null,
            null,
            ['\u6e29\u5ea6', '\u6728\u70ad', '\u7126\u70ad', '\u7164', '\u6df7\u5408']
        ];
        var validFeatures = [];
        for (var i = 0; i < INPUT_KEYS.length; i++) {
            var val = features[INPUT_KEYS[i]];
            validFeatures.push(val !== undefined && val !== null && val > 0);
        }

        var nodes = [];
        for (var l = 0; l < layers.length; l++) {
            var count = layers[l];
            var layerNodes = [];
            var totalH = h - 40;
            var spacing = Math.min(totalH / (count + 1), 28);
            var startY = 20 + (totalH - spacing * (count - 1)) / 2;
            for (var n = 0; n < count; n++) {
                var ny = startY + n * spacing;
                var active = false;
                if (l === 0) {
                    active = validFeatures[n];
                } else if (l === 3) {
                    active = true;
                } else {
                    active = Math.random() > 0.3;
                }
                layerNodes.push({ x: layerX[l], y: ny, active: active });
            }
            nodes.push(layerNodes);
        }

        for (var l = 0; l < nodes.length - 1; l++) {
            var src = nodes[l];
            var dst = nodes[l + 1];
            var maxConn = src.length * dst.length;
            var step = maxConn > 200 ? 3 : maxConn > 100 ? 2 : 1;
            for (var i = 0; i < src.length; i++) {
                for (var j = 0; j < dst.length; j += step) {
                    var bothActive = src[i].active && dst[j].active;
                    ctx.strokeStyle = bothActive ? 'rgba(93,173,226,0.12)' : 'rgba(139,148,158,0.04)';
                    ctx.lineWidth = bothActive ? 0.6 : 0.3;
                    ctx.beginPath();
                    ctx.moveTo(src[i].x, src[i].y);
                    ctx.lineTo(dst[j].x, dst[j].y);
                    ctx.stroke();
                }
            }
        }

        for (var l = 0; l < nodes.length; l++) {
            for (var n = 0; n < nodes[l].length; n++) {
                var nd = nodes[l][n];
                var radius = l === 0 || l === 3 ? 6 : 3.5;
                if (nd.active) {
                    ctx.beginPath();
                    ctx.arc(nd.x, nd.y, radius + 2, 0, Math.PI * 2);
                    ctx.fillStyle = l === 0 ? 'rgba(93,173,226,0.2)' : l === 3 ? 'rgba(76,175,80,0.2)' : 'rgba(255,193,7,0.15)';
                    ctx.fill();
                }
                ctx.beginPath();
                ctx.arc(nd.x, nd.y, radius, 0, Math.PI * 2);
                ctx.fillStyle = nd.active
                    ? (l === 0 ? '#5dade2' : l === 3 ? '#4caf50' : '#ffc107')
                    : 'rgba(139,148,158,0.25)';
                ctx.fill();
                ctx.strokeStyle = nd.active ? 'rgba(255,255,255,0.3)' : 'rgba(139,148,158,0.15)';
                ctx.lineWidth = 0.8;
                ctx.stroke();
            }
        }

        ctx.font = '9px sans-serif';
        ctx.textAlign = 'right';
        for (var n = 0; n < nodes[0].length; n++) {
            var nd = nodes[0][n];
            ctx.fillStyle = nd.active ? '#c9d1d9' : 'rgba(139,148,158,0.5)';
            ctx.fillText(layerLabels[0][n], nd.x - 10, nd.y + 3);
        }
        ctx.textAlign = 'left';
        for (var n = 0; n < nodes[3].length; n++) {
            var nd = nodes[3][n];
            ctx.fillStyle = '#c9d1d9';
            ctx.fillText(layerLabels[3][n], nd.x + 10, nd.y + 3);
        }

        ctx.font = '10px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillStyle = '#8b949e';
        var layerNames = ['\u8f93\u5165\u5c42(8)', '\u9690\u85cf\u5c421(32)', '\u9690\u85cf\u5c422(16)', '\u8f93\u51fa\u5c42(5)'];
        for (var l = 0; l < layers.length; l++) {
            ctx.fillText(layerNames[l], layerX[l], h - 4);
        }
    }

    function renderTemperatureCard(container, inv) {
        var temp = inv.estimated_temperature || 0;
        var conf = inv.temperature_confidence || 0;
        var tc = tempColor(temp);

        var card = document.createElement('div');
        card.style.cssText = 'background:#161b22;border:1px solid #30363d;border-radius:8px;padding:20px;margin-bottom:16px;';

        var header = document.createElement('div');
        header.style.cssText = 'font-size:12px;color:#8b949e;margin-bottom:12px;letter-spacing:1px;';
        header.textContent = '\u6e29\u5ea6\u4f30\u7b97';
        card.appendChild(header);

        var tempRow = document.createElement('div');
        tempRow.style.cssText = 'display:flex;align-items:baseline;gap:8px;margin-bottom:16px;';

        var tempVal = document.createElement('span');
        tempVal.style.cssText = 'font-size:48px;font-weight:700;color:' + tc + ';line-height:1;';
        tempVal.textContent = Math.round(temp) + '\u2103';
        tempRow.appendChild(tempVal);

        var tempLabel = document.createElement('span');
        tempLabel.style.cssText = 'font-size:13px;color:#8b949e;';
        tempLabel.textContent = temp < 700 ? '\u4f4e\u6e29' : temp <= 1000 ? '\u4e2d\u6e29' : '\u9ad8\u6e29';
        tempRow.appendChild(tempLabel);
        card.appendChild(tempRow);

        var confRow = document.createElement('div');
        confRow.style.cssText = 'margin-bottom:16px;';

        var confHeader = document.createElement('div');
        confHeader.style.cssText = 'display:flex;justify-content:space-between;margin-bottom:6px;';

        var confLabel = document.createElement('span');
        confLabel.style.cssText = 'font-size:12px;color:#8b949e;';
        confLabel.textContent = '\u7f6e\u4fe1\u5ea6';
        confHeader.appendChild(confLabel);

        var confVal = document.createElement('span');
        confVal.style.cssText = 'font-size:12px;color:#4caf50;font-weight:600;';
        confVal.textContent = (conf * 100).toFixed(0) + '%';
        confHeader.appendChild(confVal);
        confRow.appendChild(confHeader);

        var barBg = document.createElement('div');
        barBg.style.cssText = 'height:8px;background:#21262d;border-radius:4px;overflow:hidden;';

        var barFill = document.createElement('div');
        barFill.style.cssText = 'height:100%;width:' + (conf * 100) + '%;background:#4caf50;border-radius:4px;transition:width 0.3s;';
        barBg.appendChild(barFill);
        confRow.appendChild(barBg);
        card.appendChild(confRow);

        var histTitle = document.createElement('div');
        histTitle.style.cssText = 'font-size:11px;color:#8b949e;margin-bottom:8px;';
        histTitle.textContent = '\u6e29\u5ea6\u540e\u9a8c\u5206\u5e03';
        card.appendChild(histTitle);

        var histCanvas = document.createElement('canvas');
        histCanvas.style.cssText = 'width:100%;height:140px;display:block;';
        card.appendChild(histCanvas);

        container.appendChild(card);

        setTimeout(function () {
            drawHistogram(histCanvas, [], temp);
        }, 0);

        return histCanvas;
    }

    function renderAgentBars(container, data) {
        var probs = data.agent_probabilities || {};
        var inv = data.inversion || {};
        var bestAgent = inv.reducing_agent || '';

        var card = document.createElement('div');
        card.style.cssText = 'background:#161b22;border:1px solid #30363d;border-radius:8px;padding:20px;margin-bottom:16px;';

        var header = document.createElement('div');
        header.style.cssText = 'font-size:12px;color:#8b949e;margin-bottom:16px;letter-spacing:1px;';
        header.textContent = '\u8fd8\u539f\u5242\u63a8\u65ad';
        card.appendChild(header);

        for (var i = 0; i < AGENT_KEYS.length; i++) {
            var agent = AGENT_KEYS[i];
            var prob = probs[agent] || 0;
            var isBest = agent === bestAgent;

            var row = document.createElement('div');
            row.style.cssText = 'margin-bottom:12px;';

            var labelRow = document.createElement('div');
            labelRow.style.cssText = 'display:flex;justify-content:space-between;align-items:center;margin-bottom:5px;';

            var label = document.createElement('span');
            label.style.cssText = 'font-size:13px;color:#c9d1d9;' + (isBest ? 'font-weight:700;' : '');
            label.textContent = (isBest ? '\u2605 ' : '') + agent;
            labelRow.appendChild(label);

            var pct = document.createElement('span');
            pct.style.cssText = 'font-size:12px;color:#8b949e;' + (isBest ? 'font-weight:700;color:#c9d1d9;' : '');
            pct.textContent = (prob * 100).toFixed(1) + '%';
            labelRow.appendChild(pct);
            row.appendChild(labelRow);

            var barBg = document.createElement('div');
            barBg.style.cssText = 'height:10px;background:#21262d;border-radius:5px;overflow:hidden;';

            var barFill = document.createElement('div');
            barFill.style.cssText = 'height:100%;width:' + (prob * 100) + '%;background:' + AGENT_COLORS[agent] + ';border-radius:5px;transition:width 0.3s;' + (isBest ? 'box-shadow:0 0 6px ' + AGENT_COLORS[agent] + ';' : '');
            barBg.appendChild(barFill);
            row.appendChild(barBg);
            card.appendChild(row);
        }

        container.appendChild(card);
    }

    function renderProcessDetails(container, inv) {
        var card = document.createElement('div');
        card.style.cssText = 'background:#161b22;border:1px solid #30363d;border-radius:8px;padding:20px;margin-bottom:16px;';

        var header = document.createElement('div');
        header.style.cssText = 'font-size:12px;color:#8b949e;margin-bottom:16px;letter-spacing:1px;';
        header.textContent = '\u5de5\u827a\u8be6\u60c5';
        card.appendChild(header);

        var items = [
            { label: '\u5de5\u827a\u7c7b\u578b', value: inv.process_type_detailed || '-' },
            { label: '\u5e74\u4ee3\u4f30\u8ba1', value: inv.process_era_estimate || '-' }
        ];

        for (var i = 0; i < items.length; i++) {
            var row = document.createElement('div');
            row.style.cssText = 'display:flex;justify-content:space-between;align-items:center;padding:8px 0;border-bottom:1px solid #21262d;';

            var lbl = document.createElement('span');
            lbl.style.cssText = 'font-size:12px;color:#8b949e;';
            lbl.textContent = items[i].label;
            row.appendChild(lbl);

            var val = document.createElement('span');
            val.style.cssText = 'font-size:13px;color:#c9d1d9;';
            val.textContent = items[i].value;
            row.appendChild(val);
            card.appendChild(row);
        }

        var qRow = document.createElement('div');
        qRow.style.cssText = 'display:flex;justify-content:space-between;align-items:center;padding:10px 0;';

        var qLabel = document.createElement('span');
        qLabel.style.cssText = 'font-size:12px;color:#8b949e;';
        qLabel.textContent = '\u8d28\u91cf\u7b49\u7ea7';
        qRow.appendChild(qLabel);

        var qs = qualityStyle(inv.quality_level || '低');
        var badge = document.createElement('span');
        badge.style.cssText = 'font-size:12px;font-weight:600;padding:3px 12px;border-radius:12px;background:' + qs.bg + ';color:' + qs.color + ';border:1px solid ' + qs.border + ';';
        badge.textContent = inv.quality_level || '-';
        qRow.appendChild(badge);
        card.appendChild(qRow);

        container.appendChild(card);
    }

    function renderNetworkViz(container, data) {
        var card = document.createElement('div');
        card.style.cssText = 'background:#161b22;border:1px solid #30363d;border-radius:8px;padding:20px;margin-bottom:16px;';

        var header = document.createElement('div');
        header.style.cssText = 'font-size:12px;color:#8b949e;margin-bottom:12px;letter-spacing:1px;';
        header.textContent = 'BPNN\u7f51\u7edc\u7ed3\u6784';
        card.appendChild(header);

        var canvas = document.createElement('canvas');
        canvas.style.cssText = 'width:100%;height:300px;display:block;';
        card.appendChild(canvas);

        container.appendChild(card);

        setTimeout(function () {
            drawNetwork(canvas, data);
        }, 0);

        return canvas;
    }

    function renderFeatureTable(container, inv) {
        var features = inv.input_features || {};

        var card = document.createElement('div');
        card.style.cssText = 'background:#161b22;border:1px solid #30363d;border-radius:8px;padding:20px;margin-bottom:16px;';

        var header = document.createElement('div');
        header.style.cssText = 'font-size:12px;color:#8b949e;margin-bottom:12px;letter-spacing:1px;';
        header.textContent = '\u8f93\u5165\u7279\u5f81';
        card.appendChild(header);

        var table = document.createElement('table');
        table.style.cssText = 'width:100%;border-collapse:collapse;';

        var thead = document.createElement('thead');
        var tr = document.createElement('tr');
        tr.style.cssText = 'border-bottom:1px solid #30363d;';
        var cols = ['\u7279\u5f81', '\u503c', '\u6709\u6548'];
        for (var i = 0; i < cols.length; i++) {
            var th = document.createElement('th');
            th.style.cssText = 'font-size:11px;color:#8b949e;font-weight:400;text-align:left;padding:6px 8px;';
            th.textContent = cols[i];
            tr.appendChild(th);
        }
        thead.appendChild(tr);
        table.appendChild(thead);

        var tbody = document.createElement('tbody');
        for (var i = 0; i < INPUT_KEYS.length; i++) {
            var key = INPUT_KEYS[i];
            var val = features[key];
            var isValid = val !== undefined && val !== null && val > 0;

            var tr = document.createElement('tr');
            tr.style.cssText = 'border-bottom:1px solid #21262d;';

            var tdName = document.createElement('td');
            tdName.style.cssText = 'font-size:12px;color:#c9d1d9;padding:6px 8px;font-weight:500;';
            tdName.textContent = INPUT_LABELS[i];
            tr.appendChild(tdName);

            var tdVal = document.createElement('td');
            tdVal.style.cssText = 'font-size:12px;color:#8b949e;padding:6px 8px;font-family:monospace;';
            tdVal.textContent = val !== undefined && val !== null ? (typeof val === 'number' ? val.toFixed(4) : val) : '-';
            tr.appendChild(tdVal);

            var tdValid = document.createElement('td');
            tdValid.style.cssText = 'font-size:14px;padding:6px 8px;';
            var icon = document.createElement('span');
            icon.style.cssText = 'color:' + (isValid ? '#4caf50' : '#f44336') + ';';
            icon.textContent = isValid ? '\u2713' : '\u2717';
            tdValid.appendChild(icon);
            tr.appendChild(tdValid);

            tbody.appendChild(tr);
        }
        table.appendChild(tbody);
        card.appendChild(table);
        container.appendChild(card);
    }

    var ProcessInverter = {
        setAPIBase: function (url) { API_BASE = url; },

        load: function (siteId) {
            return fetch(API_BASE + '/sites/' + siteId + '/smelting-inversion')
                .then(function (r) { return r.json(); })
                .then(function (json) { return json.data || json; })
                .catch(function (e) {
                    console.error('ProcessInverter.load failed:', e);
                    return null;
                });
        },

        render: function (data, container) {
            if (!container) return;
            container.innerHTML = '';
            container.style.cssText = 'background:#0f1419;color:#c9d1d9;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;padding:16px;max-height:100%;overflow-y:auto;';

            if (!data) {
                var empty = document.createElement('div');
                empty.style.cssText = 'text-align:center;color:#8b949e;padding:40px 0;font-size:14px;';
                empty.textContent = '\u6682\u65e0\u51b6\u70bc\u5de5\u827a\u53cd\u6f14\u6570\u636e';
                container.appendChild(empty);
                return;
            }

            var inv = data.inversion || {};
            var histCanvas = renderTemperatureCard(container, inv);
            renderAgentBars(container, data);
            renderProcessDetails(container, inv);
            var netCanvas = renderNetworkViz(container, data);
            renderFeatureTable(container, inv);

            var dist = data.temperature_distribution || [];
            if (dist.length > 0 && histCanvas) {
                setTimeout(function () {
                    drawHistogram(histCanvas, dist, inv.estimated_temperature || 0);
                }, 50);
            }

            var resizeTimer;
            var handleResize = function () {
                clearTimeout(resizeTimer);
                resizeTimer = setTimeout(function () {
                    if (dist.length > 0 && histCanvas && histCanvas.parentElement) {
                        drawHistogram(histCanvas, dist, inv.estimated_temperature || 0);
                    }
                    if (netCanvas && netCanvas.parentElement) {
                        drawNetwork(netCanvas, data);
                    }
                }, 200);
            };
            window.addEventListener('resize', handleResize);
        }
    };

    global.ProcessInverter = ProcessInverter;

})(typeof window !== 'undefined' ? window : this);
