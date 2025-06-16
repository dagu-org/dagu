# Dagu Documentation Specification v2

## Overview

This specification defines a modern, developer-friendly documentation site for Dagu. The focus is on practical, task-oriented content that gets users productive quickly while providing comprehensive references when needed.

### The 7 Sections

1. **Overview** - What is Dagu and why use it
2. **Getting Started** - Install and run your first workflow
3. **Examples** - Visual index of all features
4. **Writing Workflows** - Learn to build production DAGs
5. **Features** - Deep dive into capabilities
6. **Configurations** - Deploy and operate Dagu
7. **Reference** - Look up commands, syntax, and troubleshooting

## Core Principles

- **Developer Journey Focused**: Content organized by what users need to do, not what Dagu can do
- **Examples First**: Show code before explaining theory
- **Progressive Disclosure**: Start simple, add complexity gradually
- **Clean Navigation**: 7 focused sections, no more
- **Complete Coverage**: All features documented, nothing hidden
- **Copy-Paste Ready**: Examples that work out of the box
- **Modern & Cool**: Documentation that developers actually want to read

## Top-Level Structure

```
- Overview
- Getting Started
- Examples
- Writing Workflows
- Features
- Configurations
- Reference
```

## Detailed Sections

### Overview
*What is Dagu and why should you care?*

- **The Problem** - Legacy cron jobs scattered everywhere, no visibility
- **The Solution** - Modern workflow orchestration in a single binary
- **Key Features** - What makes Dagu different
  - Zero dependencies (no database, no message broker)
  - Hierarchical DAG composition
  - Language agnostic
  - Single binary installation
- **Architecture** - How it works
  - Unix socket communication
  - File-based storage
  - Built-in queue system
- **Use Cases** - Who uses Dagu
  - Data engineering pipelines
  - DevOps automation
  - Business process automation
- **Comparison** - Dagu vs other tools (Airflow, Cron, GitHub Actions)

### Getting Started
*From zero to running workflows in minutes*

- **Installation**
  - One-line install script
  - Docker installation
  - Package managers (Homebrew, etc.)
  - Verify installation
  
- **Quick Start**
  - Create your first DAG
  - Run with CLI (`dagu start`)
  - View in the Web UI
  - Check status via API
  
- **Core Concepts**
  - What is a DAG?
  - Steps and dependencies
  - Workflow execution model
  - Three ways to interact: CLI, Web UI, API
  
- **Next Steps**
  - Explore examples
  - Choose your interface
  - Build your first real workflow

### Examples
*Visual index of all Dagu features - copy, paste, and modify*

Quick examples showing every feature:
- Basic sequential workflow
- Parallel execution
- Conditional execution
- Retry and repeat patterns
- Docker workflows
- SSH remote execution
- HTTP requests
- Email notifications
- Nested DAGs
- Error handling
- And more...

Each example is minimal (5-10 lines) with a link to detailed documentation.

### Writing Workflows
*Everything you need to build production workflows*

- **Basics**
  - YAML structure
  - Defining steps
  - Command execution
  - Dependencies
  
- **Control Flow**
  - Serial vs parallel execution
  - Conditional execution
  - Continue on failure
  - Repeat patterns
  
- **Data & Variables**
  - Parameters
  - Environment variables
  - Passing data between steps
  - Output variables
  - Template rendering
  
- **Error Handling**
  - Retry policies
  - Lifecycle handlers
  - Error recovery patterns
  - Notifications
  
- **Advanced Patterns**
  - Hierarchical workflows
  - Reusable modules
  - Complex dependencies
  - Performance optimization

### Features
*Deep dive into all Dagu capabilities*

- **Interfaces**
  - **Command Line (CLI)**
    - Running and managing workflows
    - Scheduler control
    - Queue operations
    - Status and monitoring
  - **Web UI**
    - Dashboard and visualizations
    - DAG editor with syntax highlighting
    - Real-time execution monitoring
    - Log viewer and search
    - History and timeline views
  - **REST API**
    - Programmatic workflow control
    - Integration with external systems
    - Metrics and monitoring endpoints
    - Authentication methods

- **Executors**
  - Shell - Run any command or script
  - Docker - Container-based execution
  - SSH - Remote server execution
  - HTTP - API calls and webhooks
  - Mail - Email notifications
  - JQ - JSON data processing
  - DAG - Nested workflow execution

- **Scheduling**
  - Cron expressions with timezone support
  - Multiple schedules per DAG
  - Start/stop/restart patterns
  - Skip redundant runs
  - Schedule-based conditions

- **Execution Control**
  - Parallel execution with concurrency limits
  - Conditional execution (preconditions)
  - Continue on failure patterns
  - Retry and repeat policies
  - Output size limits
  - Signal handling

- **Data Flow**
  - Parameters and variables
  - Output capturing between steps
  - Environment variable management
  - JSON path references
  - Template rendering
  - Special variables

- **Queue System**
  - Built-in queue management
  - Per-DAG queues
  - Priority execution
  - Manual queue operations

- **Notifications**
  - Email alerts (SMTP)
  - Success/failure triggers
  - Log attachments
  - Custom templates

- **Advanced Features**
  - Hierarchical DAG composition
  - Base configuration inheritance
  - Process group management
  - Graceful shutdown handling

### Configurations
*Everything about deployment, configuration, and operations*

- **Installation & Setup**
  - Installation methods (binary, Docker, Kubernetes)
  - System requirements
  - Initial configuration
  - Production deployment checklist

- **Server Configuration**
  - Basic settings (host, port, paths)
  - Authentication (basic auth, API tokens, TLS)
  - Permissions and security
  - UI customization

- **Operations**
  - Running as a service (systemd, Docker)
  - Monitoring and metrics (Prometheus)
  - Logging and troubleshooting
  - Backup and recovery
  - Performance tuning

- **Advanced Setup**
  - Remote nodes configuration
  - Queue management
  - Integration with CI/CD

- **Configuration Reference**
  - All configuration options
  - Environment variables
  - Default values
  - Example configurations

### Reference
*Comprehensive technical documentation*

- **CLI Reference**
  - All commands and options
  - Usage examples
  - Exit codes

- **YAML Reference**
  - Complete DAG specification
  - All fields and options
  - Schema validation

- **API Reference**
  - REST API endpoints
  - Authentication methods
  - Request/response formats

- **Configuration Reference**
  - All server options
  - Environment variables
  - Default values

- **Variables Reference**
  - System variables
  - Special variables
  - Template functions

- **Executor Reference**
  - Configuration for each executor
  - Platform-specific notes
  - Performance characteristics

- **Troubleshooting**
  - Common issues
  - Debug techniques
  - FAQ

## Documentation Structure

### File Organization
```
docs/
├── overview/
│   ├── index.md              # What is Dagu and why use it
│   ├── architecture.md       # How Dagu works
│   ├── use-cases.md         # Common scenarios
│   └── comparison.md        # vs other tools
├── getting-started/
│   ├── index.md             # Quick start guide
│   ├── installation.md      # Installation methods
│   ├── first-workflow.md    # Hello world tutorial
│   └── concepts.md          # Core concepts
├── examples/
│   └── index.md             # All features with examples
├── writing-workflows/
│   ├── index.md             # Introduction
│   ├── basics.md            # Steps and dependencies
│   ├── control-flow.md      # Conditions and flow
│   ├── data-variables.md    # Variables and data
│   ├── error-handling.md    # Retry and recovery
│   └── advanced.md          # Advanced patterns
├── features/
│   ├── interfaces/
│   │   ├── cli.md          # Command line guide
│   │   ├── web-ui.md       # Web UI features
│   │   └── api.md          # REST API usage
│   ├── executors/          # All executor docs
│   ├── scheduling.md       # Cron and schedules
│   ├── execution-control.md # Parallel, conditions
│   ├── data-flow.md        # Variables and output
│   ├── queues.md           # Queue system
│   └── notifications.md     # Email alerts
├── configurations/
│   ├── index.md            # Overview
│   ├── installation.md     # Setup guide
│   ├── server.md           # Server config
│   ├── operations.md       # Running in production
│   ├── advanced.md         # HA, remote nodes
│   └── reference.md        # All options
└── reference/
    ├── cli.md              # CLI commands
    ├── yaml.md             # YAML specification
    ├── api.md              # REST API
    ├── config.md           # Configuration
    ├── variables.md        # Variables
    ├── executors.md        # Executor details
    └── troubleshooting.md  # Debug guide
```

### Content Guidelines

1. **Page Length**: Keep pages focused on single topics, 3-5 minute read
2. **Examples First**: Show practical examples before theory
3. **Progressive Disclosure**: Start simple, add complexity gradually
4. **Cross-linking**: Liberal use of links between related topics
5. **Searchability**: Use clear headings and keywords
6. **Version Notes**: Mark version-specific features clearly

### Guide vs Reference

**Features section** (including Interfaces) contains:
- How-to guides and tutorials
- Common patterns and use cases
- Best practices
- Conceptual explanations

**Reference section** contains:
- Complete command listings
- API endpoint specifications
- Configuration option tables
- Exhaustive parameter lists

### Examples Page

The `examples/index.md` page should contain:
- One minimal example per feature
- Clear section headings (Basic Execution, Parallel, Conditions, etc.)
- Each example 5-10 lines max
- Link to detailed documentation for each feature
- Copy-paste ready YAML snippets
- Brief one-line description of what each example does
- Organized by functionality, not complexity

### Navigation Structure

Primary navigation should follow the top-level structure with expandable sections for detailed topics. Each section should have:
- Overview/index page
- Logical sub-grouping
- Clear progression from simple to complex
- Quick links to common tasks

### Special Pages

1. **Search**: Full-text search across all documentation
2. **Glossary**: Terms and definitions
3. **Cheat Sheet**: Quick reference for common patterns
4. **What's New**: Highlight recent features and changes
5. **API Explorer**: Interactive API documentation

## Important Topics to Include

### Technical Details
- **Process Architecture**: Explain Unix socket communication, no DBMS design
- **History Retention**: Default 30 days, configuration options
- **Output Limits**: 1MB default per step, configuration and implications
- **Process Groups**: How Dagu manages process groups and signal propagation
- **Queue Implementation**: File-based queue system details
- **Performance Limits**: 1000 item limit for parallel execution

### Advanced Features
- **DAG Execution Types**: Chain vs Graph vs Agent modes
- **Step ID References**: Accessing stdout/stderr/exit_code via IDs
- **Regex Support**: Using `re:` prefix in conditions
- **Command Substitution**: Backtick evaluation in conditions
- **Map-based Steps**: Alternative to array format
- **JSON Path Syntax**: Deep object access in variables

### Integration Points
- **Metrics Format**: Prometheus-compatible metrics details
- **API Versions**: v1 and v2 differences, migration notes
- **Email Services**: Sendgrid, Mailgun compatibility notes
- **CI/CD Patterns**: Specific examples for popular CI tools

### User Experience
- **UI Features**: Retry from specific step capability
- **Search Actions**: DAG search across definitions
- **Remote Nodes**: Multi-environment dashboard
- **Scheduler Daemon**: Example setup scripts

### Best Practices
- **DAG Organization**: Directory structure recommendations
- **Queue Strategies**: When to use per-DAG vs global queues
- **Error Handling**: Comprehensive patterns and examples
- **Performance Tuning**: Specific recommendations for scale

## Design Specification (VitePress)

### Visual Design

#### Color System
```css
:root {
  /* Primary Colors */
  --vp-c-brand: #00D9FF;           /* Dagu cyan */
  --vp-c-brand-light: #33E4FF;     /* Hover state */
  --vp-c-brand-dark: #00A8CC;      /* Active state */
  
  /* Accent Colors */
  --vp-c-accent: #FF6B6B;          /* Errors, important */
  --vp-c-success: #4ECDC4;         /* Success states */
  --vp-c-warning: #FFD93D;         /* Warnings */
  
  /* Backgrounds */
  --vp-c-bg: #FFFFFF;              /* Main background */
  --vp-c-bg-soft: #F6F8FA;         /* Soft background */
  --vp-c-bg-mute: #E7ECF3;        /* Muted elements */
  
  /* Dark Mode */
  --vp-c-bg-dark: #0A0E27;         /* Deep blue-black */
  --vp-c-bg-dark-soft: #1E2139;    /* Code blocks */
  --vp-c-bg-dark-mute: #2D3458;    /* Borders */
}
```

#### Typography
```css
:root {
  --vp-font-family-base: 'Inter', system-ui, -apple-system, sans-serif;
  --vp-font-family-mono: 'JetBrains Mono', 'Fira Code', Consolas, monospace;
}
```

### Layout Components

#### Hero Section (Homepage)
- Gradient text headline: "Workflows that just work"
- Subtitle: "Zero dependencies. Single binary. Infinite possibilities."
- Interactive install command with copy button
- Primary CTA: "Get Started in 3 Minutes"
- Secondary CTAs: View Examples, GitHub

#### Navigation
- **Top Navigation**: Logo + 7 sections + Search (Cmd+K) + GitHub + Theme toggle
- **Sidebar**: Auto-expanding based on current section, max 3 levels deep
- **On-page TOC**: Right sidebar showing h2 and h3 headings
- **Mobile**: Bottom sheet navigation with section icons

#### Code Blocks
- Filename headers when relevant
- Copy button (appears on hover)
- Language indicators
- Line highlighting for important parts
- "Open in Playground" button for examples

### Custom Components

#### 1. Interactive Terminal
```vue
<InteractiveTerminal>
  <TerminalCommand>curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/install.sh | bash</TerminalCommand>
  <TerminalOutput>Downloading Dagu...</TerminalOutput>
  <TerminalCommand>dagu start hello.yaml</TerminalCommand>
  <TerminalOutput>Workflow started successfully</TerminalOutput>
</InteractiveTerminal>
```

#### 2. Workflow Visualizer
```vue
<WorkflowVisualizer 
  :yaml="workflowContent" 
  :interactive="true"
  :theme="currentTheme"
/>
```

#### 3. Feature Example Card
```vue
<FeatureExample>
  <template #title>Parallel Execution</template>
  <template #code>
    ```yaml
    steps:
      - name: process
        parallel: [A, B, C]
        command: echo "Processing $ITEM"
    ```
  </template>
  <template #link>/features/execution-control#parallel</template>
</FeatureExample>
```

#### 4. Comparison Table
```vue
<ComparisonTable 
  :features="['Dependencies', 'Installation', 'UI', 'Language']"
  :tools="['Dagu', 'Airflow', 'GitHub Actions', 'Cron']"
/>
```

### Page Layouts

#### Examples Page
- Grid layout (2-3 columns on desktop)
- Each example is a card with:
  - Title
  - 5-10 line code snippet
  - One-line description
  - "Learn more" link
- Filterable by category (Executors, Control Flow, Data, etc.)
- Search within examples

#### Reference Pages
- Split view on desktop: Navigation + Content
- Sticky section headers
- Collapsible parameter tables
- Jump links for long pages

### Interactive Features

#### 1. Global Search (Cmd+K)
- Fuzzy search across all content
- Show context (section, page)
- Recent searches
- Popular searches
- Keyboard navigation

#### 2. Code Playground
- Edit YAML in browser
- See visual representation
- Validate syntax
- Share via URL

#### 3. Configuration Generator
- Interactive form for server config
- Live YAML/env var output
- Copy as file or environment variables

### Performance Optimizations

1. **Static Generation**: All pages pre-rendered
2. **Code Splitting**: Lazy load heavy components
3. **Image Optimization**: WebP with fallbacks
4. **Prefetching**: Hover prefetch for links
5. **PWA**: Offline support with service worker

### Responsive Design

#### Mobile (< 768px)
- Single column layout
- Collapsible navigation
- Bottom sheet for section navigation
- Horizontal scroll for code blocks
- Tap to copy code

#### Tablet (768px - 1024px)
- Sidebar toggleable
- 2-column grid for examples
- Floating TOC

#### Desktop (> 1024px)
- Fixed sidebar
- 3-column layout (sidebar + content + TOC)
- Hover states
- Keyboard shortcuts

### Dark Mode
- System preference detection
- Smooth transitions (300ms)
- Adjusted syntax highlighting
- Dimmed images
- Higher contrast for readability

### VitePress Configuration
```js
// .vitepress/config.js
export default {
  title: 'Dagu',
  description: 'Modern workflow orchestration made simple',
  
  head: [
    ['link', { rel: 'icon', href: '/favicon.ico' }],
    ['link', { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap' }]
  ],
  
  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'Dagu',
    
    nav: [
      { text: 'Overview', link: '/overview/', activeMatch: '/overview/' },
      { text: 'Getting Started', link: '/getting-started/', activeMatch: '/getting-started/' },
      { text: 'Examples', link: '/examples/', activeMatch: '/examples/' },
      { text: 'Writing Workflows', link: '/writing-workflows/', activeMatch: '/writing-workflows/' },
      { text: 'Features', link: '/features/', activeMatch: '/features/' },
      { text: 'Configurations', link: '/configurations/', activeMatch: '/configurations/' },
      { text: 'Reference', link: '/reference/', activeMatch: '/reference/' }
    ],
    
    sidebar: {
      '/overview/': [ /* auto-generated */ ],
      '/getting-started/': [ /* auto-generated */ ],
      // ... etc
    },
    
    socialLinks: [
      { icon: 'github', link: 'https://github.com/dagu-org/dagu' }
    ],
    
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2024 Dagu Contributors'
    },
    
    search: {
      provider: 'local',
      options: {
        placeholder: 'Search docs...',
        disableDetailedView: false,
        disableQueryPersistence: false
      }
    },
    
    editLink: {
      pattern: 'https://github.com/dagu-org/dagu/edit/main/docs/:path',
      text: 'Edit this page on GitHub'
    }
  },
  
  markdown: {
    theme: {
      light: 'github-light',
      dark: 'github-dark'
    },
    lineNumbers: true,
    languages: ['yaml', 'bash', 'json', 'python', 'go', 'javascript']
  },
  
  vite: {
    plugins: [
      // Add custom plugins for workflow visualization, etc.
    ]
  }
}
```

### Accessibility
- ARIA labels on all interactive elements
- Keyboard navigation support
- Screen reader friendly
- High contrast mode support
- Focus indicators
- Skip to content links
