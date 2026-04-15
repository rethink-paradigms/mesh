/* ============================================================
   MESH — Demo Page Interactive Scripts
   ============================================================ */

// ---- Animated counter for hero stats ----
function animateCounters() {
    const stats = document.querySelectorAll('.stat-value[data-target]');
    
    const observer = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                const el = entry.target;
                const target = parseInt(el.dataset.target);
                const duration = 1500;
                const startTime = performance.now();
                
                function update(currentTime) {
                    const elapsed = currentTime - startTime;
                    const progress = Math.min(elapsed / duration, 1);
                    const eased = 1 - Math.pow(1 - progress, 3); // ease-out cubic
                    const current = Math.round(eased * target);
                    el.textContent = current.toLocaleString();
                    
                    if (progress < 1) {
                        requestAnimationFrame(update);
                    }
                }
                
                requestAnimationFrame(update);
                observer.unobserve(el);
            }
        });
    }, { threshold: 0.5 });
    
    stats.forEach(stat => observer.observe(stat));
}

// ---- Scroll reveal ----
function initScrollReveal() {
    const reveals = document.querySelectorAll(
        '.section-badge, .section-title, .section-subtitle, ' +
        '.comparison-card, .arch-card, .lifecycle-step, ' +
        '.vision-card, .snapshot-timeline, .terminal'
    );
    
    const observer = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                entry.target.classList.add('visible');
                entry.target.style.opacity = '1';
                entry.target.style.transform = 'translateY(0)';
            }
        });
    }, { threshold: 0.1, rootMargin: '0px 0px -50px 0px' });
    
    reveals.forEach(el => {
        el.style.opacity = '0';
        el.style.transform = 'translateY(20px)';
        el.style.transition = 'opacity 0.6s ease, transform 0.6s ease';
        observer.observe(el);
    });
}

// ---- Bar chart animation ----
function initBarAnimation() {
    const bars = document.querySelectorAll('.bar-fill');
    
    const observer = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                const bar = entry.target;
                const width = bar.style.width;
                bar.style.width = '0%';
                setTimeout(() => {
                    bar.style.width = width;
                }, 100);
                observer.unobserve(bar);
            }
        });
    }, { threshold: 0.3 });
    
    bars.forEach(bar => observer.observe(bar));
}

// ---- Mesh Canvas Animation ----
function initMeshCanvas() {
    const canvas = document.getElementById('meshCanvas');
    if (!canvas) return;
    
    const ctx = canvas.getContext('2d');
    const dpr = window.devicePixelRatio || 1;
    
    function resize() {
        const rect = canvas.parentElement.getBoundingClientRect();
        canvas.width = rect.width * dpr;
        canvas.height = rect.height * dpr;
        canvas.style.width = rect.width + 'px';
        canvas.style.height = rect.height + 'px';
        ctx.scale(dpr, dpr);
    }
    resize();
    window.addEventListener('resize', resize);
    
    // Create nodes
    const nodes = [];
    const nodeCount = 12;
    const w = canvas.width / dpr;
    const h = canvas.height / dpr;
    
    for (let i = 0; i < nodeCount; i++) {
        nodes.push({
            x: Math.random() * w,
            y: Math.random() * h,
            vx: (Math.random() - 0.5) * 0.5,
            vy: (Math.random() - 0.5) * 0.3,
            radius: 3 + Math.random() * 3,
        });
    }
    
    function drawFrame() {
        ctx.clearRect(0, 0, w, h);
        
        // Draw connections
        for (let i = 0; i < nodes.length; i++) {
            for (let j = i + 1; j < nodes.length; j++) {
                const dx = nodes[j].x - nodes[i].x;
                const dy = nodes[j].y - nodes[i].y;
                const dist = Math.sqrt(dx * dx + dy * dy);
                
                if (dist < 200) {
                    const alpha = (1 - dist / 200) * 0.3;
                    ctx.strokeStyle = `rgba(196, 90, 44, ${alpha})`;
                    ctx.lineWidth = 1;
                    ctx.beginPath();
                    ctx.moveTo(nodes[i].x, nodes[i].y);
                    ctx.lineTo(nodes[j].x, nodes[j].y);
                    ctx.stroke();
                }
            }
        }
        
        // Draw nodes
        nodes.forEach(node => {
            ctx.beginPath();
            ctx.arc(node.x, node.y, node.radius, 0, Math.PI * 2);
            ctx.fillStyle = 'rgba(196, 90, 44, 0.6)';
            ctx.fill();
            
            // Glow
            ctx.beginPath();
            ctx.arc(node.x, node.y, node.radius * 3, 0, Math.PI * 2);
            const gradient = ctx.createRadialGradient(
                node.x, node.y, 0,
                node.x, node.y, node.radius * 3
            );
            gradient.addColorStop(0, 'rgba(196, 90, 44, 0.15)');
            gradient.addColorStop(1, 'rgba(196, 90, 44, 0)');
            ctx.fillStyle = gradient;
            ctx.fill();
            
            // Move
            node.x += node.vx;
            node.y += node.vy;
            
            // Bounce
            if (node.x < 0 || node.x > w) node.vx *= -1;
            if (node.y < 0 || node.y > h) node.vy *= -1;
        });
        
        requestAnimationFrame(drawFrame);
    }
    
    drawFrame();
}

// ---- Terminal Demo ----
const DEMO_LINES = [
    { type: 'prompt', text: '$ ', delay: 0 },
    { type: 'command', text: 'mesh init --provider multipass --workers 2', delay: 50 },
    { type: 'blank', delay: 400 },
    { type: 'info', text: '  → Provider: Local (Multipass)', delay: 100 },
    { type: 'info', text: '  → Workers: 2', delay: 100 },
    { type: 'info', text: '  → Control Plane: 530 MB', delay: 100 },
    { type: 'blank', delay: 200 },
    { type: 'dim', text: '  [1/5] Generating Tailscale auth key...', delay: 300 },
    { type: 'dim', text: '  [2/5] Provisioning leader (agent-mesh-leader)...', delay: 800 },
    { type: 'dim', text: '  [3/5] Provisioning worker (agent-mesh-worker-1)...', delay: 600 },
    { type: 'dim', text: '  [4/5] Provisioning worker (agent-mesh-worker-2)...', delay: 600 },
    { type: 'dim', text: '  [5/5] Configuring mesh network...', delay: 400 },
    { type: 'blank', delay: 200 },
    { type: 'success', text: '  ✓ Cluster is ready!', delay: 100 },
    { type: 'info', text: '    Nodes: 1 leader + 2 workers', delay: 50 },
    { type: 'info', text: '    Provider: multipass (local)', delay: 50 },
    { type: 'blank', delay: 600 },
    
    { type: 'prompt', text: '$ ', delay: 0 },
    { type: 'command', text: 'mesh agent deploy researcher --image team/researcher:v1 --memory 512', delay: 50 },
    { type: 'blank', delay: 300 },
    { type: 'agent', text: '  🤖 Deploying agent: researcher', delay: 100 },
    { type: 'dim', text: '  [1/4] Pulling image...', delay: 400 },
    { type: 'dim', text: '  [2/4] Creating Nomad job spec...', delay: 200 },
    { type: 'dim', text: '  [3/4] Scheduling on mesh...', delay: 300 },
    { type: 'dim', text: '  [4/4] Agent starting...', delay: 200 },
    { type: 'success', text: '  ✓ Agent \'researcher\' deployed to mesh', delay: 100 },
    { type: 'blank', delay: 400 },
    
    { type: 'prompt', text: '$ ', delay: 0 },
    { type: 'command', text: 'mesh agent deploy code-writer --image team/code-writer:v1 --memory 256', delay: 50 },
    { type: 'blank', delay: 200 },
    { type: 'agent', text: '  🤖 Deploying agent: code-writer', delay: 100 },
    { type: 'dim', text: '  [1/4] Pulling image...', delay: 300 },
    { type: 'dim', text: '  [2/4] Creating Nomad job spec...', delay: 150 },
    { type: 'dim', text: '  [3/4] Scheduling on mesh...', delay: 200 },
    { type: 'dim', text: '  [4/4] Agent starting...', delay: 150 },
    { type: 'success', text: '  ✓ Agent \'code-writer\' deployed to mesh', delay: 100 },
    { type: 'blank', delay: 400 },
    
    { type: 'prompt', text: '$ ', delay: 0 },
    { type: 'command', text: 'mesh agent snapshot researcher --tag checkpoint-v1', delay: 50 },
    { type: 'blank', delay: 200 },
    { type: 'warning', text: '  📸 Snapshotting agent \'researcher\'', delay: 100 },
    { type: 'dim', text: '  [1/3] Pausing agent processes...', delay: 300 },
    { type: 'dim', text: '  [2/3] Committing container filesystem...', delay: 500 },
    { type: 'dim', text: '  [3/3] Tagging snapshot image...', delay: 200 },
    { type: 'success', text: '  ✓ Snapshot saved: researcher:checkpoint-v1', delay: 100 },
    { type: 'dim', text: '    Restore: mesh agent deploy researcher --image researcher:checkpoint-v1', delay: 50 },
    { type: 'dim', text: '    Clone:   mesh agent deploy researcher-2 --image researcher:checkpoint-v1', delay: 50 },
    { type: 'blank', delay: 400 },
    
    { type: 'prompt', text: '$ ', delay: 0 },
    { type: 'command', text: 'mesh status', delay: 50 },
    { type: 'blank', delay: 300 },
    { type: 'info', text: '  🔗 Mesh Network', delay: 100 },
    { type: 'info', text: '  ├── 👑 mesh-leader      🟢  100.64.0.1   RAM: 2 GB', delay: 80 },
    { type: 'info', text: '  ├── ⚙️  mesh-worker-1    🟢  100.64.0.2   RAM: 2 GB', delay: 80 },
    { type: 'agent', text: '  │   ├── 🤖 researcher    🟢  512 MB  📸 1 snapshot', delay: 80 },
    { type: 'agent', text: '  │   └── 🤖 code-writer   🟢  256 MB', delay: 80 },
    { type: 'info', text: '  └── ⚙️  mesh-worker-2    🟢  100.64.0.3   RAM: 4 GB', delay: 80 },
    { type: 'blank', delay: 200 },
    { type: 'dim', text: '  Nodes: 3  │  Agents: 2/2 running  │  Snapshots: 1  │  Control Plane: 530 MB', delay: 100 },
];

let demoRunning = false;
let demoTimeout = null;

function startTerminalDemo() {
    if (demoRunning) return;
    demoRunning = true;
    
    const content = document.getElementById('terminalContent');
    content.innerHTML = '';
    
    const playBtn = document.getElementById('playDemo');
    playBtn.textContent = '⏸ Running...';
    playBtn.disabled = true;
    
    let lineIndex = 0;
    let totalDelay = 0;
    
    function addLine() {
        if (lineIndex >= DEMO_LINES.length) {
            demoRunning = false;
            playBtn.textContent = '▶ Play Demo';
            playBtn.disabled = false;
            return;
        }
        
        const line = DEMO_LINES[lineIndex];
        lineIndex++;
        
        if (line.type === 'blank') {
            content.innerHTML += '<br>';
        } else {
            const span = document.createElement('span');
            span.className = `term-${line.type}`;
            
            if (line.type === 'prompt' || line.type === 'command') {
                // Type out commands character by character
                const isCommand = line.type === 'command';
                if (!isCommand) {
                    span.textContent = line.text;
                    content.appendChild(span);
                    demoTimeout = setTimeout(addLine, line.delay);
                    return;
                } else {
                    let charIndex = 0;
                    span.textContent = '';
                    content.appendChild(span);
                    
                    function typeChar() {
                        if (charIndex < line.text.length) {
                            span.textContent += line.text[charIndex];
                            charIndex++;
                            // Auto-scroll
                            const body = document.getElementById('terminalBody');
                            body.scrollTop = body.scrollHeight;
                            demoTimeout = setTimeout(typeChar, 25);
                        } else {
                            content.innerHTML += '<br>';
                            demoTimeout = setTimeout(addLine, line.delay);
                        }
                    }
                    typeChar();
                    return;
                }
            } else {
                span.textContent = line.text;
                content.appendChild(span);
                content.innerHTML += '<br>';
            }
        }
        
        // Auto-scroll
        const body = document.getElementById('terminalBody');
        body.scrollTop = body.scrollHeight;
        
        demoTimeout = setTimeout(addLine, line.delay);
    }
    
    addLine();
}

function resetTerminalDemo() {
    demoRunning = false;
    if (demoTimeout) clearTimeout(demoTimeout);
    
    const content = document.getElementById('terminalContent');
    content.innerHTML = '<span class="term-dim">Click "Play Demo" to see mesh in action...</span>';
    
    const playBtn = document.getElementById('playDemo');
    playBtn.textContent = '▶ Play Demo';
    playBtn.disabled = false;
}

// ---- Hero particles ----
function initParticles() {
    const container = document.getElementById('heroParticles');
    if (!container) return;
    
    for (let i = 0; i < 30; i++) {
        const particle = document.createElement('div');
        particle.style.cssText = `
            position: absolute;
            width: ${2 + Math.random() * 3}px;
            height: ${2 + Math.random() * 3}px;
            background: rgba(196, 90, 44, ${0.1 + Math.random() * 0.3});
            border-radius: 50%;
            left: ${Math.random() * 100}%;
            top: ${Math.random() * 100}%;
            animation: float ${5 + Math.random() * 10}s ease-in-out infinite;
            animation-delay: ${Math.random() * 5}s;
        `;
        container.appendChild(particle);
    }
    
    // Add float keyframe
    const style = document.createElement('style');
    style.textContent = `
        @keyframes float {
            0%, 100% { transform: translate(0, 0) scale(1); opacity: 0.3; }
            25% { transform: translate(${20}px, -${30}px) scale(1.2); opacity: 0.6; }
            50% { transform: translate(-${10}px, ${20}px) scale(0.8); opacity: 0.4; }
            75% { transform: translate(${15}px, ${10}px) scale(1.1); opacity: 0.5; }
        }
    `;
    document.head.appendChild(style);
}

// ---- Nav scroll effect ----
function initNavScroll() {
    const nav = document.getElementById('nav');
    window.addEventListener('scroll', () => {
        if (window.scrollY > 50) {
            nav.style.borderBottomColor = 'rgba(196, 90, 44, 0.1)';
            nav.style.background = 'rgba(10, 10, 15, 0.95)';
        } else {
            nav.style.borderBottomColor = 'rgba(255, 255, 255, 0.06)';
            nav.style.background = 'rgba(10, 10, 15, 0.85)';
        }
    });
}

// ---- Initialize everything ----
document.addEventListener('DOMContentLoaded', () => {
    animateCounters();
    initScrollReveal();
    initBarAnimation();
    initMeshCanvas();
    initParticles();
    initNavScroll();
    
    // Set initial terminal state
    const content = document.getElementById('terminalContent');
    if (content) {
        content.innerHTML = '<span class="term-dim">Click "Play Demo" to see mesh in action...</span>';
    }
});
