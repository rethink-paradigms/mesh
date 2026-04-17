(function () {
  'use strict';

  function initNav() {
    var nav = document.querySelector('nav');
    var toggle = document.querySelector('.nav-mobile-toggle');
    var links = document.querySelector('.nav-links');
    if (!nav) return;

    window.addEventListener('scroll', function () {
      if (window.scrollY > 10) {
        nav.classList.add('scrolled');
      } else {
        nav.classList.remove('scrolled');
      }
    }, { passive: true });

    if (toggle && links) {
      toggle.addEventListener('click', function () {
        links.classList.toggle('open');
        var expanded = links.classList.contains('open');
        toggle.setAttribute('aria-expanded', expanded);
      });

      links.querySelectorAll('a').forEach(function (link) {
        link.addEventListener('click', function () {
          links.classList.remove('open');
          toggle.setAttribute('aria-expanded', 'false');
        });
      });
    }

    var currentPage = window.location.pathname.split('/').pop() || 'index.html';
    document.querySelectorAll('.nav-links a').forEach(function (link) {
      var href = link.getAttribute('href');
      if (href === currentPage || (currentPage === '' && href === 'index.html')) {
        link.classList.add('active');
      }
    });
  }

  function initScrollAnimations() {
    var observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add('visible');
          observer.unobserve(entry.target);
        }
      });
    }, { threshold: 0.1, rootMargin: '0px 0px -40px 0px' });

    document.querySelectorAll('[data-animate]').forEach(function (el) {
      observer.observe(el);
    });
  }

  function initLoading() {
    var overlay = document.querySelector('.loading-overlay');
    if (overlay) {
      window.addEventListener('load', function () {
        setTimeout(function () {
          overlay.classList.add('hidden');
        }, 300);
      });
    }
  }

  function initTheme() {
    var style = document.createElement('style');
    style.textContent =
      '[data-animate] { opacity: 0; transform: translateY(20px); transition: opacity 0.5s ease, transform 0.5s ease; }' +
      '[data-animate].visible { opacity: 1; transform: translateY(0); }';
    document.head.appendChild(style);
  }

  document.addEventListener('DOMContentLoaded', function () {
    initTheme();
    initNav();
    initScrollAnimations();
    initLoading();
  });
})();
