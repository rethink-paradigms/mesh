(function () {
    var running = false;

    function injectStyles() {
        var s = document.createElement("style");
        s.id = "wtStyles";
        s.textContent =
            ".btn-demo{background:transparent;color:#c45a2c;border:1px solid rgba(196,90,44,0.3);}" +
            ".btn-demo:hover{background:rgba(196,90,44,0.08);border-color:#c45a2c;opacity:1;}" +
            ".btn-demo:disabled{cursor:default;}" +
            "#wtOverlay{position:fixed;bottom:20px;left:50%;transform:translateX(-50%);z-index:10000;" +
            "background:rgba(12,12,12,0.95);border:1px solid #c45a2c;border-radius:4px;padding:10px 18px;" +
            "display:flex;align-items:center;gap:14px;font-family:'Inter','IBM Plex Sans',sans-serif;" +
            "opacity:0;transition:opacity 0.4s ease;}" +
            "#wtOverlay.visible{opacity:1;}" +
            "#wtLabel{font-size:0.7rem;font-weight:600;text-transform:uppercase;letter-spacing:0.08em;color:#c45a2c;}" +
            "#wtBar{width:160px;height:3px;background:#1e1e1e;border-radius:2px;overflow:hidden;}" +
            "#wtProgress{height:100%;background:#c45a2c;border-radius:2px;width:0%;transition:width 0.3s linear;}" +
            "#wtStep{font-size:0.72rem;color:#666;max-width:200px;white-space:nowrap;}" +
            "#wtFade{position:fixed;inset:0;z-index:10001;background:#080808;opacity:0;" +
            "transition:opacity 0.6s ease;pointer-events:none;}" +
            "#wtFade.active{opacity:1;pointer-events:all;}";
        document.head.appendChild(s);
    }

    function smoothScroll(targetY, duration) {
        return new Promise(function (resolve) {
            var startY = window.scrollY;
            var diff = targetY - startY;
            if (Math.abs(diff) < 5) { resolve(); return; }
            var start = performance.now();
            function tick(now) {
                var t = Math.min((now - start) / duration, 1);
                var eased = 1 - Math.pow(1 - t, 3);
                window.scrollTo(0, startY + diff * eased);
                if (t < 1) requestAnimationFrame(tick);
                else resolve();
            }
            requestAnimationFrame(tick);
        });
    }

    function sectionTop(id) {
        var el = document.getElementById(id);
        return el ? el.offsetTop - 80 : 0;
    }

    function wait(ms) {
        return new Promise(function (r) { setTimeout(r, ms); });
    }

    window.startFullWalkthrough = function () {
        if (running) return;
        running = true;

        var btn = document.getElementById("watchDemoBtn");
        if (btn) { btn.disabled = true; btn.style.opacity = "0.5"; btn.style.pointerEvents = "none"; }

        injectStyles();

        document.body.style.overflow = "hidden";

        var overlay = document.createElement("div");
        overlay.id = "wtOverlay";
        overlay.innerHTML =
            '<span id="wtLabel">demo</span>' +
            '<div id="wtBar"><div id="wtProgress"></div></div>' +
            '<span id="wtStep">starting...</span>';
        document.body.appendChild(overlay);

        var fade = document.createElement("div");
        fade.id = "wtFade";
        document.body.appendChild(fade);

        requestAnimationFrame(function () {
            overlay.classList.add("visible");
        });

        var progress = document.getElementById("wtProgress");
        var step = document.getElementById("wtStep");

        function setP(pct, text) {
            progress.style.width = pct + "%";
            step.textContent = text;
        }

        window.scrollTo(0, 0);

        (async function run() {
            try {
                await wait(400);

                setP(5, "mesh platform");
                await wait(1600);

                setP(20, "the problem");
                await smoothScroll(sectionTop("problem"), 1000);
                await wait(2500);

                setP(45, "architecture");
                await smoothScroll(sectionTop("architecture"), 1000);
                await wait(2000);

                setP(65, "stateful agents");
                await smoothScroll(sectionTop("agents"), 1000);
                await wait(2000);

                setP(82, "live demo");
                await smoothScroll(sectionTop("demo"), 800);
                await wait(700);

                setP(95, "loading dashboard...");
                fade.classList.add("active");
                await wait(800);

                setP(100, "");
                await wait(200);

                window.location.href = "cluster.html?demo=1";
            } catch (e) {
                document.body.style.overflow = "";
                overlay.remove();
                fade.remove();
                running = false;
            }
        })();
    };
})();
