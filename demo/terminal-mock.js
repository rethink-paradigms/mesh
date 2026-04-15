/* ============================================================
   MESH — Terminal Mock Engine
   Interactive terminal simulation using xterm.js
   ============================================================ */

class TerminalMock {
    constructor(xtermInstance, agentConfig) {
        this.term = xtermInstance;
        this.agent = agentConfig;
        this.inputBuffer = '';
        this.isProcessing = false;
        this.commandHistory = [];
        this.historyIndex = -1;
        this.active = true;

        this.welcomeBanner = this._buildWelcomeBanner();
        this.prompt = this._buildPrompt();
        this.mockResponses = this._buildMockResponses();

        this._init();
    }

    _init() {
        // Write welcome banner
        this.term.write(this.welcomeBanner);
        this.term.write(this.prompt);

        // Listen for input
        this.term.onData(this._handleInput.bind(this));
    }

    _buildWelcomeBanner() {
        const banners = {
            'gemini-cli': '\r\n\x1b[1;36m  ██████╗ ██╗███╗   ██╗███████╗██╗███╗   ██╗███████╗██╗     ██╗     ███████╗██████╗ \r\n\x1b[1;36m  ██╔════╝ ██║████╗  ██║██╔════╝██║████╗  ██║██╔════╝██║     ██║     ██╔════╝██╔══██╗\r\n\x1b[1;36m  ██║  ███╗██║██╔██╗ ██║█████╗  ██║██╔██╗ ██║█████╗  ██║     ██║     █████╗  ██████╔╝\r\n\x1b[1;36m  ██║   ██║██║██║╚██╗██║██╔══╝  ██║██║╚██╗██║██╔══╝  ██║     ██║     ██╔══╝  ██╔══██╗\r\n\x1b[1;36m  ╚██████╔╝██║██║ ╚████║██║     ██║██║ ╚████║███████╗███████╗███████╗███████╗██║  ██║\r\n\x1b[1;36m   ╚═════╝ ╚═╝╚═╝  ╚═══╝╚═╝     ╚═╝╚═╝  ╚═══╝╚══════╝╚══════╝╚══════╝╚══════╝╚═╝  ╚═╝\r\n\x1b[0m\r\n\x1b[2m  Gemini CLI v2.1.0 | Model: gemini-2.0-flash | Agent: gemini-cli\r\n\x1b[2m  Type a prompt to chat, /help for commands\r\n\x1b[0m\r\n',

            'plotcode': '\r\n\x1b[1;35m  ╔══════════════════════════════════════╗\r\n\x1b[1;35m  ║   Plotcode Agent v1.4.0              ║\r\n\x1b[1;35m  ║   Code Generation & Visualization    ║\r\n\x1b[1;35m  ║   Model: plotcode-v2                 ║\r\n\x1b[1;35m  ╚══════════════════════════════════════╝\r\n\x1b[0m\r\n\x1b[2m  Commands: plot, analyze, export, render, help\r\n\x1b[0m\r\n',

            'researcher': '\r\n\x1b[1;33m  ┌────────────────────────────────────────┐\r\n\x1b[1;33m  │  Research Agent v3.0.0                 │\r\n\x1b[1;33m  │  Deep Research & Analysis Engine       │\r\n\x1b[1;33m  │  Sources: arxiv, github, web           │\r\n\x1b[1;33m  └────────────────────────────────────────┘\r\n\x1b[0m\r\n\x1b[2m  Commands: search <topic>, summarize <url>, cite <topic>, report <topic>\r\n\x1b[0m\r\n'
        };

        return banners[this.agent.id] || '\r\n\x1b[2m  Agent terminal ready.\r\n\x1b[0m\r\n';
    }

    _buildPrompt() {
        const prompts = {
            'gemini-cli': '\x1b[1;36m❯\x1b[0m ',
            'plotcode': '\x1b[1;35mplot>\x1b[0m ',
            'researcher': '\x1b[1;33mresearch>\x1b[0m '
        };
        return prompts[this.agent.id] || '\x1b[1;36m>\x1b[0m ';
    }

    _buildMockResponses() {
        const responses = {
            'gemini-cli': {
                '/help': () => '\r\n\x1b[1;36m  Available Commands:\x1b[0m\r\n\r\n' +
                    '    /help          Show this help message\r\n' +
                    '    /model         Show current model info\r\n' +
                    '    /clear         Clear terminal\r\n' +
                    '    /history       Show conversation history\r\n' +
                    '    /snapshot      Save agent state\r\n' +
                    '    /status        Show agent status\r\n\r\n' +
                    '  Or type any prompt to chat with Gemini.\r\n\r\n',

                '/model': () => '\r\n\x1b[2m  Current model: gemini-2.0-flash\r\n  Context window: 1M tokens\r\n  Temperature: 0.7\r\n  Agent memory: 47MB (12 conversations)\x1b[0m\r\n\r\n',

                '/status': () => '\r\n\x1b[32m  ● Agent Status: Running\x1b[0m\r\n\r\n' +
                    '    Uptime:     47h 23m\r\n' +
                    '    Memory:     312 MB / 512 MB\r\n' +
                    '    CPU:        12%\r\n' +
                    '    Conversations: 12\r\n' +
                    '    Snapshots:  1\r\n\r\n',

                '/history': () => '\r\n\x1b[2m  Recent conversations:\x1b[0m\r\n\r\n' +
                    '    1. "Explain the mesh architecture" (2h ago)\r\n' +
                    '    2. "Write a Python script for..." (5h ago)\r\n' +
                    '    3. "Analyze the deployment logs" (8h ago)\r\n' +
                    '    4. "What is Tailscale?" (12h ago)\r\n\r\n',

                '/snapshot': () => '\r\n\x1b[33m  📸 Snapshotting agent state...\x1b[0m\r\n\r\n' +
                    '\x1b[2m  [1/3] Pausing agent processes...\x1b[0m\r\n' +
                    '\x1b[2m  [2/3] Committing container filesystem...\x1b[0m\r\n' +
                    '\x1b[2m  [3/3] Tagging snapshot image...\x1b[0m\r\n\r\n' +
                    '\x1b[32m  ✓ Snapshot saved: gemini-cli:snapshot-' + new Date().toISOString().slice(0,10) + '\x1b[0m\r\n\r\n',

                'default': (input) => {
                    const responses = [
                        `\r\n\x1b[2m  Thinking...\x1b[0m\r\n\r\n` +
                        `  I can help you with that! Based on your prompt "${input.slice(0, 50)}...",\r\n` +
                        `  here's what I found:\r\n\r\n` +
                        `  The Mesh Platform uses a lightweight architecture combining Nomad,\r\n` +
                        `  Consul, Tailscale, and Traefik for a total control plane overhead of\r\n` +
                        `  just 530MB. This makes it ideal for running AI agents on small VMs.\r\n\r\n` +
                        `  \x1b[2m  [Response generated in 1.2s | Tokens: 847 | Model: gemini-2.0-flash]\x1b[0m\r\n\r\n`,

                        `\r\n\x1b[2m  Analyzing...\x1b[0m\r\n\r\n` +
                        `  Great question! Let me break this down:\r\n\r\n` +
                        `  1. \x1b[1mAgent Isolation\x1b[0m — Each agent runs in its own Docker container\r\n` +
                        `  2. \x1b[1mStateful Persistence\x1b[0m — The container filesystem IS the agent's brain\r\n` +
                        `  3. \x1b[1mSnapshot & Revert\x1b[0m — Save and restore agent state at any time\r\n\r\n` +
                        `  This approach is fundamentally different from stateless microservices.\r\n\r\n` +
                        `  \x1b[2m  [Response generated in 0.8s | Tokens: 623]\x1b[0m\r\n\r\n`,

                        `\r\n\x1b[2m  Processing your request...\x1b[0m\r\n\r\n` +
                        `  Here's my analysis of "${input.slice(0, 40)}...":\r\n\r\n` +
                        `  The key insight is that AI agents need persistent, stateful environments.\r\n` +
                        `  Unlike traditional web services, an agent's accumulated knowledge lives\r\n` +
                        `  in its filesystem. Losing the container means losing the brain.\r\n\r\n` +
                        `  The Mesh Platform solves this with periodic snapshots (docker commit)\r\n` +
                        `  that capture the full container state, enabling revert and clone.\r\n\r\n` +
                        `  \x1b[2m  [Response generated in 1.5s | Tokens: 1,024]\x1b[0m\r\n\r\n`
                    ];
                    return responses[Math.floor(Math.random() * responses.length)];
                }
            },

            'plotcode': {
                '/help': () => '\r\n\x1b[1;35m  Plotcode Commands:\x1b[0m\r\n\r\n' +
                    '    plot <type> <data>    Generate a plot\r\n' +
                    '    analyze <file>        Analyze data file\r\n' +
                    '    export <format>       Export last plot\r\n' +
                    '    render <template>     Render from template\r\n' +
                    '    status                Show agent status\r\n\r\n',

                'plot': (args) => '\r\n\x1b[35m  Generating plot...\x1b[0m\r\n\r\n' +
                    '  ┌─────────────────────────────────────────────┐\r\n' +
                    '  │  ██                                          │\r\n' +
                    '  │  ██  ████                                    │\r\n' +
                    '  │  ██  ████  ██████                            │\r\n' +
                    '  │  ██  ████  ██████  ████████                  │\r\n' +
                    '  │  ██  ████  ██████  ████████  ████████████    │\r\n' +
                    '  │  ────┬─────┬───────┬─────────┬──────────     │\r\n' +
                    '  │      Q1    Q2      Q3        Q4              │\r\n' +
                    '  └─────────────────────────────────────────────┘\r\n\r\n' +
                    '  \x1b[32m✓ Plot generated\x1b[0m | Type: bar | Data points: 4\r\n' +
                    '  \x1b[2mSaved to /agent/output/plot-' + Date.now() + '.svg\x1b[0m\r\n\r\n',

                'analyze': (args) => '\r\n\x1b[35m  Analyzing data...\x1b[0m\r\n\r\n' +
                    '  \x1b[1mSummary Statistics:\x1b[0m\r\n' +
                    '    Records:     1,247\r\n' +
                    '    Columns:     8\r\n' +
                    '    Null values: 3 (0.02%)\r\n\r\n' +
                    '  \x1b[1mKey Findings:\x1b[0m\r\n' +
                    '    • Strong positive correlation (r=0.89) between columns A and C\r\n' +
                    '    • Outlier detected at row 892 (value: 47.3σ)\r\n' +
                    '    • Distribution is right-skewed (skewness: 2.1)\r\n\r\n' +
                    '  \x1b[32m✓ Analysis complete\x1b[0m | Time: 0.4s\r\n\r\n',

                'export': (args) => '\r\n\x1b[35m  Exporting...\x1b[0m\r\n\r\n' +
                    '  Format: SVG\r\n' +
                    '  Size:   800x600px\r\n' +
                    '  Path:   /agent/output/export-' + Date.now() + '.svg\r\n\r\n' +
                    '  \x1b[32m✓ Export complete\x1b[0m\r\n\r\n',

                'status': () => '\r\n\x1b[32m  ● Plotcode Agent: Running\x1b[0m\r\n\r\n' +
                    '    Uptime:      47h 23m\r\n' +
                    '    Memory:      198 MB / 256 MB\r\n' +
                    '    Plots:       23 generated\r\n' +
                    '    Analyses:    8 completed\r\n\r\n',

                'default': (input) => {
                    const responses = [
                        '\r\n\x1b[35m  Rendering visualization...\x1b[0m\r\n\r\n' +
                        '  ┌──────────────────────────────────────────┐\r\n' +
                        '  │  ·  *   ·    *  ·     ·   *    ·  *      │\r\n' +
                        '  │    ·    *   ·    *   ·    *   ·    *     │\r\n' +
                        '  │  *   ·    *   ·  *   ·    *   ·    *    │\r\n' +
                        '  │    ·   *   ·    *   ·  *   ·    *   ·   │\r\n' +
                        '  │  ·   ·   *   ·   *   ·   *   ·    *     │\r\n' +
                        '  └──────────────────────────────────────────┘\r\n\r\n' +
                        '  \x1b[32m✓ Scatter plot rendered\x1b[0m | Points: 156\r\n\r\n',

                        '\r\n\x1b[35m  Processing...\x1b[0m\r\n\r\n' +
                        '  I can generate various visualizations. Try:\r\n' +
                        '    plot bar    — Bar chart\r\n' +
                        '    plot line   — Line chart\r\n' +
                        '    plot scatter — Scatter plot\r\n' +
                        '    analyze <file> — Data analysis\r\n\r\n'
                    ];
                    return responses[Math.floor(Math.random() * responses.length)];
                }
            },

            'researcher': {
                '/help': () => '\r\n\x1b[1;33m  Research Agent Commands:\x1b[0m\r\n\r\n' +
                    '    search <topic>      Search across sources\r\n' +
                    '    summarize <url>     Summarize a document\r\n' +
                    '    cite <topic>        Generate citations\r\n' +
                    '    report <topic>      Full research report\r\n' +
                    '    status              Show agent status\r\n\r\n',

                'search': (args) => '\r\n\x1b[33m  Searching across sources...\x1b[0m\r\n\r\n' +
                    '  \x1b[1mResults for "' + (args || 'mesh infrastructure') + '":\x1b[0m\r\n\r\n' +
                    '  1. \x1b[36mNomad: A next-generation cluster manager\x1b[0m\r\n' +
                    '     HashiCorp, 2023 — arxiv.org/abs/2301.12345\r\n' +
                    '     Relevance: 0.94\r\n\r\n' +
                    '  2. \x1b[36mLightweight Container Orchestration at Scale\x1b[0m\r\n' +
                    '     ACM SIGCOMM, 2023 — doi:10.1145/1234567\r\n' +
                    '     Relevance: 0.89\r\n\r\n' +
                    '  3. \x1b[36mTailscale: Mesh VPNs for Modern Infrastructure\x1b[0m\r\n' +
                    '     USENIX, 2022 — arxiv.org/abs/2209.54321\r\n' +
                    '     Relevance: 0.85\r\n\r\n' +
                    '  \x1b[32m✓ 3 results found\x1b[0m | Sources: arxiv, github, web\r\n\r\n',

                'summarize': (args) => '\r\n\x1b[33m  Summarizing document...\x1b[0m\r\n\r\n' +
                    '  \x1b[1mSummary:\x1b[0m\r\n' +
                    '  This paper presents a novel approach to container orchestration\r\n' +
                    '  that prioritizes memory efficiency over feature completeness.\r\n' +
                    '  The system achieves 90% less control plane overhead compared to\r\n' +
                    '  Kubernetes while maintaining compatibility with standard Docker\r\n' +
                    '  workloads.\r\n\r\n' +
                    '  \x1b[1mKey Findings:\x1b[0m\r\n' +
                    '  • 530MB total overhead vs 1GB+ for Kubernetes\r\n' +
                    '  • Supports 1000+ nodes with linear scaling\r\n' +
                    '  • Sub-second scheduling latency for simple workloads\r\n\r\n' +
                    '  \x1b[32m✓ Summary complete\x1b[0m | Words: 847 | Reading time: 2 min\r\n\r\n',

                'cite': (args) => '\r\n\x1b[33m  Generating citations...\x1b[0m\r\n\r\n' +
                    '  \x1b[1mAPA:\x1b[0m\r\n' +
                    '  HashiCorp. (2023). Nomad: A next-generation cluster manager.\r\n' +
                    '  Retrieved from https://nomadproject.io\r\n\r\n' +
                    '  \x1b[1mBibTeX:\x1b[0m\r\n' +
                    '  @misc{nomad2023,\r\n' +
                    '    title={Nomad: A next-generation cluster manager},\r\n' +
                    '    author={HashiCorp},\r\n' +
                    '    year={2023},\r\n' +
                    '    howpublished={\\url{https://nomadproject.io}}\r\n' +
                    '  }\r\n\r\n' +
                    '  \x1b[32m✓ 2 citation formats generated\x1b[0m\r\n\r\n',

                'report': (args) => '\r\n\x1b[33m  Generating research report...\x1b[0m\r\n\r\n' +
                    '  \x1b[2m  [1/4] Searching sources...\x1b[0m\r\n' +
                    '  \x1b[2m  [2/4] Analyzing documents...\x1b[0m\r\n' +
                    '  \x1b[2m  [3/4] Synthesizing findings...\x1b[0m\r\n' +
                    '  \x1b[2m  [4/4] Formatting report...\x1b[0m\r\n\r\n' +
                    '  \x1b[1mResearch Report: Lightweight Infrastructure for AI Agents\x1b[0m\r\n' +
                    '  Generated: ' + new Date().toLocaleDateString() + '\r\n\r\n' +
                    '  \x1b[1mAbstract:\x1b[0m\r\n' +
                    '  This report examines the feasibility of running stateful AI agents\r\n' +
                    '  on resource-constrained infrastructure. We analyze the trade-offs\r\n' +
                    '  between Kubernetes and lighter alternatives.\r\n\r\n' +
                    '  \x1b[1mConclusion:\x1b[0m\r\n' +
                    '  Nomad + Consul + Tailscale provides a viable alternative to K8s\r\n' +
                    '  for agent workloads, with 90% less memory overhead.\r\n\r\n' +
                    '  \x1b[32m✓ Report saved\x1b[0m | /agent/output/report-' + Date.now() + '.md\r\n\r\n',

                'status': () => '\r\n\x1b[32m  ● Research Agent: Running\x1b[0m\r\n\r\n' +
                    '    Uptime:      47h 23m\r\n' +
                    '    Memory:      245 MB / 384 MB\r\n' +
                    '    Searches:    156 completed\r\n' +
                    '    Reports:     12 generated\r\n\r\n',

                'default': (input) => {
                    const responses = [
                        '\r\n\x1b[33m  Searching knowledge base...\x1b[0m\r\n\r\n' +
                        '  I found several relevant sources for your query.\r\n' +
                        '  Try these commands for detailed results:\r\n\r\n' +
                        '    search <topic>   — Full search across sources\r\n' +
                        '    report <topic>   — Generate a comprehensive report\r\n' +
                        '    summarize <url>  — Summarize a specific document\r\n\r\n',

                        '\r\n\x1b[33m  Analyzing...\x1b[0m\r\n\r\n' +
                        '  Based on my research database, here are key insights:\r\n\r\n' +
                        '  The field of lightweight container orchestration has evolved\r\n' +
                        '  significantly since 2020. Key trends include:\r\n\r\n' +
                        '  1. Shift from monolithic orchestrators to composable tools\r\n' +
                        '  2. Growing adoption of mesh networking (WireGuard-based)\r\n' +
                        '  3. Focus on memory efficiency for edge deployments\r\n\r\n' +
                        '  \x1b[32m✓ Analysis complete\x1b[0m\r\n\r\n'
                    ];
                    return responses[Math.floor(Math.random() * responses.length)];
                }
            }
        };

        return responses[this.agent.id] || {
            '/help': () => '\r\n  Commands: help, status, exit\r\n\r\n',
            'status': () => '\r\n  Agent running.\r\n\r\n',
            'default': () => '\r\n  Command received.\r\n\r\n'
        };
    }

    _handleInput(data) {
        if (!this.active || this.isProcessing) return;

        const code = data.charCodeAt(0);

        // Ctrl+C
        if (code === 3) {
            this.term.write('^C\r\n');
            this.term.write(this.prompt);
            this.inputBuffer = '';
            return;
        }

        // Enter
        if (data === '\r') {
            this.term.write('\r\n');
            if (this.inputBuffer.trim()) {
                this.commandHistory.push(this.inputBuffer);
                this.historyIndex = this.commandHistory.length;
                this._processCommand(this.inputBuffer.trim());
            } else {
                this.term.write(this.prompt);
            }
            this.inputBuffer = '';
            return;
        }

        // Backspace
        if (code === 127 || data === '\x7f') {
            if (this.inputBuffer.length > 0) {
                this.inputBuffer = this.inputBuffer.slice(0, -1);
                this.term.write('\b \b');
            }
            return;
        }

        // Up arrow (history)
        if (data === '\x1b[A') {
            if (this.historyIndex > 0) {
                this.historyIndex--;
                // Clear current input
                for (let i = 0; i < this.inputBuffer.length; i++) {
                    this.term.write('\b \b');
                }
                this.inputBuffer = this.commandHistory[this.historyIndex];
                this.term.write(this.inputBuffer);
            }
            return;
        }

        // Down arrow (history)
        if (data === '\x1b[B') {
            if (this.historyIndex < this.commandHistory.length - 1) {
                this.historyIndex++;
                for (let i = 0; i < this.inputBuffer.length; i++) {
                    this.term.write('\b \b');
                }
                this.inputBuffer = this.commandHistory[this.historyIndex];
                this.term.write(this.inputBuffer);
            }
            return;
        }

        // Regular printable character
        if (code >= 32 && code < 127) {
            this.inputBuffer += data;
            this.term.write(data);
        }
    }

    _processCommand(input) {
        this.isProcessing = true;

        const parts = input.split(/\s+/);
        const cmd = parts[0].toLowerCase();
        const args = parts.slice(1).join(' ');

        let response;

        if (cmd === '/clear') {
            this.term.clear();
            this.term.write(this.prompt);
            this.isProcessing = false;
            return;
        }

        // Check for exact command match
        if (this.mockResponses[cmd]) {
            response = this.mockResponses[cmd](args);
        } else if (this.mockResponses[cmd.replace('/', '')]) {
            response = this.mockResponses[cmd.replace('/', '')](args);
        } else {
            response = this.mockResponses['default'](input);
        }

        // Simulate typing delay
        const delay = 200 + Math.random() * 400;
        setTimeout(() => {
            this.term.write(response);
            this.term.write(this.prompt);
            this.isProcessing = false;
        }, delay);
    }

    // Write a command programmatically (for guided demo)
    typeCommand(command, callback) {
        if (!this.active) return;

        this.isProcessing = true;
        let i = 0;

        const typeChar = () => {
            if (i < command.length) {
                this.term.write(command[i]);
                this.inputBuffer += command[i];
                i++;
                setTimeout(typeChar, 30 + Math.random() * 40);
            } else {
                // Submit
                this.term.write('\r\n');
                const cmd = this.inputBuffer.trim();
                this.inputBuffer = '';
                this.commandHistory.push(cmd);
                this.historyIndex = this.commandHistory.length;
                this._processCommand(cmd);

                // Wait for response then callback
                const checkDone = setInterval(() => {
                    if (!this.isProcessing) {
                        clearInterval(checkDone);
                        if (callback) callback();
                    }
                }, 100);
            }
        };

        typeChar();
    }

    // Clear and reset the terminal
    reset() {
        this.term.clear();
        this.inputBuffer = '';
        this.commandHistory = [];
        this.historyIndex = -1;
        this.isProcessing = false;
        this.term.write(this.welcomeBanner);
        this.term.write(this.prompt);
    }

    destroy() {
        this.active = false;
    }
}
