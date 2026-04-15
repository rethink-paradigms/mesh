const { chromium } = require('playwright');
const path = require('path');

const DEMO_DIR = __dirname;
const OUTPUT = DEMO_DIR + '/mesh-demo-30s.mp4';
const URL = 'http://localhost:8765';
const WIDTH = 1920;
const HEIGHT = 1080;
const TOTAL_MS = 34000;

(async () => {
    const browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({
        viewport: { width: WIDTH, height: HEIGHT },
        recordVideo: { dir: DEMO_DIR, size: { width: WIDTH, height: HEIGHT } }
    });
    const page = await context.newPage();

    console.log('Phase 1: Landing page...');
    await page.goto(URL + '/index.html', { waitUntil: 'networkidle' });
    await page.waitForTimeout(1000);

    await page.click('#watchDemoBtn');
    console.log('Demo started. Recording...');

    await page.waitForTimeout(TOTAL_MS);

    const video = page.video();
    const path = await video.path();
    console.log('Raw video saved to:', path);

    await context.close();
    await browser.close();

    const fs = require('fs');
    fs.renameSync(path, OUTPUT);
    console.log('Video saved to:', OUTPUT);
})();
