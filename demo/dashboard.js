/* ============================================================
   MESH — Dashboard Demo Logic
   Mock data, routing, view management, and UI interactions
   ============================================================ */

// ---- Mock Data ----
const MOCK_DATA = {
    nodes: [
        {
            id: 'leader',
            name: 'mesh-leader',
            ip: '100.64.0.1',
            role: 'leader',
            cpu: 34,
            ram: 62,
            disk: 28,
            memory: '2 GB',
            agents: ['gemini-cli'],
            uptime: '47h 23m'
        },
        {
            id: 'worker-1',
            name: 'mesh-worker-1',
            ip: '100.64.0.2',
            role: 'worker',
            cpu: 58,
            ram: 71,
            disk: 45,
            memory: '2 GB',
            agents: ['plotcode', 'researcher'],
            uptime: '47h 23m'
        },
        {
            id: 'worker-2',
            name: 'mesh-worker-2',
            ip: '100.64.0.3',
            role: 'worker',
            cpu: 12,
            ram: 35,
            disk: 18,
            memory: '4 GB',
            agents: [],
            uptime: '47h 23m'
        }
    ],

    agents: [
        {
            id: 'gemini-cli',
            name: 'Gemini CLI',
            image: 'gemini/cli:v2.1',
            node: 'mesh-leader',
            memory: '312 MB / 512 MB',
            status: 'running',
            snapshots: 1,
            icon: '\u{1F916}',
            color: 'cyan'
        },
        {
            id: 'plotcode',
            name: 'Plotcode',
            image: 'plotcode/agent:v1.4',
            node: 'mesh-worker-1',
            memory: '198 MB / 256 MB',
            status: 'running',
            snapshots: 0,
            icon: '\u{1F4CA}',
            color: 'purple'
        },
        {
            id: 'researcher',
            name: 'Researcher',
            image: 'team/researcher:v3.0',
            node: 'mesh-worker-1',
            memory: '245 MB / 384 MB',
            status: 'running',
            snapshots: 1,
            icon: '\u{1F50D}',
            color: 'yellow'
        }
    ],

    inviteTokens: ['mesh-demo-2026', 'mesh-invite-abc123', 'mesh-admin-token']
};

// ---- State ----
let currentView = 'login';
let activeAgent = null;
let terminalInstances = {};  // agentId -> { xterm, fitAddon, mock }
let guidedDemoRunning = false;

// ---- Toast System ----
function showToast(message, type = 'info') {
    const container = document.getElementById('toastContainer');
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    container.appendChild(toast);

    setTimeout(() => {
        if (toast.parentNode) toast.remove();
    }, 3000);
}

// ---- Login ----
function handleLogin() {
    const input = document.getElementById('inviteToken');
    const loading = document.getElementById('loginLoading');
    const error = document.getElementById('loginError');
    const token = input.value.trim();

    error.classList.remove('active');

    if (!token) {
        error.textContent = 'Please enter an invite token.';
        error.classList.add('active');
        return;
    }

    if (!MOCK_DATA.inviteTokens.includes(token)) {
        error.textContent = 'Invalid invite token. Please try again.';
        error.classList.add('active');
        return;
    }

    // Show loading
    loading.classList.add('active');
    document.getElementById('loginBtn').disabled = true;

    setTimeout(() => {
        loading.classList.remove('active');
        document.getElementById('view-login').classList.add('hidden');
        document.getElementById('appShell').classList.add('active');
        currentView = 'dashboard';
        renderDashboard();
        showToast('Connected to agent-mesh cluster', 'success');
    }, 1500);
}

// Allow Enter key on login input
document.addEventListener('DOMContentLoaded', () => {
    const input = document.getElementById('inviteToken');
    if (input) {
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') handleLogin();
        });
        input.focus();
    }
});

// ---- Navigation ----
function navigateTo(view) {
    // Update sidebar active state
    document.querySelectorAll('.nav-item').forEach(item => {
        item.classList.toggle('active', item.dataset.view === view);
    });

    // Hide all views
    document.querySelectorAll('.view-content').forEach(v => v.classList.remove('active'));

    // Show target view
    const target = document.getElementById(`view-${view}`);
    if (target) {
        target.classList.add('active');
    }

    currentView = view;

    // Render view-specific content
    if (view === 'dashboard') {
        renderDashboard();
    } else if (view === 'agents') {
        renderAgentsView();
    } else if (view === 'topology') {
        renderTopology();
    }
}

// ---- Dashboard View ----
function renderDashboard() {
    renderNodeGrid();
    renderAgentGrid();
    animateMetricBars();
}

function renderNodeGrid() {
    const grid = document.getElementById('nodeGrid');
    if (!grid) return;

    grid.innerHTML = MOCK_DATA.nodes.map(node => `
        <div class="node-card ${node.role === 'leader' ? 'leader' : ''}">
            <div class="node-header">
                <span class="node-name">${node.role === 'leader' ? '\u{1F451}' : '\u{2699}\uFE0F'} ${node.name}</span>
                <span class="node-role ${node.role}">${node.role}</span>
            </div>
            <div class="node-ip">${node.ip} | ${node.memory}</div>
            <div class="node-metrics">
                <div class="metric-row">
                    <span class="metric-label">CPU</span>
                    <div class="metric-bar"><div class="metric-fill cpu" data-width="${node.cpu}%"></div></div>
                    <span class="metric-value">${node.cpu}%</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">RAM</span>
                    <div class="metric-bar"><div class="metric-fill ram" data-width="${node.ram}%"></div></div>
                    <span class="metric-value">${node.ram}%</span>
                </div>
                <div class="metric-row">
                    <span class="metric-label">Disk</span>
                    <div class="metric-bar"><div class="metric-fill disk" data-width="${node.disk}%"></div></div>
                    <span class="metric-value">${node.disk}%</span>
                </div>
            </div>
            ${node.agents.length > 0 ? `
                <div class="node-agents">
                    ${node.agents.length} agent${node.agents.length > 1 ? 's' : ''}: ${node.agents.join(', ')}
                </div>
            ` : `
                <div class="node-agents">No agents deployed</div>
            `}
        </div>
    `).join('');
}

function renderAgentGrid() {
    const grid = document.getElementById('agentGrid');
    if (!grid) return;

    grid.innerHTML = MOCK_DATA.agents.map(agent => `
        <div class="agent-card" onclick="openAgentTerminal('${agent.id}')">
            <div class="agent-card-header">
                <span class="agent-name">${agent.icon} ${agent.name}</span>
                <span class="agent-status"></span>
            </div>
            <div class="agent-image">${agent.image}</div>
            <div class="agent-meta">
                <span class="agent-meta-item">\u{1F4BB} ${agent.node}</span>
                <span class="agent-meta-item">\u{1F4BE} ${agent.memory}</span>
            </div>
            ${agent.snapshots > 0 ? `
                <div class="agent-snapshots">\u{1F4F7} ${agent.snapshots} snapshot${agent.snapshots > 1 ? 's' : ''}</div>
            ` : ''}
        </div>
    `).join('');
}

function animateMetricBars() {
    setTimeout(() => {
        document.querySelectorAll('.metric-fill').forEach(bar => {
            const width = bar.dataset.width;
            if (width) bar.style.width = width;
        });
    }, 100);
}

// ---- Agents / Terminal View ----
function renderAgentsView() {
    renderAgentList();
    renderTerminalTabs();

    // If no agent is active, show placeholder
    if (!activeAgent) {
        showTerminalPlaceholder();
    }
}

function renderAgentList() {
    const list = document.getElementById('agentList');
    if (!list) return;

    list.innerHTML = MOCK_DATA.agents.map(agent => `
        <div class="agent-list-item ${activeAgent === agent.id ? 'active' : ''}"
             onclick="selectAgent('${agent.id}')">
            <div class="agent-list-icon">${agent.icon}</div>
            <div class="agent-list-info">
                <span class="agent-list-name">${agent.name}</span>
                <span class="agent-list-detail">${agent.memory}</span>
            </div>
        </div>
    `).join('');
}

function renderTerminalTabs() {
    const tabs = document.getElementById('terminalTabs');
    if (!tabs) return;

    tabs.innerHTML = MOCK_DATA.agents.map(agent => `
        <button class="terminal-tab ${activeAgent === agent.id ? 'active' : ''}"
                onclick="selectAgent('${agent.id}')">
            ${agent.name}
        </button>
    `).join('');
}

function showTerminalPlaceholder() {
    const container = document.getElementById('terminalContainer');
    const placeholder = document.getElementById('terminalPlaceholder');
    if (placeholder) placeholder.style.display = 'flex';

    // Hide all xterm instances
    Object.values(terminalInstances).forEach(inst => {
        if (inst.element) inst.element.style.display = 'none';
    });
}

function selectAgent(agentId) {
    activeAgent = agentId;
    renderAgentList();
    renderTerminalTabs();

    const placeholder = document.getElementById('terminalPlaceholder');
    if (placeholder) placeholder.style.display = 'none';

    // Hide other terminals
    Object.entries(terminalInstances).forEach(([id, inst]) => {
        if (inst.element) inst.element.style.display = id === agentId ? 'block' : 'none';
    });

    // Create terminal if not exists
    if (!terminalInstances[agentId]) {
        createTerminal(agentId);
    } else {
        // Resize existing terminal
        setTimeout(() => {
            if (terminalInstances[agentId].fitAddon) {
                terminalInstances[agentId].fitAddon.fit();
            }
        }, 50);
    }
}

function createTerminal(agentId) {
    const agent = MOCK_DATA.agents.find(a => a.id === agentId);
    if (!agent) return;

    const container = document.getElementById('terminalContainer');

    // Create terminal element
    const termEl = document.createElement('div');
    termEl.id = `terminal-${agentId}`;
    termEl.style.height = '100%';
    termEl.style.display = 'block';
    container.appendChild(termEl);

    // Initialize xterm.js
    const term = new Terminal({
        theme: {
            background: '#1a1a2e',
            foreground: '#e8e8f0',
            cursor: '#00d4ff',
            cursorAccent: '#1a1a2e',
            selectionBackground: 'rgba(0, 212, 255, 0.3)',
            black: '#1a1a2e',
            red: '#ff4444',
            green: '#00ff88',
            yellow: '#ffd700',
            blue: '#00d4ff',
            magenta: '#b388ff',
            cyan: '#00d4ff',
            white: '#e8e8f0',
            brightBlack: '#555570',
            brightRed: '#ff6666',
            brightGreen: '#33ff99',
            brightYellow: '#ffdd33',
            brightBlue: '#33ddff',
            brightMagenta: '#cc99ff',
            brightCyan: '#33ddff',
            brightWhite: '#ffffff'
        },
        fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
        fontSize: 13,
        lineHeight: 1.4,
        cursorBlink: true,
        cursorStyle: 'bar',
        scrollback: 1000,
        allowProposedApi: true
    });

    const fitAddon = new FitAddon.FitAddon();
    term.loadAddon(fitAddon);

    term.open(termEl);
    setTimeout(() => fitAddon.fit(), 50);

    // Create mock
    const mock = new TerminalMock(term, agent);

    terminalInstances[agentId] = {
        xterm: term,
        fitAddon: fitAddon,
        mock: mock,
        element: termEl
    };

    // Handle resize
    const resizeObserver = new ResizeObserver(() => {
        try { fitAddon.fit(); } catch(e) {}
    });
    resizeObserver.observe(termEl);
}

function openAgentTerminal(agentId) {
    navigateTo('agents');
    setTimeout(() => selectAgent(agentId), 100);
}

// ---- Terminal Toolbar Actions ----
function clearActiveTerminal() {
    if (activeAgent && terminalInstances[activeAgent]) {
        terminalInstances[activeAgent].mock.reset();
        showToast('Terminal cleared', 'info');
    }
}

function snapshotActiveAgent() {
    if (activeAgent) {
        const agent = MOCK_DATA.agents.find(a => a.id === activeAgent);
        if (agent) {
            agent.snapshots = (agent.snapshots || 0) + 1;
            showToast(`Snapshot saved for ${agent.name}!`, 'success');
            // Update the agent list if visible
            renderAgentList();
            renderAgentGrid();
        }
    }
}

function restartActiveTerminal() {
    if (activeAgent && terminalInstances[activeAgent]) {
        terminalInstances[activeAgent].mock.reset();
        showToast('Agent terminal restarted', 'info');
    }
}

// ---- Topology View ----
function renderTopology() {
    const diagram = document.getElementById('topologyDiagram');
    if (!diagram) return;

    const leader = MOCK_DATA.nodes.find(n => n.role === 'leader');
    const workers = MOCK_DATA.nodes.filter(n => n.role === 'worker');

    diagram.innerHTML = `
        <div class="topology-node leader-node">
            <span class="topology-node-name">\u{1F451} ${leader.name}</span>
            <span class="topology-node-ip">${leader.ip}</span>
            <span class="topology-node-role" style="background: var(--green-dim); color: var(--green);">Leader</span>
            <div class="topology-agents">
                ${leader.agents.map(a => {
                    const agent = MOCK_DATA.agents.find(ag => ag.id === a);
                    return agent ? `<span class="topology-agent">${agent.icon} ${agent.name}</span>` : '';
                }).join('')}
            </div>
        </div>

        <div class="topology-link"></div>

        <div class="topology-workers">
            ${workers.map(w => `
                <div class="topology-link"></div>
                <div class="topology-node worker-node">
                    <span class="topology-node-name">\u{2699}\uFE0F ${w.name}</span>
                    <span class="topology-node-ip">${w.ip}</span>
                    <span class="topology-node-role" style="background: var(--cyan-dim); color: var(--cyan);">Worker</span>
                    <div class="topology-agents">
                        ${w.agents.length > 0 ? w.agents.map(a => {
                            const agent = MOCK_DATA.agents.find(ag => ag.id === a);
                            return agent ? `<span class="topology-agent">${agent.icon} ${agent.name}</span>` : '';
                        }).join('') : '<span style="color: var(--text-dim); font-size: 0.78rem;">No agents</span>'}
                    </div>
                </div>
            `).join('')}
        </div>
    `;
}

// ---- Guided Demo ----
function playGuidedDemo() {
    if (guidedDemoRunning) return;
    guidedDemoRunning = true;

    showToast('Starting guided demo...', 'info');

    // Step 1: Navigate to agents view
    navigateTo('agents');

    setTimeout(() => {
        // Step 2: Select gemini-cli
        selectAgent('gemini-cli');

        setTimeout(() => {
            if (!terminalInstances['gemini-cli']) return;
            const mock = terminalInstances['gemini-cli'].mock;

            // Step 3: Type a command
            mock.typeCommand('/status', () => {
                setTimeout(() => {
                    mock.typeCommand('Explain the mesh architecture', () => {
                        setTimeout(() => {
                            // Step 4: Switch to plotcode
                            selectAgent('plotcode');

                            setTimeout(() => {
                                if (!terminalInstances['plotcode']) return;
                                const plotMock = terminalInstances['plotcode'].mock;

                                plotMock.typeCommand('plot bar quarterly-revenue', () => {
                                    setTimeout(() => {
                                        // Step 5: Take snapshot
                                        snapshotActiveAgent();

                                        setTimeout(() => {
                                            // Step 6: Switch to researcher
                                            selectAgent('researcher');

                                            setTimeout(() => {
                                                if (!terminalInstances['researcher']) return;
                                                const resMock = terminalInstances['researcher'].mock;

                                                resMock.typeCommand('search lightweight container orchestration', () => {
                                                    setTimeout(() => {
                                                        // Step 7: Back to dashboard
                                                        navigateTo('dashboard');
                                                        showToast('Guided demo complete! Try interacting with the terminals yourself.', 'success');
                                                        guidedDemoRunning = false;
                                                    }, 1000);
                                                });
                                            }, 500);
                                        }, 800);
                                    }, 1500);
                                });
                            }, 500);
                        }, 2000);
                    });
                }, 1500);
            });
        }, 800);
    }, 500);
}

// ---- Initialize ----
document.addEventListener('DOMContentLoaded', () => {
    // Check if we should skip login (for testing)
    const hash = window.location.hash;
    if (hash === '#dashboard' || hash === '#agents') {
        document.getElementById('view-login').classList.add('hidden');
        document.getElementById('appShell').classList.add('active');
        currentView = hash.slice(1);
        navigateTo(currentView);
    }

    // Handle hash changes
    window.addEventListener('hashchange', () => {
        const newHash = window.location.hash.slice(1);
        if (['dashboard', 'agents', 'topology', 'settings'].includes(newHash)) {
            navigateTo(newHash);
        }
    });
});
