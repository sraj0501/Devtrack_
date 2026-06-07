/* ── THEME TOGGLE ── */
const toggleBtn = document.getElementById('theme-toggle');
let isDark = localStorage.getItem('theme') !== 'light';

function applyTheme(dark, rerender) {
  isDark = dark;
  document.body.classList.toggle('light-mode', !dark);
  toggleBtn.textContent = dark ? '☀️' : '🌙';
  localStorage.setItem('theme', dark ? 'dark' : 'light');
  if (rerender) renderMermaid(dark);
}

applyTheme(isDark, false);
toggleBtn.addEventListener('click', () => applyTheme(!isDark, true));

/* ── MERMAID ── */
function mermaidConfig(dark) {
  return {
    startOnLoad: false,
    theme: dark ? 'dark' : 'default',
    flowchart: { htmlLabels: true, curve: 'basis', padding: 20 },
    fontSize: 15,
    securityLevel: 'loose',
  };
}

document.querySelectorAll('.mermaid').forEach(el => {
  el.dataset.src = el.textContent.trim();
});

async function renderMermaid(dark) {
  mermaid.initialize(mermaidConfig(dark === undefined ? isDark : dark));
  document.querySelectorAll('.mermaid').forEach(el => {
    if (el.dataset.src) {
      el.innerHTML = el.dataset.src;
      el.removeAttribute('data-processed');
      el.style.transform = '';
    }
  });
  await mermaid.run({ querySelector: '.mermaid' });
  // Two rAFs ensure the SVG has fully painted before measuring dimensions
  requestAnimationFrame(() => requestAnimationFrame(initDiagramZoom));
}

renderMermaid(isDark);

/* ── PER-DIAGRAM PAN / ZOOM ── */
function initDiagramZoom() {
  document.querySelectorAll('.diagram-wrap').forEach(wrap => {
    const viewport  = wrap.querySelector('.diagram-viewport');
    const mermaidEl = wrap.querySelector('.mermaid');
    const levelEl   = wrap.querySelector('.zoom-level');
    if (!viewport || !mermaidEl) return;

    // Tear down any previous ResizeObserver stored on the element
    if (wrap._zoomRO) { wrap._zoomRO.disconnect(); }

    let scale = 1, tx = 0, ty = 0;
    let dragging = false, startX, startY, startTX, startTY;

    function getSvgNaturalWidth() {
      const svg = mermaidEl.querySelector('svg');
      if (!svg) return 600;
      const vb = svg.viewBox?.baseVal;
      if (vb && vb.width > 0) return vb.width;
      const w = parseFloat(svg.getAttribute('width'));
      return (w && w > 0) ? w : svg.getBoundingClientRect().width || 600;
    }

    const defaultZoom = parseFloat(wrap.dataset.defaultZoom);

    function fitScale() {
      const available = viewport.clientWidth - 56;
      return Math.min(1.5, Math.max(0.15, available / getSvgNaturalWidth()));
    }

    function apply() {
      mermaidEl.style.transform = `translate(${tx}px, ${ty}px) scale(${scale})`;
      if (levelEl) levelEl.textContent = Math.round(scale * 100) + '%';
    }

    function fit() { scale = fitScale(); tx = 0; ty = 0; apply(); }
    function applyDefault() { scale = isNaN(defaultZoom) ? fitScale() : defaultZoom; tx = 0; ty = 0; apply(); }

    applyDefault();

    // Zoom buttons
    wrap.querySelectorAll('.zoom-btn').forEach(btn => {
      btn.addEventListener('click', e => {
        e.stopPropagation();
        const a = btn.dataset.action;
        if      (a === 'in')  scale = Math.min(scale * 1.3, 5);
        else if (a === 'out') scale = Math.max(scale / 1.3, 0.1);
        else                { fit(); return; }
        apply();
      });
    });

    // Mouse-wheel zoom toward cursor
    viewport.addEventListener('wheel', e => {
      e.preventDefault();
      const rect  = viewport.getBoundingClientRect();
      const mx    = e.clientX - rect.left;
      const my    = e.clientY - rect.top;
      const delta = e.deltaY < 0 ? 1.12 : 0.89;
      const ns    = Math.max(0.1, Math.min(5, scale * delta));
      tx = mx - (mx - tx) * (ns / scale);
      ty = my - (my - ty) * (ns / scale);
      scale = ns;
      apply();
    }, { passive: false });

    // Drag to pan
    viewport.addEventListener('mousedown', e => {
      dragging = true;
      startX = e.clientX; startY = e.clientY;
      startTX = tx; startTY = ty;
      viewport.classList.add('grabbing');
      e.preventDefault();
    });
    window.addEventListener('mousemove', e => {
      if (!dragging) return;
      tx = startTX + (e.clientX - startX);
      ty = startTY + (e.clientY - startY);
      apply();
    });
    window.addEventListener('mouseup', () => {
      if (!dragging) return;
      dragging = false;
      viewport.classList.remove('grabbing');
    });

    // Re-apply default on resize; skip the first (synchronous) observation
    // so it doesn't override the configured default zoom on load.
    let skipFirst = true;
    const ro = new ResizeObserver(() => {
      if (skipFirst) { skipFirst = false; return; }
      applyDefault();
    });
    ro.observe(viewport);
    wrap._zoomRO = ro;
  });
}

/* ── ACTIVE NAV LINK ── */
const links    = document.querySelectorAll('.nav-link');
const sections = document.querySelectorAll('.section-anchor');

const observer = new IntersectionObserver((entries) => {
  entries.forEach(entry => {
    if (entry.isIntersecting) {
      const id = entry.target.id;
      links.forEach(l => l.classList.remove('active'));
      const active = document.querySelector(`.nav-link[href="#${id}"]`);
      if (active) active.classList.add('active');
    }
  });
}, { rootMargin: '-20% 0px -70% 0px' });

sections.forEach(s => observer.observe(s));
