# DevTrack Wiki

A comprehensive, modern knowledge base for the DevTrack Developer Automation Tools.

## Quick Start

Open `index.html` in your browser to view the complete wiki:

```bash
# From the project root
open wiki/index.html

# Or using a simple HTTP server
python3 -m http.server 8000
# Then visit: http://localhost:8000/wiki/
```

## Wiki Structure

### Main Wiki (`index.html`)
Comprehensive documentation with tabbed navigation covering:

- **Overview** - System introduction and architecture
- **Getting Started** - Installation and quick start guide
- **Architecture** - Technical architecture and data flow
- **Features** - Complete feature descriptions
- **Daemon** - Background daemon documentation
- **Git Integration** - Git monitoring system
- **Scheduler** - Time-based triggers
- **IPC Communication** - Inter-process communication
- **Personalized AI** - AI learning system
- **Integrations** - Azure DevOps, GitHub, Microsoft Graph
- **Notifications** - Email and Teams configuration
- **Commands Reference** - Complete command list
- **Configuration** - Configuration guide
- **Roadmap** - Development roadmap

### Privacy Policy (`privacy.html`)
Dedicated privacy documentation covering:

- **Privacy Overview** - Core privacy principles
- **Data Collection** - What data is collected and why
- **Consent Management** - How consent works
- **Data Storage** - Where and how data is stored
- **AI Processing** - Local AI with Ollama
- **User Rights** - Your rights regarding data
- **Security Measures** - Security best practices

## Features

### Modern Design
- Clean, professional interface
- Responsive layout for all screen sizes
- Dark sidebar with light content area
- Smooth animations and transitions
- Syntax-highlighted code blocks

### Navigation
- Tabbed interface for easy browsing
- Persistent sidebar navigation
- Direct linking to specific sections
- Browser back/forward support
- Mobile-friendly menu

### User Experience
- Copy buttons on code blocks
- Searchable content
- Keyboard navigation support
- Print-friendly layout
- Accessible design (WCAG compliant)

### Content Organization
- Logical section grouping
- Progressive disclosure
- Visual hierarchy
- Cross-references between sections
- Comprehensive examples

## Assets

### `assets/style.css`
Modern CSS with:
- CSS custom properties for theming
- Responsive grid layouts
- Component styling (cards, boxes, tables)
- Syntax highlighting
- Print styles

### `assets/script.js`
Interactive features:
- Tab navigation system
- Code block copy functionality
- URL hash management
- Smooth scrolling
- Mobile menu toggle

## Customization

### Updating Content
Edit the HTML files directly. Each section is contained in a `<section>` tag with the class `tab-content`.

### Styling
Modify `assets/style.css`. Key CSS variables in `:root`:
```css
--primary-color: #2563eb;
--sidebar-bg: #1e293b;
--content-bg: #ffffff;
```

### Adding New Sections
1. Add navigation item in sidebar:
```html
<a href="#new-section" class="nav-item" data-tab="new-section">New Section</a>
```

2. Add content section:
```html
<section id="new-section" class="tab-content">
    <h1>New Section Title</h1>
    <!-- Content here -->
</section>
```

## Browser Compatibility

Tested and working on:
- Chrome 90+
- Firefox 88+
- Safari 14+
- Edge 90+

## Contributing

When updating documentation:
1. Maintain consistent formatting
2. Test on multiple browsers
3. Verify all links work
4. Check mobile responsiveness
5. Update this README if adding new pages

## License

Same as the main project (MIT License).

## Credits

Built with vanilla HTML, CSS, and JavaScript - no frameworks required for fast loading and broad compatibility.
