// DevTrack Wiki - Modern Interactive Features
document.addEventListener('DOMContentLoaded', function() {
    // ===== DARK MODE =====
    const darkModeToggle = document.getElementById('darkModeToggle');
    const html = document.documentElement;
    
    // Check for saved theme preference or default to light mode
    const currentTheme = localStorage.getItem('theme') || 'light';
    html.setAttribute('data-theme', currentTheme);
    updateDarkModeIcon(currentTheme);
    
    darkModeToggle.addEventListener('click', function() {
        const currentTheme = html.getAttribute('data-theme');
        const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
        
        html.setAttribute('data-theme', newTheme);
        localStorage.setItem('theme', newTheme);
        updateDarkModeIcon(newTheme);
    });
    
    function updateDarkModeIcon(theme) {
        const icon = darkModeToggle.querySelector('i');
        if (theme === 'dark') {
            icon.className = 'fas fa-sun';
        } else {
            icon.className = 'fas fa-moon';
        }
    }
    
    // ===== SIDEBAR COLLAPSE =====
    const sidebarCollapse = document.getElementById('sidebarCollapse');
    const sidebarToggle = document.getElementById('sidebarToggle');
    const body = document.body;
    
    // Check for saved sidebar state
    const sidebarState = localStorage.getItem('sidebarCollapsed');
    if (sidebarState === 'true') {
        body.classList.add('sidebar-collapsed');
    }
    
    sidebarCollapse.addEventListener('click', function() {
        body.classList.toggle('sidebar-collapsed');
        localStorage.setItem('sidebarCollapsed', body.classList.contains('sidebar-collapsed'));
    });
    
    sidebarToggle.addEventListener('click', function() {
        body.classList.remove('sidebar-collapsed');
        localStorage.setItem('sidebarCollapsed', 'false');
    });
    
    // ===== TAB NAVIGATION =====
    const navItems = document.querySelectorAll('.nav-item[data-tab]');
    const tabContents = document.querySelectorAll('.tab-content');
    
    navItems.forEach(item => {
        item.addEventListener('click', function(e) {
            e.preventDefault();
            
            const targetTab = this.getAttribute('data-tab');
            
            // Remove active class from all nav items and tab contents
            navItems.forEach(nav => nav.classList.remove('active'));
            tabContents.forEach(content => content.classList.remove('active'));
            
            // Add active class to clicked nav item and corresponding tab content
            this.classList.add('active');
            const targetContent = document.getElementById(targetTab);
            if (targetContent) {
                targetContent.classList.add('active');
                
                // Update URL hash without scrolling
                history.pushState(null, null, '#' + targetTab);
                
                // Scroll to top of content
                window.scrollTo({ top: 0, behavior: 'smooth' });
            }
        });
    });
    
    // Handle initial load with hash
    function loadTabFromHash() {
        const hash = window.location.hash.substring(1);
        if (hash) {
            const targetNav = document.querySelector(`.nav-item[data-tab="${hash}"]`);
            if (targetNav) {
                targetNav.click();
            }
        }
    }
    
    loadTabFromHash();
    window.addEventListener('hashchange', loadTabFromHash);
    
    // ===== CODE BLOCK COPY FUNCTIONALITY =====
    const codeBlocks = document.querySelectorAll('.code-block');
    codeBlocks.forEach(block => {
        const header = block.querySelector('.code-header');
        if (header) {
            const copyButton = document.createElement('button');
            copyButton.className = 'copy-btn';
            copyButton.innerHTML = '<i class="fas fa-copy"></i> Copy';
            
            copyButton.addEventListener('click', () => {
                const code = block.querySelector('code');
                if (code) {
                    navigator.clipboard.writeText(code.textContent).then(() => {
                        copyButton.innerHTML = '<i class="fas fa-check"></i> Copied!';
                        setTimeout(() => {
                            copyButton.innerHTML = '<i class="fas fa-copy"></i> Copy';
                        }, 2000);
                    }).catch(err => {
                        console.error('Failed to copy:', err);
                    });
                }
            });
            
            header.appendChild(copyButton);
        }
    });
    
    // ===== SMOOTH SCROLL FOR ANCHOR LINKS =====
    document.querySelectorAll('a[href^="#"]').forEach(anchor => {
        anchor.addEventListener('click', function(e) {
            const targetId = this.getAttribute('href').substring(1);
            const targetElement = document.getElementById(targetId);
            
            // Only smooth scroll if it's not a tab navigation
            if (targetElement && !this.hasAttribute('data-tab')) {
                e.preventDefault();
                targetElement.scrollIntoView({
                    behavior: 'smooth',
                    block: 'start'
                });
            }
        });
    });
    
    // ===== MOBILE RESPONSIVENESS =====
    function handleResize() {
        if (window.innerWidth <= 768) {
            // On mobile, show toggle button
            sidebarToggle.style.display = 'flex';
        } else {
            // On desktop, hide toggle button unless sidebar is collapsed
            if (!body.classList.contains('sidebar-collapsed')) {
                sidebarToggle.style.display = 'none';
            }
        }
    }
    
    handleResize();
    window.addEventListener('resize', handleResize);
    
    // ===== KEYBOARD SHORTCUTS =====
    document.addEventListener('keydown', function(e) {
        // Ctrl/Cmd + K to toggle dark mode
        if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
            e.preventDefault();
            darkModeToggle.click();
        }
        
        // Ctrl/Cmd + B to toggle sidebar
        if ((e.ctrlKey || e.metaKey) && e.key === 'b') {
            e.preventDefault();
            sidebarCollapse.click();
        }
    });
    
    // ===== PLATFORM TAB SWITCHING =====
    const platformTabs = document.querySelectorAll('.platform-tab');
    platformTabs.forEach(tab => {
        tab.addEventListener('click', function() {
            const platform = this.getAttribute('data-platform');
            
            // Remove active class from all tabs and content
            document.querySelectorAll('.platform-tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.platform-content').forEach(c => c.classList.remove('active'));
            
            // Add active class to clicked tab and corresponding content
            this.classList.add('active');
            const content = document.getElementById(`platform-${platform}`);
            if (content) {
                content.classList.add('active');
            }
        });
    });
    
    // ===== ANALYTICS (PLACEHOLDER) =====
    const trackPageView = (page) => {
        console.log('Page view:', page);
        // Integration point for analytics service
    };
    
    trackPageView(window.location.pathname + window.location.hash);
    
    navItems.forEach(item => {
        item.addEventListener('click', () => {
            trackPageView(window.location.pathname + '#' + item.getAttribute('data-tab'));
        });
    });
    
    // ===== SCROLL TO TOP BUTTON =====
    const scrollToTopBtn = document.createElement('button');
    scrollToTopBtn.className = 'scroll-to-top';
    scrollToTopBtn.innerHTML = '<i class="fas fa-arrow-up"></i>';
    scrollToTopBtn.style.cssText = `
        position: fixed;
        bottom: 2rem;
        right: 2rem;
        width: 48px;
        height: 48px;
        border-radius: 50%;
        background: var(--primary);
        color: white;
        border: none;
        cursor: pointer;
        display: none;
        align-items: center;
        justify-content: center;
        box-shadow: var(--shadow-lg);
        transition: all 0.3s ease;
        z-index: 999;
    `;
    
    scrollToTopBtn.addEventListener('click', () => {
        window.scrollTo({ top: 0, behavior: 'smooth' });
    });
    
    document.body.appendChild(scrollToTopBtn);
    
    window.addEventListener('scroll', () => {
        if (window.scrollY > 300) {
            scrollToTopBtn.style.display = 'flex';
        } else {
            scrollToTopBtn.style.display = 'none';
        }
    });
    
    scrollToTopBtn.addEventListener('mouseenter', () => {
        scrollToTopBtn.style.transform = 'scale(1.1)';
    });
    
    scrollToTopBtn.addEventListener('mouseleave', () => {
        scrollToTopBtn.style.transform = 'scale(1)';
    });
    
    console.log('DevTrack Wiki initialized successfully');
    console.log('Keyboard shortcuts:');
    console.log('  Ctrl/Cmd + K: Toggle dark mode');
    console.log('  Ctrl/Cmd + B: Toggle sidebar');
});
