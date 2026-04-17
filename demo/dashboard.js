(function () {
  'use strict';

  function initCharts() {
    var cpuChart = document.getElementById('cpuChart');
    if (cpuChart) {
      var values = [23, 61, 15, 38, 52, 45, 33, 67, 29, 41, 55, 48];
      var maxVal = Math.max.apply(null, values);
      values.forEach(function (v) {
        var bar = document.createElement('div');
        bar.className = 'chart-bar';
        var pct = (v / maxVal) * 100;
        bar.style.height = '0%';
        var color = v > 60 ? 'var(--accent-red)' : v > 40 ? 'var(--accent-yellow)' : 'var(--accent-blue)';
        bar.style.background = color;
        bar.style.opacity = '0.7';
        cpuChart.appendChild(bar);
        setTimeout(function () {
          bar.style.height = pct + '%';
        }, 100);
      });
    }

    var memChart = document.getElementById('memChart');
    if (memChart) {
      var memValues = [45, 72, 34, 58, 63, 51, 44, 69, 41, 56, 62, 53];
      var memMax = Math.max.apply(null, memValues);
      memValues.forEach(function (v) {
        var bar = document.createElement('div');
        bar.className = 'chart-bar';
        var pct = (v / memMax) * 100;
        bar.style.height = '0%';
        var color = v > 65 ? 'var(--accent-red)' : v > 45 ? 'var(--accent-yellow)' : 'var(--accent-green)';
        bar.style.background = color;
        bar.style.opacity = '0.7';
        memChart.appendChild(bar);
        setTimeout(function () {
          bar.style.height = pct + '%';
        }, 200);
      });
    }
  }

  function initMetricAnimations() {
    document.querySelectorAll('.metric-bar-fill').forEach(function (el) {
      var target = el.getAttribute('data-width');
      if (target) {
        el.style.width = '0%';
        setTimeout(function () {
          el.style.width = target + '%';
        }, 300);
      }
    });
  }

  window.addEventListener('DOMContentLoaded', function () {
    initCharts();
    initMetricAnimations();
  });
})();
