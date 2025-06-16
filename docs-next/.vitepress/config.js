import { defineConfig } from "vitepress";
import { withMermaid } from "vitepress-plugin-mermaid";

// Define the complete sidebar structure
const fullSidebar = [
  {
    text: "Overview",
    items: [
      { text: "What is Dagu?", link: "/overview/" },
      { text: "Architecture", link: "/overview/architecture" },
      { text: "Use Cases", link: "/overview/use-cases" },
      { text: "Comparison", link: "/overview/comparison" },
    ],
  },
  {
    text: "Getting Started",
    items: [
      { text: "Quick Start", link: "/getting-started/" },
      { text: "Installation", link: "/getting-started/installation" },
      { text: "First Workflow", link: "/getting-started/first-workflow" },
      { text: "Core Concepts", link: "/getting-started/concepts" },
    ],
  },
  {
    text: "Writing Workflows",
    items: [
      { text: "Introduction", link: "/writing-workflows/" },
      { text: "Basics", link: "/writing-workflows/basics" },
      { text: "By Examples", link: "/writing-workflows/examples/" },
      { text: "Control Flow", link: "/writing-workflows/control-flow" },
      { text: "Data & Variables", link: "/writing-workflows/data-variables" },
      { text: "Error Handling", link: "/writing-workflows/error-handling" },
      { text: "Advanced Patterns", link: "/writing-workflows/advanced" },
    ],
  },
  {
    text: "Features",
    items: [
      { text: "Overview", link: "/features/" },
      {
        text: "Interfaces",
        collapsed: false,
        items: [
          { text: "Command Line", link: "/features/interfaces/cli" },
          { text: "Web UI", link: "/features/interfaces/web-ui" },
          { text: "REST API", link: "/features/interfaces/api" },
        ],
      },
      {
        text: "Executors",
        collapsed: false,
        items: [
          { text: "Shell", link: "/features/executors/shell" },
          { text: "Docker", link: "/features/executors/docker" },
          { text: "SSH", link: "/features/executors/ssh" },
          { text: "HTTP", link: "/features/executors/http" },
          { text: "Mail", link: "/features/executors/mail" },
          { text: "JQ", link: "/features/executors/jq" },
          { text: "DAG", link: "/features/executors/dag" },
        ],
      },
      { text: "Scheduling", link: "/features/scheduling" },
      { text: "Execution Control", link: "/features/execution-control" },
      { text: "Data Flow", link: "/features/data-flow" },
      { text: "Queue System", link: "/features/queues" },
      { text: "Notifications", link: "/features/notifications" },
    ],
  },
  {
    text: "Configurations",
    items: [
      { text: "Overview", link: "/configurations/" },
      { text: "Installation & Setup", link: "/configurations/installation" },
      { text: "Server Configuration", link: "/configurations/server" },
      { text: "Operations", link: "/configurations/operations" },
      { text: "Advanced Setup", link: "/configurations/advanced" },
      { text: "Configuration Reference", link: "/configurations/reference" },
    ],
  },
  {
    text: "Reference",
    items: [
      { text: "CLI Commands", link: "/reference/cli" },
      { text: "YAML Specification", link: "/reference/yaml" },
      { text: "REST API", link: "/reference/api" },
      { text: "Configuration", link: "/reference/config" },
      { text: "Variables", link: "/reference/variables" },
      { text: "Executors", link: "/reference/executors" },
      { text: "Troubleshooting", link: "/reference/troubleshooting" },
      { text: "Changelog", link: "/reference/changelog" },
    ],
  },
];

export default withMermaid(
  defineConfig({
    title: "Dagu",
    description: "Modern workflow orchestration made simple",
    head: [
      ["link", { rel: "icon", href: "/favicon.ico" }],
      ["link", { rel: "preconnect", href: "https://fonts.googleapis.com" }],
      [
        "link",
        {
          rel: "preconnect",
          href: "https://fonts.gstatic.com",
          crossorigin: "",
        },
      ],
      [
        "link",
        {
          rel: "stylesheet",
          href: "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap",
        },
      ],
      [
        "script",
        {},
        `
          // Set dark mode as default
          ;(function() {
            const userMode = localStorage.getItem('vitepress-theme-appearance')
            if (!userMode || userMode === 'auto') {
              localStorage.setItem('vitepress-theme-appearance', 'dark')
              document.documentElement.classList.add('dark')
            }
          })()
        `
      ]
    ],

    themeConfig: {
      logo: "/logo.svg",
      siteTitle: "Dagu",
      
      appearance: {
        defaultTheme: 'dark'
      },

      outline: {
        level: [2, 3],
        label: "On this page",
      },

      nav: [
        { text: "Overview", link: "/overview/", activeMatch: "/overview/" },
        {
          text: "Getting Started",
          link: "/getting-started/",
          activeMatch: "/getting-started/",
        },
        {
          text: "Writing Workflows",
          link: "/writing-workflows/",
          activeMatch: "/writing-workflows/",
        },
        { text: "Features", link: "/features/", activeMatch: "/features/" },
        {
          text: "Configurations",
          link: "/configurations/",
          activeMatch: "/configurations/",
        },
        { text: "Reference", link: "/reference/", activeMatch: "/reference/" },
      ],

      sidebar: {
        "/": fullSidebar,
        "/overview/": fullSidebar,
        "/getting-started/": fullSidebar,
        "/writing-workflows/": fullSidebar,
        "/features/": fullSidebar,
        "/configurations/": fullSidebar,
        "/reference/": fullSidebar,
      },

      socialLinks: [
        { icon: "github", link: "https://github.com/dagu-org/dagu" },
      ],

      footer: {
        message: "Released under the MIT License.",
        copyright: "Copyright © 2024 Dagu Contributors",
      },

      search: {
        provider: "local",
        options: {
          placeholder: "Search docs...",
          disableDetailedView: false,
          disableQueryPersistence: false,
        },
      },

      editLink: {
        pattern: "https://github.com/dagu-org/dagu/edit/main/docs-next/:path",
        text: "Edit this page on GitHub",
      },
    },

    markdown: {
      theme: {
        light: "github-light",
        dark: "github-dark",
      },
      lineNumbers: true,
      config: () => {
        // Add any markdown-it plugins here
      },
    },

    // Mermaid plugin configuration
    mermaid: {
      theme: "default",
      darkMode: true,
      themeVariables: {
        primaryColor: "#25b3c0",
        primaryTextColor: "#eee",
        primaryBorderColor: "#0085a3",
        lineColor: "#666",
        secondaryColor: "#f3f3f3",
        tertiaryColor: "#eee",
      },
    },
    mermaidPlugin: {
      class: "mermaid my-class", // set additional css classes for parent container
    },
  })
);
