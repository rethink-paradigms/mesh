const DATA = {
    agents: [
        {
            id: 'researcher',
            name: 'researcher',
            image: 'team/researcher:v3',
            node: 'mesh-leader',
            memory: { used: 312, total: 512 },
            status: 'running',
            snapshots: 2,
            uptime: '47h 23m',
            replicas: [
                { id: 'researcher-r1', status: 'running' },
                { id: 'researcher-r2', status: 'idle' }
            ],
            conversations: 12,
            lastAction: 'search "lightweight orchestration"',
            cpu: 14
        },
        {
            id: 'plotcode',
            name: 'plotcode',
            image: 'plotcode/agent:v1.4',
            node: 'mesh-worker-1',
            memory: { used: 198, total: 256 },
            status: 'running',
            snapshots: 0,
            uptime: '47h 23m',
            replicas: [],
            conversations: 8,
            lastAction: 'plot bar quarterly-revenue',
            cpu: 9
        },
        {
            id: 'code-writer',
            name: 'code-writer',
            image: 'team/code-writer:v2',
            node: 'mesh-worker-1',
            memory: { used: 224, total: 384 },
            status: 'running',
            snapshots: 1,
            uptime: '31h 10m',
            replicas: [
                { id: 'code-writer-r1', status: 'running' }
            ],
            conversations: 23,
            lastAction: 'refactor api/handlers.go',
            cpu: 18
        },
        {
            id: 'sentinel',
            name: 'sentinel',
            image: 'mesh/sentinel:v1',
            node: 'mesh-worker-2',
            memory: { used: 87, total: 256 },
            status: 'running',
            snapshots: 3,
            uptime: '47h 23m',
            replicas: [
                { id: 'sentinel-r1', status: 'running' },
                { id: 'sentinel-r2', status: 'running' },
                { id: 'sentinel-r3', status: 'idle' }
            ],
            conversations: 0,
            lastAction: 'watch — monitoring cluster health',
            cpu: 3
        }
    ],
    nodes: [
        {
            id: 'mesh-leader',
            name: 'mesh-leader',
            ip: '100.64.0.1',
            role: 'leader',
            ram: { used: 1.2, total: 2 },
            cpu: 34,
            disk: 28,
            memory: '2 GB'
        },
        {
            id: 'mesh-worker-1',
            name: 'mesh-worker-1',
            ip: '100.64.0.2',
            role: 'worker',
            ram: { used: 1.4, total: 2 },
            cpu: 58,
            disk: 45,
            memory: '2 GB'
        },
        {
            id: 'mesh-worker-2',
            name: 'mesh-worker-2',
            ip: '100.64.0.3',
            role: 'worker',
            ram: { used: 0.7, total: 4 },
            cpu: 12,
            disk: 18,
            memory: '4 GB'
        }
    ]
};

let activeTermAgent = null;
let termContent = {};

function $(sel) { return document.querySelector(sel); }
function $$(sel) { return document.querySelectorAll(sel); }

function switchView(view) {
    $$('.nav-link').forEach(l => l.classList.toggle('active', l.dataset.view === view));
    $$('.view').forEach(v => v.classList.remove('active'));
    const target = $(`#view-${view}`);
    if (target) target.classList.add('active');
    if (view === 'agents') renderAgents();
    if (view === 'distribution') renderDistribution();
    if (view === 'performance') renderPerformance();
    if (view === 'terminal') renderTerminal();
}

function showToast(msg, type = 'ok') {
    const rack = $('#toastRack');
    const t = document.createElement('div');
    t.className = `toast ${type}`;
    t.textContent = msg;
    rack.appendChild(t);
    setTimeout(() => { if (t.parentNode) t.remove(); }, 3000);
}

function barClass(pct) {
    if (pct >= 75) return 'high';
    if (pct >= 45) return 'med';
    return 'low';
}

function agentInitial(name) {
    return name.charAt(0).toUpperCase();
}

function totalReplicas() {
    return DATA.agents.reduce((sum, a) => sum + a.replicas.length, 0);
}

function totalSnapshots() {
    return DATA.agents.reduce((sum, a) => sum + a.snapshots, 0);
}

function agentsOnNode(nodeId) {
    return DATA.agents.filter(a => a.node === nodeId);
}

function memPct(agent) {
    return Math.round((agent.memory.used / agent.memory.total) * 100);
}

/* ========== AGENTS VIEW ========== */

function renderAgents() {
    renderAgentsSummary();
    renderAgentsGrid();
}

function renderAgentsSummary() {
    const el = $('#agentsSummary');
    const running = DATA.agents.filter(a => a.status === 'running').length;
    const replicas = totalReplicas();
    const snapshots = totalSnapshots();
    const convos = DATA.agents.reduce((s, a) => s + a.conversations, 0);

    el.innerHTML = `
        <div class="summary-card">
            <span class="summary-value orange">${running}</span>
            <span class="summary-label">agents running</span>
        </div>
        <div class="summary-card">
            <span class="summary-value">${replicas}</span>
            <span class="summary-label">replicas</span>
        </div>
        <div class="summary-card">
            <span class="summary-value">${snapshots}</span>
            <span class="summary-label">snapshots</span>
        </div>
        <div class="summary-card">
            <span class="summary-value green">${convos}</span>
            <span class="summary-label">conversations</span>
        </div>
    `;
}

function renderAgentsGrid() {
    const el = $('#agentsGrid');
    el.innerHTML = DATA.agents.map(a => `
        <div class="agent-card" onclick="switchView('terminal'); setTimeout(() => selectTermAgent('${a.id}'), 100)">
            <div class="agent-card-header">
                <div class="agent-card-name">
                    <div class="agent-icon">${agentInitial(a.name)}</div>
                    <span class="agent-name">${a.name}</span>
                </div>
                <div class="agent-status-dot"></div>
            </div>
            <div class="agent-card-body">
                <div class="agent-stat">
                    <span class="agent-stat-label">memory</span>
                    <span class="agent-stat-value">${a.memory.used} / ${a.memory.total} MB</span>
                </div>
                <div class="agent-stat">
                    <span class="agent-stat-label">cpu</span>
                    <span class="agent-stat-value">${a.cpu}%</span>
                </div>
                <div class="agent-stat">
                    <span class="agent-stat-label">uptime</span>
                    <span class="agent-stat-value">${a.uptime}</span>
                </div>
                <div class="agent-stat">
                    <span class="agent-stat-label">last action</span>
                    <span class="agent-stat-value" style="font-size:0.72rem">${a.lastAction}</span>
                </div>
            </div>
            <div class="agent-card-footer">
                <span class="agent-node-tag">${a.node}</span>
                ${a.snapshots > 0 ? `<span class="agent-snapshot-tag">${a.snapshots} snapshot${a.snapshots > 1 ? 's' : ''}</span>` : ''}
            </div>
            ${a.replicas.length > 0 ? `
                <div class="agent-replicas">
                    <div class="agent-replicas-label">${a.replicas.length} replica${a.replicas.length > 1 ? 's' : ''}</div>
                    <div class="replica-pills">
                        ${a.replicas.map(r => `<span class="replica-pill ${r.status === 'idle' ? 'idle' : ''}">${r.id}</span>`).join('')}
                    </div>
                </div>
            ` : ''}
        </div>
    `).join('');
}

/* ========== DISTRIBUTION VIEW ========== */

function renderDistribution() {
    const el = $('#distGrid');
    el.innerHTML = DATA.nodes.map(n => {
        const agents = agentsOnNode(n.id);
        return `
            <div class="dist-node">
                <div class="dist-node-header">
                    <div class="dist-node-name">
                        <div class="dist-node-dot ${n.role}"></div>
                        <span class="dist-node-title">${n.name}</span>
                        <span class="dist-node-meta">${n.ip} · ${n.memory}</span>
                    </div>
                    <span class="dist-node-role ${n.role}">${n.role}</span>
                </div>
                <div class="dist-agents-row">
                    <div class="dist-agents-label">${agents.length} agent${agents.length !== 1 ? 's' : ''} scheduled</div>
                    ${agents.length > 0 ? agents.map(a => `
                        <div class="dist-agent-item">
                            <div class="dist-agent-left">
                                <div class="dist-agent-icon">${agentInitial(a.name)}</div>
                                <div>
                                    <div class="dist-agent-name">${a.name}</div>
                                    <div class="dist-agent-replicas">${a.replicas.length > 0 ? a.replicas.length + ' replicas' : 'no replicas'}</div>
                                </div>
                            </div>
                            <span class="dist-agent-memory">${a.memory.used} / ${a.memory.total} MB</span>
                        </div>
                    `).join('') : '<div class="dist-empty">no agents on this node</div>'}
                </div>
            </div>
        `;
    }).join('');
}

/* ========== PERFORMANCE VIEW ========== */

function renderPerformance() {
    const el = $('#perfGrid');
    el.innerHTML = DATA.nodes.map(n => {
        const agents = agentsOnNode(n.id);
        const ramPct = Math.round((n.ram.used / n.ram.total) * 100);
        return `
            <div class="perf-card">
                <div class="perf-card-header">
                    <span class="perf-card-name">${n.name}</span>
                    <span class="perf-card-role ${n.role}">${n.role}</span>
                </div>
                <div class="perf-metric">
                    <div class="perf-metric-header">
                        <span class="perf-metric-label">cpu</span>
                        <span class="perf-metric-value">${n.cpu}%</span>
                    </div>
                    <div class="perf-bar"><div class="perf-bar-fill ${barClass(n.cpu)}" data-w="${n.cpu}%"></div></div>
                </div>
                <div class="perf-metric">
                    <div class="perf-metric-header">
                        <span class="perf-metric-label">ram</span>
                        <span class="perf-metric-value">${n.ram.used} / ${n.ram.total} GB</span>
                    </div>
                    <div class="perf-bar"><div class="perf-bar-fill ${barClass(ramPct)}" data-w="${ramPct}%"></div></div>
                </div>
                <div class="perf-metric">
                    <div class="perf-metric-header">
                        <span class="perf-metric-label">disk</span>
                        <span class="perf-metric-value">${n.disk}%</span>
                    </div>
                    <div class="perf-bar"><div class="perf-bar-fill ${barClass(n.disk)}" data-w="${n.disk}%"></div></div>
                </div>
                <div class="perf-agents">
                    ${agents.length > 0 ? agents.map(a => `${agentInitial(a.name)} ${a.name} (${a.memory.used}MB)`).join(' · ') : 'no agents'}
                </div>
            </div>
        `;
    }).join('');

    setTimeout(() => {
        $$('.perf-bar-fill').forEach(b => { b.style.width = b.dataset.w; });
    }, 60);
}

/* ========== TERMINAL VIEW ========== */

function renderTerminal() {
    const list = $('#termAgentList');
    list.innerHTML = DATA.agents.map(a => `
        <div class="term-agent-item ${activeTermAgent === a.id ? 'active' : ''}" onclick="selectTermAgent('${a.id}')">
            <div class="term-agent-dot"></div>
            <div>
                <div class="term-agent-item-name">${a.name}</div>
                <div class="term-agent-item-mem">${a.memory.used}/${a.memory.total}MB</div>
            </div>
        </div>
    `).join('');
}

function selectTermAgent(agentId) {
    activeTermAgent = agentId;
    renderTerminal();

    const agent = DATA.agents.find(a => a.id === agentId);
    $('#termBarLabel').textContent = agent ? agent.name : '';

    const body = $('#termBody');

    if (!termContent[agentId]) {
        termContent[agentId] = buildWelcome(agent);
    }

    body.innerHTML = termContent[agentId];
    body.scrollTop = body.scrollHeight;
}

function buildWelcome(agent) {
    return `<span class="term-out">  ${agent.name} — ${agent.image}</span>\n` +
        `<span class="term-out">  node: ${agent.node} | mem: ${agent.memory.used}/${agent.memory.total}MB | uptime: ${agent.uptime}</span>\n` +
        `<span class="term-out">  type a command below. this is your agent.</span>\n\n`;
}

function termPrint(html) {
    if (!activeTermAgent) return;
    termContent[activeTermAgent] += html + '\n';
    const body = $('#termBody');
    body.innerHTML = termContent[activeTermAgent];
    body.scrollTop = body.scrollHeight;
}

function clearTerm() {
    if (!activeTermAgent) return;
    const agent = DATA.agents.find(a => a.id === activeTermAgent);
    termContent[activeTermAgent] = buildWelcome(agent);
    $('#termBody').innerHTML = termContent[activeTermAgent];
    showToast('terminal cleared');
}

/* ========== GUIDED DEMO ========== */

let demoRunning = false;

function playGuidedDemo() {
    if (demoRunning) return;
    demoRunning = true;

    const overlay = $('#demoOverlay');
    overlay.classList.add('active');

    const progress = $('#demoProgress');
    const step = $('#demoStep');

    function setProgress(pct, text) {
        progress.style.width = pct + '%';
        step.textContent = text;
    }

    function wait(ms) {
        return new Promise(r => setTimeout(r, ms));
    }

    async function run() {
        try {
            setProgress(5, 'opening cluster...');
            await wait(800);

            setProgress(15, 'viewing agents');
            switchView('agents');
            await wait(1200);

            setProgress(30, 'inspecting researcher');
            await wait(600);
            showToast('researcher — 2 replicas, persistent since 47h', 'info');
            await wait(1500);

            setProgress(45, 'checking distribution');
            switchView('distribution');
            await wait(1200);

            setProgress(55, 'agents across nodes');
            showToast('4 agents across 3 nodes — all on your hardware', 'info');
            await wait(1500);

            setProgress(65, 'opening terminal');
            switchView('terminal');
            await wait(600);

            selectTermAgent('researcher');
            setProgress(72, 'researcher terminal');
            await wait(600);

            termPrint(`<span class="term-prompt">research&gt;</span> <span class="term-cmd">search "lightweight container orchestration"</span>`);
            await wait(800);

            termPrint(`<span class="term-out">  searching across arxiv, github, web...</span>`);
            await wait(600);

            termPrint(`<span class="term-ok">  ✓ 3 results found</span>`);
            termPrint(`<span class="term-out">  1. Nomad: A next-generation cluster manager (relevance: 0.94)</span>`);
            termPrint(`<span class="term-out">  2. Lightweight Container Orchestration at Scale (relevance: 0.89)</span>`);
            await wait(800);

            setProgress(82, 'snapshotting agent');
            termPrint(`\n<span class="term-prompt">research&gt;</span> <span class="term-cmd">snapshot --tag pre-experiment</span>`);
            await wait(600);
            termPrint(`<span class="term-warn">  📸 committing container filesystem...</span>`);
            await wait(500);
            termPrint(`<span class="term-ok">  ✓ snapshot saved: researcher:pre-experiment</span>`);
            await wait(600);

            setProgress(90, 'switching agent');
            selectTermAgent('sentinel');
            await wait(400);

            termPrint(`<span class="term-prompt">sentinel&gt;</span> <span class="term-cmd">status</span>`);
            await wait(400);
            termPrint(`<span class="term-ok">  ● sentinel — running</span>`);
            termPrint(`<span class="term-out">  replicas: 3 (2 active, 1 idle)</span>`);
            termPrint(`<span class="term-out">  watching cluster health · 47h uptime</span>`);
            await wait(800);

            setProgress(100, 'done');
            switchView('agents');
            showToast('guided demo complete — explore freely', 'ok');
            await wait(1500);

        } finally {
            overlay.classList.remove('active');
            demoRunning = false;
        }
    }

    run();
}

/* ========== AUTO DEMO (?demo=1) ========== */

function autoStartDemo() {
    demoRunning = true;

    var overlay = document.getElementById("demoOverlay");
    var progress = document.getElementById("demoProgress");
    var stepEl = document.getElementById("demoStep");
    var transScreen = document.getElementById("demoTransitionScreen");

    function setP(pct, text) {
        progress.style.width = pct + "%";
        stepEl.textContent = text;
    }

    function wait(ms) {
        return new Promise(function (r) { setTimeout(r, ms); });
    }

    setTimeout(function () {
        if (transScreen) {
            transScreen.style.transition = "opacity 0.5s ease";
            transScreen.style.opacity = "0";
        }

        setTimeout(function () {
            if (transScreen) transScreen.style.display = "none";

            overlay.classList.add("active");

            (async function run() {
                try {
                    setP(5, "opening dashboard...");
                    await wait(1500);

                    setP(15, "viewing agents");
                    switchView("agents");
                    await wait(2000);

                    setP(25, "inspecting researcher");
                    showToast("researcher — 2 replicas, persistent since 47h", "info");
                    await wait(1500);

                    setP(40, "checking distribution");
                    switchView("distribution");
                    await wait(1500);

                    setP(50, "agents across nodes");
                    showToast("4 agents across 3 nodes — all on your hardware", "info");
                    await wait(1000);

                    setP(60, "opening terminal");
                    switchView("terminal");
                    await wait(800);

                    selectTermAgent("researcher");
                    setP(68, "researcher terminal");
                    await wait(500);

                    termPrint('<span class="term-prompt">researcher&gt;</span> <span class="term-cmd">search "lightweight orchestration"</span>');
                    await wait(800);
                    termPrint('<span class="term-out">  searching across arxiv, github, web...</span>');
                    await wait(700);
                    termPrint('<span class="term-ok">  \u2713 3 results found</span>');
                    await wait(500);

                    setP(78, "snapshotting agent");
                    termPrint('\n<span class="term-prompt">researcher&gt;</span> <span class="term-cmd">snapshot --tag pre-experiment</span>');
                    await wait(600);
                    termPrint('<span class="term-warn">  \uD83D\uDCF8 committing container filesystem...</span>');
                    await wait(500);
                    termPrint('<span class="term-ok">  \u2713 snapshot saved: researcher:pre-experiment</span>');
                    await wait(800);

                    setP(88, "checking sentinel");
                    selectTermAgent("sentinel");
                    await wait(500);

                    termPrint('<span class="term-prompt">sentinel&gt;</span> <span class="term-cmd">status</span>');
                    await wait(500);
                    termPrint('<span class="term-ok">  \u25CF sentinel \u2014 running</span>');
                    termPrint('<span class="term-out">  replicas: 3 (2 active, 1 idle)</span>');
                    await wait(800);

                    setP(100, "done");
                    switchView("agents");
                    showToast("guided demo complete — explore freely", "ok");
                    await wait(1200);
                } finally {
                    overlay.classList.remove("active");
                    demoRunning = false;
                    if (window.history && window.history.replaceState) {
                        window.history.replaceState({}, "", "cluster.html");
                    }
                }
            })();
        }, 500);
    }, 500);
}

/* ========== INIT ========== */

document.addEventListener('DOMContentLoaded', () => {
    renderAgents();
    if (new URLSearchParams(window.location.search).has('demo')) {
        autoStartDemo();
    }
});
