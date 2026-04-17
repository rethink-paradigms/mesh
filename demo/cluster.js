(function () {
  'use strict';

  var nodes = [
    { id: 'leader', label: 'leader', type: 'leader', ip: '100.64.0.1', status: 'ready', apps: 2, cpu: 23, mem: 45, region: 'us-east-1', provider: 'AWS' },
    { id: 'worker-1', label: 'worker-1', type: 'worker', ip: '100.64.0.2', status: 'ready', apps: 3, cpu: 61, mem: 72, region: 'nyc3', provider: 'DigitalOcean' },
    { id: 'worker-2', label: 'worker-2', type: 'worker', ip: '100.64.0.3', status: 'ready', apps: 1, cpu: 15, mem: 34, region: 'fsn1', provider: 'Hetzner' },
    { id: 'worker-3', label: 'worker-3', type: 'worker', ip: '100.64.0.4', status: 'pending', apps: 0, cpu: 0, mem: 0, region: 'eu-west-1', provider: 'AWS' }
  ];

  var edges = [
    { from: 'leader', to: 'worker-1', latency: '12ms' },
    { from: 'leader', to: 'worker-2', latency: '85ms' },
    { from: 'leader', to: 'worker-3', latency: '45ms' },
    { from: 'worker-1', to: 'worker-2', latency: '92ms' }
  ];

  function drawCluster() {
    var canvas = document.getElementById('clusterCanvas');
    if (!canvas) return;
    var container = canvas.parentElement;
    var rect = container.getBoundingClientRect();
    var dpr = window.devicePixelRatio || 1;

    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;
    canvas.style.width = rect.width + 'px';
    canvas.style.height = rect.height + 'px';

    var ctx = canvas.getContext('2d');
    ctx.scale(dpr, dpr);

    var w = rect.width;
    var h = rect.height;

    var cx = w / 2;
    var cy = h / 2;
    var radius = Math.min(w, h) * 0.3;

    var positions = {};
    positions['leader'] = { x: cx, y: cy };

    var workers = nodes.filter(function (n) { return n.type === 'worker'; });
    workers.forEach(function (node, i) {
      var angle = -Math.PI / 2 + (2 * Math.PI * i) / workers.length;
      positions[node.id] = {
        x: cx + radius * Math.cos(angle),
        y: cy + radius * Math.sin(angle)
      };
    });

    ctx.clearRect(0, 0, w, h);

    edges.forEach(function (edge) {
      var from = positions[edge.from];
      var to = positions[edge.to];
      if (!from || !to) return;

      ctx.beginPath();
      ctx.moveTo(from.x, from.y);
      ctx.lineTo(to.x, to.y);
      ctx.strokeStyle = '#2a2a42';
      ctx.lineWidth = 1.5;
      ctx.setLineDash([6, 4]);
      ctx.stroke();
      ctx.setLineDash([]);

      var midX = (from.x + to.x) / 2;
      var midY = (from.y + to.y) / 2;
      ctx.font = '11px "JetBrains Mono", monospace';
      ctx.fillStyle = '#64748b';
      ctx.textAlign = 'center';
      ctx.fillText(edge.latency, midX, midY - 6);
    });

    nodes.forEach(function (node) {
      var pos = positions[node.id];
      if (!pos) return;

      var isLeader = node.type === 'leader';
      var nodeRadius = isLeader ? 36 : 28;
      var color = isLeader ? '#3b82f6' : '#06b6d4';

      if (node.status === 'pending') {
        color = '#f59e0b';
      }

      ctx.beginPath();
      ctx.arc(pos.x, pos.y, nodeRadius + 8, 0, Math.PI * 2);
      var glow = ctx.createRadialGradient(pos.x, pos.y, nodeRadius, pos.x, pos.y, nodeRadius + 8);
      glow.addColorStop(0, color + '30');
      glow.addColorStop(1, color + '00');
      ctx.fillStyle = glow;
      ctx.fill();

      ctx.beginPath();
      ctx.arc(pos.x, pos.y, nodeRadius, 0, Math.PI * 2);
      ctx.fillStyle = '#12121e';
      ctx.fill();
      ctx.strokeStyle = color;
      ctx.lineWidth = 2;
      ctx.stroke();

      ctx.font = (isLeader ? 'bold 13px' : '12px') + ' "Inter", sans-serif';
      ctx.fillStyle = '#e2e8f0';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(node.label, pos.x, pos.y - 2);

      ctx.font = '10px "JetBrains Mono", monospace';
      ctx.fillStyle = '#64748b';
      ctx.fillText(node.provider, pos.x, pos.y + 12);

      if (node.status === 'ready') {
        ctx.beginPath();
        ctx.arc(pos.x + nodeRadius - 4, pos.y - nodeRadius + 4, 5, 0, Math.PI * 2);
        ctx.fillStyle = '#10b981';
        ctx.fill();
        ctx.strokeStyle = '#12121e';
        ctx.lineWidth = 2;
        ctx.stroke();
      } else if (node.status === 'pending') {
        ctx.beginPath();
        ctx.arc(pos.x + nodeRadius - 4, pos.y - nodeRadius + 4, 5, 0, Math.PI * 2);
        ctx.fillStyle = '#f59e0b';
        ctx.fill();
        ctx.strokeStyle = '#12121e';
        ctx.lineWidth = 2;
        ctx.stroke();
      }
    });
  }

  function renderNodeList() {
    var container = document.getElementById('nodeListBody');
    if (!container) return;

    container.innerHTML = '';
    nodes.forEach(function (node) {
      var statusClass = node.status === 'ready' ? 'status-ready' : node.status === 'pending' ? 'status-pending' : 'status-unhealthy';
      var dotColor = node.type === 'leader' ? 'var(--node-leader)' : node.status === 'pending' ? 'var(--node-pending)' : 'var(--node-worker)';

      var row = document.createElement('div');
      row.className = 'node-list-row';
      row.innerHTML =
        '<div class="node-name">' +
          '<span class="node-name-dot" style="background:' + dotColor + '"></span>' +
          node.label +
          (node.type === 'leader' ? '<span class="node-role" style="background:var(--accent-blue-glow);color:var(--accent-blue);">leader</span>' : '') +
        '</div>' +
        '<div style="font-family:var(--font-mono);font-size:0.8rem;color:var(--text-muted);">' + node.ip + '</div>' +
        '<div class="' + statusClass + '" style="font-weight:600;font-size:0.8rem;">' + node.status + '</div>' +
        '<div style="font-weight:500;">' + node.apps + '</div>' +
        '<div>' +
          '<div style="display:flex;align-items:center;gap:8px;">' +
            '<div style="flex:1;height:4px;background:var(--bg-elevated);border-radius:2px;overflow:hidden;">' +
              '<div style="width:' + node.cpu + '%;height:100%;background:' + (node.cpu > 70 ? 'var(--accent-red)' : node.cpu > 40 ? 'var(--accent-yellow)' : 'var(--accent-green)') + ';border-radius:2px;"></div>' +
            '</div>' +
            '<span style="font-size:0.75rem;color:var(--text-muted);font-family:var(--font-mono);">' + node.cpu + '%</span>' +
          '</div>' +
        '</div>';
      container.appendChild(row);
    });
  }

  window.addEventListener('DOMContentLoaded', function () {
    drawCluster();
    renderNodeList();
  });

  var resizeTimer;
  window.addEventListener('resize', function () {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(drawCluster, 150);
  });
})();
