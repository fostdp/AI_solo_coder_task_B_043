(function (global) {
    'use strict';

    var API_BASE = '/api';

    var OXIDE_COLORS = {
        'SiO₂': '#58a6ff',
        'Al₂O₃': '#f78166',
        'CaO': '#7ee787',
        'FeO+Fe₂O₃': '#f85149',
        'MgO+MnO': '#d2a8ff',
        'SO₃': '#ffd33d',
        '其他': '#8b949e'
    };

    var SlagRecycle = {
        setAPIBase: function (url) { API_BASE = url; },
        load: function (siteId) {
            return fetch(API_BASE + '/sites/' + siteId + '/slag-recycle')
                .then(function (r) { return r.json(); })
                .then(function (d) { return d.data; });
        },

        render: function (data, container) {
            if (!data || !container) return;
            container.innerHTML = '';

            var wrapper = document.createElement('div');
            wrapper.className = 'slag-recycle-wrapper';

            wrapper.appendChild(renderCompositionPie(data));
            wrapper.appendChild(renderCementAssessment(data));
            wrapper.appendChild(renderRoadAssessment(data));
            wrapper.appendChild(renderLeachingRisk(data));
            wrapper.appendChild(renderRecommendation(data));

            container.appendChild(wrapper);
        }
    };

    function renderCompositionPie(data) {
        var comp = data.composition;
        if (!comp) return document.createElement('div');

        var section = mkSection('矿渣主要氧化物成分');
        var row = document.createElement('div');
        row.style.cssText = 'display:flex;align-items:center;gap:16px;flex-wrap:wrap;';

        var canvas = document.createElement('canvas');
        canvas.style.cssText = 'width:220px;height:220px;flex-shrink:0;';
        drawDonutPie(canvas, comp);

        var legend = document.createElement('div');
        legend.style.cssText = 'display:flex;flex-direction:column;gap:4px;font-size:11px;';
        var items = getOxideItems(comp);
        items.forEach(function (it) {
            var row2 = document.createElement('div');
            row2.style.cssText = 'display:flex;align-items:center;gap:6px;';
            var dot = document.createElement('span');
            dot.style.cssText = 'width:10px;height:10px;border-radius:2px;background:' + it.color + ';flex-shrink:0;';
            var txt = document.createElement('span');
            txt.style.color = '#c9d1d9';
            txt.textContent = it.name + '  ' + it.value.toFixed(2) + '%';
            row2.appendChild(dot);
            row2.appendChild(txt);
            legend.appendChild(row2);
        });

        row.appendChild(canvas);
        row.appendChild(legend);
        section.appendChild(row);
        return section;
    }

    function getOxideItems(comp) {
        var otherVal = 100;
        var list = [
            { name: 'SiO₂', value: comp.sio2 || 0, color: OXIDE_COLORS['SiO₂'] },
            { name: 'Al₂O₃', value: comp.al2o3 || 0, color: OXIDE_COLORS['Al₂O₃'] },
            { name: 'CaO', value: comp.cao || 0, color: OXIDE_COLORS['CaO'] },
            { name: 'FeO+Fe₂O₃', value: (comp.feo || 0) + (comp.fe2o3 || 0), color: OXIDE_COLORS['FeO+Fe₂O₃'] },
            { name: 'MgO+MnO', value: (comp.mgo || 0) + (comp.mno || 0), color: OXIDE_COLORS['MgO+MnO'] },
            { name: 'SO₃', value: comp.so3 || 0, color: OXIDE_COLORS['SO₃'] }
        ];
        list.forEach(function (it) { otherVal -= it.value; });
        otherVal = Math.max(0, otherVal);
        list.push({ name: '其他', value: otherVal, color: OXIDE_COLORS['其他'] });
        return list;
    }

    function drawDonutPie(canvas, comp) {
        var dpr = window.devicePixelRatio || 1;
        var w = 220, h = 220;
        canvas.width = w * dpr;
        canvas.height = h * dpr;
        canvas.style.width = w + 'px';
        canvas.style.height = h + 'px';
        var ctx = canvas.getContext('2d');
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

        var cx = w / 2, cy = h / 2;
        var outerR = 90, innerR = 50;
        var items = getOxideItems(comp);
        var total = 0;
        items.forEach(function (it) { total += it.value; });
        if (total <= 0) total = 1;

        var startAngle = -Math.PI / 2;
        items.forEach(function (it) {
            var sweep = (it.value / total) * Math.PI * 2;
            if (sweep <= 0) return;
            ctx.beginPath();
            ctx.moveTo(cx + innerR * Math.cos(startAngle), cy + innerR * Math.sin(startAngle));
            ctx.arc(cx, cy, outerR, startAngle, startAngle + sweep);
            ctx.arc(cx, cy, innerR, startAngle + sweep, startAngle, true);
            ctx.closePath();
            ctx.fillStyle = it.color;
            ctx.fill();
            ctx.strokeStyle = '#0d1117';
            ctx.lineWidth = 1.5;
            ctx.stroke();

            if (it.value / total > 0.04) {
                var midAngle = startAngle + sweep / 2;
                var labelR = (outerR + innerR) / 2;
                var lx = cx + labelR * Math.cos(midAngle);
                var ly = cy + labelR * Math.sin(midAngle);
                ctx.fillStyle = '#fff';
                ctx.font = 'bold 10px sans-serif';
                ctx.textAlign = 'center';
                ctx.textBaseline = 'middle';
                ctx.fillText((it.value / total * 100).toFixed(1) + '%', lx, ly);
            }
            startAngle += sweep;
        });

        ctx.fillStyle = '#c9d1d9';
        ctx.font = 'bold 13px sans-serif';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText('氧化物', cx, cy - 6);
        ctx.fillStyle = '#8b949e';
        ctx.font = '10px sans-serif';
        ctx.fillText('wt%', cx, cy + 10);
    }

    function renderCementAssessment(data) {
        var assessment = data.assessment;
        var checks = data.cement_checks;
        if (!assessment) return document.createElement('div');

        var section = mkSection('水泥混合材可行性评估');
        var header = document.createElement('div');
        header.style.cssText = 'display:flex;align-items:center;gap:16px;margin-bottom:12px;flex-wrap:wrap;';

        var feasTag = mkFeasibilityTag(assessment.cement_blended_feasibility);
        header.appendChild(feasTag);

        var scoreWrap = document.createElement('div');
        scoreWrap.style.cssText = 'display:flex;align-items:center;gap:8px;';
        var scoreCanvas = document.createElement('canvas');
        scoreCanvas.style.cssText = 'width:54px;height:54px;flex-shrink:0;';
        drawScoreRing(scoreCanvas, assessment.cement_blended_score || 0);
        scoreWrap.appendChild(scoreCanvas);
        var scoreLabel = document.createElement('span');
        scoreLabel.style.cssText = 'font-size:12px;color:#8b949e;';
        scoreLabel.textContent = '综合评分 ' + (assessment.cement_blended_score || 0).toFixed(1);
        scoreWrap.appendChild(scoreLabel);
        header.appendChild(scoreWrap);

        if (assessment.cement_blended_grade) {
            var gradeTag = document.createElement('span');
            gradeTag.style.cssText = 'padding:2px 10px;border-radius:10px;font-size:11px;font-weight:600;background:rgba(88,166,255,0.15);color:#58a6ff;';
            gradeTag.textContent = assessment.cement_blended_grade;
            header.appendChild(gradeTag);
        }

        section.appendChild(header);
        section.appendChild(renderCheckTable(checks, 'cement'));
        return section;
    }

    function renderRoadAssessment(data) {
        var assessment = data.assessment;
        var checks = data.road_checks;
        if (!assessment) return document.createElement('div');

        var section = mkSection('路基材料可行性评估');
        var header = document.createElement('div');
        header.style.cssText = 'display:flex;align-items:center;gap:16px;margin-bottom:12px;flex-wrap:wrap;';

        var feasTag = mkFeasibilityTag(assessment.road_base_feasibility);
        header.appendChild(feasTag);

        var scoreWrap = document.createElement('div');
        scoreWrap.style.cssText = 'display:flex;align-items:center;gap:8px;';
        var scoreCanvas = document.createElement('canvas');
        scoreCanvas.style.cssText = 'width:54px;height:54px;flex-shrink:0;';
        drawScoreRing(scoreCanvas, assessment.road_base_score || 0);
        scoreWrap.appendChild(scoreCanvas);
        var scoreLabel = document.createElement('span');
        scoreLabel.style.cssText = 'font-size:12px;color:#8b949e;';
        scoreLabel.textContent = '综合评分 ' + (assessment.road_base_score || 0).toFixed(1);
        scoreWrap.appendChild(scoreLabel);
        header.appendChild(scoreWrap);

        if (assessment.road_base_grade) {
            var gradeTag = document.createElement('span');
            gradeTag.style.cssText = 'padding:2px 10px;border-radius:10px;font-size:11px;font-weight:600;background:rgba(88,166,255,0.15);color:#58a6ff;';
            gradeTag.textContent = assessment.road_base_grade;
            header.appendChild(gradeTag);
        }

        section.appendChild(header);
        section.appendChild(renderCheckTable(checks, 'road'));
        return section;
    }

    function renderCheckTable(checks, type) {
        var table = document.createElement('table');
        table.className = 'slag-check-table';
        var thead = document.createElement('thead');
        thead.innerHTML = '<tr><th>项目</th><th>实测值</th><th>标准限值</th><th>达标</th></tr>';
        table.appendChild(thead);
        var tbody = document.createElement('tbody');
        if (!checks || !checks.length) {
            var tr = document.createElement('tr');
            var td = document.createElement('td');
            td.colSpan = 4;
            td.style.cssText = 'text-align:center;color:#8b949e;padding:12px;';
            td.textContent = '暂无数据';
            tr.appendChild(td);
            tbody.appendChild(tr);
        } else {
            checks.forEach(function (c) {
                var tr = document.createElement('tr');
                var itemName = document.createElement('td');
                itemName.textContent = c.item || '';
                itemName.style.fontWeight = '600';

                var valTd = document.createElement('td');
                valTd.textContent = formatCheckValue(c.value);

                var stdTd = document.createElement('td');
                stdTd.textContent = formatCheckValue(c.standard_limit);

                var passTd = document.createElement('td');
                passTd.style.textAlign = 'center';
                passTd.style.fontWeight = '700';
                if (c.pass) {
                    passTd.textContent = '✓';
                    passTd.style.color = '#3fb950';
                } else {
                    passTd.textContent = '✗';
                    passTd.style.color = '#f85149';
                }

                if (type === 'road' && c.grade) {
                    var gradeSpan = document.createElement('span');
                    gradeSpan.style.cssText = 'margin-left:4px;font-size:10px;color:#8b949e;font-weight:400;';
                    gradeSpan.textContent = '(' + c.grade + ')';
                    itemName.appendChild(gradeSpan);
                }

                if (!c.pass) {
                    tr.style.background = 'rgba(248,81,73,0.06)';
                }

                tr.appendChild(itemName);
                tr.appendChild(valTd);
                tr.appendChild(stdTd);
                tr.appendChild(passTd);
                tbody.appendChild(tr);
            });
        }
        table.appendChild(tbody);
        return table;
    }

    function renderLeachingRisk(data) {
        var assessment = data.assessment;
        if (!assessment) return document.createElement('div');
        var riskDetails = assessment.leaching_risk_details;
        var riskLevel = assessment.leaching_risk_level || '未知';

        var section = mkSection('环境浸出风险评估');

        var header = document.createElement('div');
        header.style.cssText = 'display:flex;align-items:center;gap:12px;margin-bottom:12px;flex-wrap:wrap;';

        var riskTag = mkRiskTag(riskLevel);
        header.appendChild(riskTag);

        if (riskDetails && riskDetails.standard) {
            var stdSpan = document.createElement('span');
            stdSpan.style.cssText = 'font-size:11px;color:#8b949e;';
            stdSpan.textContent = riskDetails.standard;
            header.appendChild(stdSpan);
        }
        section.appendChild(header);

        if (riskDetails && riskDetails.metal_results) {
            var canvas = document.createElement('canvas');
            canvas.style.cssText = 'width:100%;height:180px;display:block;margin-bottom:8px;';
            drawLeachingChart(canvas, riskDetails.metal_results);
            section.appendChild(canvas);

            var list = document.createElement('div');
            list.style.cssText = 'display:flex;flex-direction:column;gap:4px;';
            riskDetails.metal_results.forEach(function (mr) {
                var row = document.createElement('div');
                row.style.cssText = 'display:flex;align-items:center;gap:8px;font-size:12px;padding:4px 8px;border-radius:4px;';
                if (mr.exceed) {
                    row.style.background = 'rgba(248,81,73,0.12)';
                }
                var nameSpan = document.createElement('span');
                nameSpan.style.cssText = 'font-weight:600;width:28px;color:' + (mr.exceed ? '#f85149' : '#c9d1d9') + ';';
                nameSpan.textContent = mr.metal || '';
                var valSpan = document.createElement('span');
                valSpan.style.color = '#c9d1d9';
                valSpan.textContent = (mr.value || 0).toFixed(4) + ' mg/L';
                var vs = document.createElement('span');
                vs.style.color = '#484f58';
                vs.textContent = '/';
                var limSpan = document.createElement('span');
                limSpan.style.color = '#8b949e';
                limSpan.textContent = (mr.limit || 0).toFixed(4);
                var badge = document.createElement('span');
                if (mr.exceed) {
                    badge.style.cssText = 'margin-left:auto;font-size:10px;font-weight:700;color:#f85149;';
                    badge.textContent = '超标 ×' + (mr.exceed_ratio || 0).toFixed(2);
                } else {
                    badge.style.cssText = 'margin-left:auto;font-size:10px;font-weight:700;color:#3fb950;';
                    badge.textContent = '达标';
                }
                row.appendChild(nameSpan);
                row.appendChild(valSpan);
                row.appendChild(vs);
                row.appendChild(limSpan);
                row.appendChild(badge);
                list.appendChild(row);
            });
            section.appendChild(list);
        }

        return section;
    }

    function drawLeachingChart(canvas, metalResults) {
        var dpr = window.devicePixelRatio || 1;
        var w = canvas.clientWidth || 400;
        var h = 180;
        canvas.width = w * dpr;
        canvas.height = h * dpr;
        canvas.style.height = h + 'px';
        var ctx = canvas.getContext('2d');
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx.clearRect(0, 0, w, h);

        if (!metalResults || !metalResults.length) return;

        var left = 36, right = 16, top = 12, bottom = 24;
        var chartW = w - left - right;
        var chartH = h - top - bottom;
        var barCount = metalResults.length;
        var barGap = 8;
        var barW = Math.min(28, (chartW - barGap * (barCount + 1)) / barCount);
        var totalBarArea = barW * barCount + barGap * (barCount + 1);
        var offsetX = left + (chartW - totalBarArea) / 2 + barGap;

        var maxVal = 0;
        metalResults.forEach(function (mr) {
            if ((mr.value || 0) > maxVal) maxVal = mr.value;
            if ((mr.limit || 0) > maxVal) maxVal = mr.limit;
        });
        maxVal = maxVal * 1.3 || 1;

        metalResults.forEach(function (mr, i) {
            var x = offsetX + i * (barW + barGap);
            var barH = (mr.value / maxVal) * chartH;
            var y = top + chartH - barH;

            var isExceed = mr.exceed;
            ctx.fillStyle = isExceed ? 'rgba(248,81,73,0.15)' : '#58a6ff';
            if (isExceed) {
                ctx.fillRect(x - 2, top, barW + 4, chartH);
            }
            ctx.fillStyle = '#58a6ff';
            var r = Math.min(3, barW / 4);
            ctx.beginPath();
            ctx.moveTo(x, top + chartH);
            ctx.lineTo(x, y + r);
            ctx.quadraticCurveTo(x, y, x + r, y);
            ctx.lineTo(x + barW - r, y);
            ctx.quadraticCurveTo(x + barW, y, x + barW, y + r);
            ctx.lineTo(x + barW, top + chartH);
            ctx.closePath();
            ctx.fill();

            var limitY = top + chartH - (mr.limit / maxVal) * chartH;
            ctx.beginPath();
            ctx.strokeStyle = '#f85149';
            ctx.lineWidth = 1.5;
            ctx.setLineDash([3, 3]);
            ctx.moveTo(x - 4, limitY);
            ctx.lineTo(x + barW + 4, limitY);
            ctx.stroke();
            ctx.setLineDash([]);

            ctx.fillStyle = '#8b949e';
            ctx.font = '10px sans-serif';
            ctx.textAlign = 'center';
            ctx.textBaseline = 'top';
            ctx.fillText(mr.metal || '', x + barW / 2, top + chartH + 4);
        });
    }

    function renderRecommendation(data) {
        var assessment = data.assessment;
        var processFlow = data.process_flow;
        if (!assessment) return document.createElement('div');

        var plan = assessment.utilization_plan || {};
        var recommended = plan.recommended_use || assessment.recommended_use || '未确定';

        var section = mkSection('综合推荐方案');

        var recTag = document.createElement('div');
        recTag.style.cssText = 'font-size:18px;font-weight:700;color:#58a6ff;margin-bottom:12px;padding:10px 16px;background:rgba(88,166,255,0.1);border-radius:8px;border:1px solid rgba(88,166,255,0.2);';
        recTag.textContent = recommended;
        section.appendChild(recTag);

        if (plan.reasons && plan.reasons.length) {
            var reasonsDiv = document.createElement('div');
            reasonsDiv.style.cssText = 'margin-bottom:12px;font-size:12px;color:#8b949e;line-height:1.6;';
            plan.reasons.forEach(function (r) {
                var p = document.createElement('div');
                p.textContent = '• ' + r;
                reasonsDiv.appendChild(p);
            });
            section.appendChild(reasonsDiv);
        }

        if (plan.alternatives && plan.alternatives.length) {
            var altDiv = document.createElement('div');
            altDiv.style.cssText = 'margin-bottom:12px;font-size:12px;color:#8b949e;';
            var altLabel = document.createElement('span');
            altLabel.style.color = '#d29922';
            altLabel.style.fontWeight = '600';
            altLabel.textContent = '备选方案: ';
            altDiv.appendChild(altLabel);
            altDiv.appendChild(document.createTextNode(plan.alternatives.join('、')));
            section.appendChild(altDiv);
        }

        if (processFlow && processFlow.length) {
            var flowTitle = document.createElement('div');
            flowTitle.style.cssText = 'font-size:13px;font-weight:600;color:#c9d1d9;margin-bottom:8px;';
            flowTitle.textContent = '处理工艺流程';
            section.appendChild(flowTitle);

            var totalCost = 0;
            var flowList = document.createElement('div');
            flowList.style.cssText = 'display:flex;flex-direction:column;gap:6px;margin-bottom:12px;';
            processFlow.forEach(function (step) {
                totalCost += step.cost || 0;
                var row = document.createElement('div');
                row.style.cssText = 'display:flex;align-items:flex-start;gap:10px;padding:8px 10px;background:#161b22;border-radius:6px;border:1px solid #21262d;';

                var numSpan = document.createElement('span');
                numSpan.style.cssText = 'width:22px;height:22px;display:flex;align-items:center;justify-content:center;border-radius:50%;background:#58a6ff;color:#0d1117;font-size:11px;font-weight:700;flex-shrink:0;';
                numSpan.textContent = step.step;

                var descWrap = document.createElement('div');
                descWrap.style.cssText = 'flex:1;min-width:0;';

                var descLine = document.createElement('div');
                descLine.style.cssText = 'display:flex;align-items:center;gap:8px;';

                var descText = document.createElement('span');
                descText.style.cssText = 'font-size:13px;font-weight:600;color:#e6edf3;';
                descText.textContent = step.desc;
                descLine.appendChild(descText);

                var costTag = document.createElement('span');
                costTag.style.cssText = 'font-size:10px;padding:1px 6px;border-radius:8px;background:rgba(210,153,34,0.15);color:#d29922;flex-shrink:0;';
                costTag.textContent = (step.cost || 0).toFixed(0) + ' 元/吨';
                descLine.appendChild(costTag);

                descWrap.appendChild(descLine);

                if (step.note) {
                    var note = document.createElement('div');
                    note.style.cssText = 'font-size:11px;color:#8b949e;margin-top:2px;line-height:1.4;';
                    note.textContent = step.note;
                    descWrap.appendChild(note);
                }

                row.appendChild(numSpan);
                row.appendChild(descWrap);
                flowList.appendChild(row);
            });
            section.appendChild(flowList);

            var totalDiv = document.createElement('div');
            totalDiv.style.cssText = 'display:flex;align-items:center;gap:8px;padding:10px 14px;background:rgba(210,153,34,0.08);border:1px solid rgba(210,153,34,0.2);border-radius:8px;margin-bottom:12px;';
            var totalLabel = document.createElement('span');
            totalLabel.style.cssText = 'font-size:12px;color:#8b949e;';
            totalLabel.textContent = '总成本估算';
            var totalVal = document.createElement('span');
            totalVal.style.cssText = 'font-size:18px;font-weight:700;color:#d29922;margin-left:auto;';
            totalVal.textContent = totalCost.toFixed(0) + ' 元/吨';
            totalDiv.appendChild(totalLabel);
            totalDiv.appendChild(totalVal);
            section.appendChild(totalDiv);
        }

        var envBenefits = document.createElement('div');
        envBenefits.style.cssText = 'display:flex;gap:8px;flex-wrap:wrap;';
        var benefitItems = [
            { label: 'CO₂减排', icon: '🌿', color: '#3fb950' },
            { label: '资源节约', icon: '♻️', color: '#58a6ff' },
            { label: '填埋减量', icon: '📦', color: '#d2a8ff' }
        ];
        benefitItems.forEach(function (b) {
            var tag = document.createElement('span');
            tag.style.cssText = 'display:inline-flex;align-items:center;gap:4px;padding:4px 10px;border-radius:10px;font-size:11px;font-weight:600;background:' + b.color + '18;color:' + b.color + ';';
            tag.textContent = b.icon + ' ' + b.label;
            envBenefits.appendChild(tag);
        });
        section.appendChild(envBenefits);

        return section;
    }

    function drawScoreRing(canvas, score) {
        var dpr = window.devicePixelRatio || 1;
        var size = 54;
        canvas.width = size * dpr;
        canvas.height = size * dpr;
        canvas.style.width = size + 'px';
        canvas.style.height = size + 'px';
        var ctx = canvas.getContext('2d');
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

        var cx = size / 2, cy = size / 2, r = 22, lw = 5;
        score = Math.max(0, Math.min(100, score));

        ctx.beginPath();
        ctx.arc(cx, cy, r, 0, Math.PI * 2);
        ctx.strokeStyle = '#21262d';
        ctx.lineWidth = lw;
        ctx.stroke();

        var startAngle = -Math.PI / 2;
        var endAngle = startAngle + (score / 100) * Math.PI * 2;
        var color = score >= 85 ? '#3fb950' : score >= 60 ? '#d29922' : '#f85149';
        ctx.beginPath();
        ctx.arc(cx, cy, r, startAngle, endAngle);
        ctx.strokeStyle = color;
        ctx.lineWidth = lw;
        ctx.lineCap = 'round';
        ctx.stroke();

        ctx.fillStyle = color;
        ctx.font = 'bold 13px sans-serif';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(Math.round(score), cx, cy);
    }

    function mkSection(title) {
        var section = document.createElement('div');
        section.className = 'slag-section';
        var h = document.createElement('h4');
        h.className = 'slag-section-title';
        h.textContent = title;
        section.appendChild(h);
        return section;
    }

    function mkFeasibilityTag(feasibility) {
        var tag = document.createElement('span');
        tag.className = 'slag-feas-tag';
        if (feasibility === '可行') {
            tag.style.cssText = 'padding:4px 14px;border-radius:12px;font-size:13px;font-weight:700;background:rgba(63,185,80,0.2);color:#3fb950;';
        } else if (feasibility === '条件可行') {
            tag.style.cssText = 'padding:4px 14px;border-radius:12px;font-size:13px;font-weight:700;background:rgba(210,153,34,0.2);color:#d29922;';
        } else {
            tag.style.cssText = 'padding:4px 14px;border-radius:12px;font-size:13px;font-weight:700;background:rgba(248,81,73,0.2);color:#f85149;';
        }
        tag.textContent = feasibility || '未知';
        return tag;
    }

    function mkRiskTag(level) {
        var tag = document.createElement('span');
        tag.className = 'slag-risk-tag';
        var map = {
            '低风险': { bg: 'rgba(63,185,80,0.2)', color: '#3fb950' },
            '中风险': { bg: 'rgba(210,153,34,0.2)', color: '#d29922' },
            '高风险': { bg: 'rgba(255,152,0,0.2)', color: '#ff9800' },
            '极高风险': { bg: 'rgba(248,81,73,0.25)', color: '#f85149' }
        };
        var style = map[level] || { bg: 'rgba(139,148,158,0.2)', color: '#8b949e' };
        tag.style.cssText = 'padding:4px 14px;border-radius:12px;font-size:13px;font-weight:700;background:' + style.bg + ';color:' + style.color + ';';
        tag.textContent = level || '未知';
        return tag;
    }

    function formatCheckValue(v) {
        if (v === null || v === undefined) return '-';
        if (Math.abs(v) >= 100) return v.toFixed(1);
        if (Math.abs(v) >= 1) return v.toFixed(2);
        return v.toFixed(3);
    }

    global.SlagRecycle = SlagRecycle;

})(typeof window !== 'undefined' ? window : this);
