// ── Terminal typewriter ───────────────────────────────────────────────────────
const sequences = [
  {
    // The silent path — you commit, the daemon does the rest, nothing interrupts
    cmd: 'git commit -m "fix auth redirect"',
    lines: [
      { text: '[main 3f9a1c2] fix auth redirect', cls: 'term-out', delay: 500 },
      { text: '', cls: 'term-out', delay: 700 },
      { text: '# nothing else happens. no prompt, no pause.', cls: 'term-out', delay: 1300 },
      { text: '# in the background: PROJ-142 inferred from the branch,', cls: 'term-out', delay: 1900 },
      { text: '# comment drafted in your voice, staged for review.', cls: 'term-out', delay: 2400 },
    ]
  },
  {
    // The queue — the trust primitive
    cmd: 'devtrack queue list',
    lines: [
      { text: '2 actions pending review', cls: 'term-info', delay: 400 },
      { text: '  1  PROJ-142  comment  "Fixed the OAuth redirect loop..."', cls: 'term-out', delay: 800 },
      { text: '  2  PROJ-142  status   In Progress → In Review', cls: 'term-out', delay: 1100 },
      { text: '✓ approve 1  ·  edit 1  ·  reject 1', cls: 'term-success', delay: 1600 },
    ]
  },
  {
    // EOD
    cmd: 'devtrack eod show',
    lines: [
      { text: 'Today · 6 commits · 2 tickets', cls: 'term-info', delay: 400 },
      { text: '  PROJ-142  Fixed the OAuth redirect loop on token expiry.', cls: 'term-out', delay: 900 },
      { text: '  PROJ-151  Started the retry backoff — not finished yet.', cls: 'term-out', delay: 1300 },
      { text: '✓ Written before you asked.', cls: 'term-success', delay: 1800 },
    ]
  },
  {
    // MCP — the memory your agent lacks
    cmd: 'devtrack mcp setup',
    lines: [
      { text: '✦ Registering DevTrack with Claude Code...', cls: 'term-info', delay: 500 },
      { text: '✓ 6 read-only tools exposed over MCP', cls: 'term-success', delay: 1100 },
      { text: '', cls: 'term-out', delay: 1300 },
      { text: '# now ask your agent: "what am I working on?"', cls: 'term-out', delay: 1900 },
    ]
  },
];

let seqIdx = 0;
const cmdEl = document.getElementById('termCmd');
const cursorEl = document.getElementById('termCursor');
const outputEl = document.getElementById('termOutput');

function typeSequence(seq) {
  cmdEl.textContent = '';
  outputEl.innerHTML = '';
  const cmd = seq.cmd;
  let i = 0;
  const type = () => {
    if (i < cmd.length) {
      cmdEl.textContent += cmd[i++];
      setTimeout(type, 38 + Math.random() * 20);
    } else {
      cursorEl.style.display = 'none';
      seq.lines.forEach(line => {
        setTimeout(() => {
          const div = document.createElement('div');
          div.className = 'term-line';
          div.innerHTML = `<span class="${line.cls}">${line.text}</span>`;
          div.style.opacity = '0';
          div.style.transform = 'translateY(4px)';
          div.style.transition = 'opacity .3s, transform .3s';
          outputEl.appendChild(div);
          requestAnimationFrame(() => {
            div.style.opacity = '1';
            div.style.transform = 'translateY(0)';
          });
        }, line.delay);
      });
      const totalDelay = seq.lines.reduce((m, l) => Math.max(m, l.delay), 0) + 2400;
      setTimeout(() => {
        outputEl.innerHTML = '';
        cmdEl.textContent = '';
        cursorEl.style.display = 'inline-block';
        seqIdx = (seqIdx + 1) % sequences.length;
        setTimeout(() => typeSequence(sequences[seqIdx]), 300);
      }, totalDelay);
    }
  };
  setTimeout(type, 400);
}

typeSequence(sequences[0]);

// ── Pipeline terminal ─────────────────────────────────────────────────────────
const pipelineLines = [
  { text: '$ git commit -m "add payment gateway"', cls: 'term-cmd', delay: 0 },
  { text: '', delay: 200 },
  { text: '[git_monitor] commit detected', cls: 'term-info', delay: 600 },
  { text: '[ipc] → python_bridge: commit_trigger', cls: 'term-out', delay: 1000 },
  { text: '[nlp] extracting entities...', cls: 'term-info', delay: 1400 },
  { text: '[nlp] ticket: PROJ-88, time: 2h', cls: 'term-success', delay: 1800 },
  { text: '[llm] enhancing message (ollama)...', cls: 'term-info', delay: 2200 },
  { text: '[llm] personalization: 3 RAG examples', cls: 'term-info', delay: 2600 },
  { text: '[azure] matching work item...', cls: 'term-info', delay: 3000 },
  { text: '[azure] → PROJ-88 found: "Payment API"', cls: 'term-success', delay: 3400 },
  { text: '[azure] commenting + transitioning state', cls: 'term-info', delay: 3800 },
  { text: '[azure] ✓ In Progress → Active', cls: 'term-success', delay: 4200 },
  { text: '[telegram] ✓ notification sent', cls: 'term-success', delay: 4600 },
  { text: '', delay: 5000 },
  { text: '✓ Pipeline complete (4.8s)', cls: 'term-success', delay: 5200 },
];

const pipelineBody = document.getElementById('pipelineBody');
let pipelineRunning = false;

function runPipeline() {
  if (pipelineRunning) return;
  pipelineRunning = true;
  pipelineBody.innerHTML = '';

  pipelineLines.forEach(line => {
    setTimeout(() => {
      if (!line.text) { pipelineBody.appendChild(document.createElement('br')); return; }
      const div = document.createElement('div');
      div.className = 'term-line';
      div.innerHTML = `<span class="${line.cls}">${line.text}</span>`;
      div.style.opacity = '0';
      div.style.transition = 'opacity .25s';
      pipelineBody.appendChild(div);
      requestAnimationFrame(() => div.style.opacity = '1');
    }, line.delay);
  });

  setTimeout(() => {
    pipelineRunning = false;
    setTimeout(runPipeline, 3000);
  }, pipelineLines[pipelineLines.length - 1].delay + 3000);
}

// Start pipeline when section scrolls into view
const pipelineObs = new IntersectionObserver(entries => {
  entries.forEach(e => { if (e.isIntersecting) runPipeline(); });
}, { threshold: .3 });
pipelineObs.observe(document.querySelector('.how-visual'));

// ── How steps cycling ──────────────────────────────────────────────────────────
let howStep = 0;
const steps = document.querySelectorAll('.how-step');
setInterval(() => {
  steps.forEach(s => s.classList.remove('active'));
  howStep = (howStep + 1) % steps.length;
  steps[howStep].classList.add('active');
}, 2200);

// ── Marquee ───────────────────────────────────────────────────────────────────
const techs = [
  { label: 'Go 1.21',        color: '#00add8' },
  { label: 'Python 3.12',    color: '#3b82f6' },
  { label: 'Ollama',         color: '#8b5cf6' },
  { label: 'SQLite',         color: '#22c55e' },
  { label: 'spaCy',          color: '#14b8a6' },
  { label: 'ChromaDB',       color: '#f59e0b' },
  { label: 'fsnotify',       color: '#ef4444' },
  { label: 'Azure DevOps',   color: '#0078d4' },
  { label: 'GitLab API',     color: '#fc6d26' },
  { label: 'Telegram Bot',   color: '#2aabee' },
  { label: 'OpenAI SDK',     color: '#10b981' },
  { label: 'FastAPI',        color: '#8b5cf6' },
  { label: 'robfig/cron',    color: '#f59e0b' },
  { label: 'MS Graph',       color: '#0078d4' },
  { label: 'uv',             color: '#6366f1' },
];
const marquee = document.getElementById('marquee');
// Duplicate for infinite loop
[...techs, ...techs].forEach(t => {
  const pill = document.createElement('div');
  pill.className = 'tech-pill';
  pill.innerHTML = `<span class="tech-dot" style="background:${t.color}"></span>${t.label}`;
  marquee.appendChild(pill);
});

// ── Scroll reveal ──────────────────────────────────────────────────────────────
const revealObs = new IntersectionObserver(entries => {
  entries.forEach(e => { if (e.isIntersecting) e.target.classList.add('visible'); });
}, { threshold: .1 });
document.querySelectorAll('.reveal').forEach(el => revealObs.observe(el));

// ── Dev Banner ────────────────────────────────────────────────────────────────
function dismissBanner() {
  document.getElementById('devBanner').classList.add('hidden');
  document.body.classList.remove('banner-visible');
  sessionStorage.setItem('banner-dismissed', '1');
}
if (!sessionStorage.getItem('banner-dismissed')) {
  document.body.classList.add('banner-visible');
} else {
  document.getElementById('devBanner').classList.add('hidden');
}

// ── Contact form (static — opens mailto fallback) ─────────────────────────────
function handleFormSubmit(btn) {
  const form = btn.closest('.contact-form');
  const name    = form.querySelector('input[type=text]').value.trim();
  const email   = form.querySelector('input[type=email]').value.trim();
  const topic   = form.querySelector('select').value;
  const message = form.querySelector('textarea').value.trim();
  if (!name || !email || !message) {
    btn.textContent = 'Please fill all fields';
    btn.style.background = '#7f1d1d';
    setTimeout(() => { btn.textContent = 'Send Message'; btn.style.background = ''; }, 2000);
    return;
  }
  const subject = encodeURIComponent(`[DevTrack] ${topic || 'Message'} from ${name}`);
  const body    = encodeURIComponent(`Name: ${name}\nEmail: ${email}\nTopic: ${topic}\n\n${message}`);
  window.location.href = `mailto:admin@mogrov.com?subject=${subject}&body=${body}`;
}

// ── Hero typewriter word cycle ────────────────────────────────────────────────
(function() {
  const words = ['standup', 'ticket update', 'EOD report', 'status email', 'progress note'];
  let wordIdx = 0, charIdx = 0, deleting = false;
  const el = document.getElementById('typewriter-word');
  const TYPE_SPEED = 85, DELETE_SPEED = 45, PAUSE = 2000;

  function tick() {
    const word = words[wordIdx];
    if (!deleting) {
      el.textContent = word.slice(0, ++charIdx);
      if (charIdx === word.length) { deleting = true; setTimeout(tick, PAUSE); return; }
    } else {
      el.textContent = word.slice(0, --charIdx);
      if (charIdx === 0) { deleting = false; wordIdx = (wordIdx + 1) % words.length; }
    }
    setTimeout(tick, deleting ? DELETE_SPEED : TYPE_SPEED);
  }
  tick();
})();

// ── Nav scroll tint ────────────────────────────────────────────────────────────
window.addEventListener('scroll', () => {
  const nav = document.getElementById('navbar');
  nav.style.background = window.scrollY > 20
    ? 'rgba(8,8,8,.92)' : 'rgba(8,8,8,.7)';
}, { passive: true });
