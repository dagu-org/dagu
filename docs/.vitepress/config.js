import { defineConfig } from "vitepress";
import { withMermaid } from "vitepress-plugin-mermaid";
import { issueLinksPlugin } from "./theme/plugins/issueLinks.js";

// Define the complete sidebar structure
const fullSidebar = [
  {
    text: "Overview",
    items: [
      { text: "What is Dagu?", link: "/overview/" },
      { text: "Architecture", link: "/overview/architecture" },
      { text: "CLI", link: "/overview/cli" },
      { text: "Web UI", link: "/overview/web-ui" },
      { text: "API", link: "/overview/api" },
    ],
  },
  {
    text: "Getting Started",
    items: [
      { text: "Quickstart", link: "/getting-started/quickstart" },
      { text: "Installation", link: "/getting-started/installation" },
      { text: "Core Concepts", link: "/getting-started/concepts" },
    ],
  },
  {
    text: "Writing Workflows",
    items: [
      { text: "Introduction", link: "/writing-workflows/" },
      { text: "Basics", link: "/writing-workflows/basics" },
      { text: "Examples", link: "/writing-workflows/examples/" },
      { text: "Parameters", link: "/writing-workflows/parameters" },
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
        text: "Executors",
        collapsed: false,
        items: [
          { text: "Shell", link: "/features/executors/shell" },
          { text: "Docker", link: "/features/executors/docker" },
          { text: "SSH", link: "/features/executors/ssh" },
          { text: "HTTP", link: "/features/executors/http" },
          { text: "Mail", link: "/features/executors/mail" },
          { text: "JQ", link: "/features/executors/jq" },
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
      { text: "Server Configuration", link: "/configurations/server" },
      { text: "Deployment", link: "/configurations/deployment" },
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
      { text: "Changelog", link: "/reference/changelog" },
    ],
  },
];

export default withMermaid(
  defineConfig({
    title: "Dagu",
    description: "Modern workflow orchestration made simple",
    lang: "en-US",
    lastUpdated: true,
    cleanUrls: true,

    head: [
      ["link", { rel: "icon", type: "image/x-icon", href: "/favicon.ico" }],
      ["link", { rel: "shortcut icon", href: "/favicon.ico" }],
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
        `,
      ],
    ],

    themeConfig: {
      logo: "/logo.svg",
      siteTitle: "Dagu",
      logoLink: "https://dagu.cloud/",

      appearance: {
        defaultTheme: "dark",
      },

      outline: {
        level: [2, 3],
        label: "On this page",
      },

      nav: [
        { text: "Home", link: "/" },
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

      lastUpdated: {
        text: "Last updated",
        formatOptions: {
          dateStyle: "medium",
          timeStyle: "short",
        },
      },
    },

    markdown: {
      theme: {
        light: "github-light",
        dark: "github-dark",
      },
      lineNumbers: true,
      config: (md) => {
        // Add issue links plugin
        md.use(issueLinksPlugin);

        // Add custom link renderer to open external links in new tab
        const defaultLinkRender =
          md.renderer.rules.link_open ||
          function (tokens, idx, options, _env, self) {
            return self.renderToken(tokens, idx, options);
          };

        md.renderer.rules.link_open = function (
          tokens,
          idx,
          options,
          _env,
          self
        ) {
          const token = tokens[idx];
          const hrefIndex = token.attrIndex("href");
          if (hrefIndex >= 0) {
            const href = token.attrs[hrefIndex][1];
            if (
              href &&
              (href.startsWith("http://") || href.startsWith("https://"))
            ) {
              token.attrSet("target", "_blank");
              token.attrSet("rel", "noopener noreferrer");
            }
          }
          return defaultLinkRender(tokens, idx, options, _env, self);
        };
      },
    },

    // Mermaid plugin configuration
    mermaid: {
      theme: "base",
      darkMode: false,
      themeVariables: {
        primaryColor: "#25b3c0",
        primaryTextColor: "#333",
        primaryBorderColor: "#0085a3",
        lineColor: "#666",
        secondaryColor: "#f3f3f3",
        tertiaryColor: "#eee",
        background: "#ffffff",
        mainBkg: "#ffffff",
        secondaryBkg: "#f8f9fa",
        tertiaryBkg: "#ffffff",
        secondaryBorderColor: "#ccc",
        tertiaryBorderColor: "#ccc",
        secondaryTextColor: "#333",
        tertiaryTextColor: "#333",
        textColor: "#333",
        taskBkgColor: "#ffffff",
        taskTextColor: "#333",
        taskTextLightColor: "#333",
        taskTextOutsideColor: "#333",
        taskTextClickableColor: "#333",
        activeTaskBkgColor: "#f0f0f0",
        activeTaskBorderColor: "#0085a3",
        gridColor: "#e1e5e9",
        section0: "#ffffff",
        section1: "#f8f9fa",
        section2: "#ffffff",
        section3: "#f8f9fa",
        altBackground: "#f8f9fa",
        altBackgroundSecondary: "#ffffff",
        fillType0: "#ffffff",
        fillType1: "#f8f9fa",
        fillType2: "#ffffff",
        fillType3: "#f8f9fa",
        fillType4: "#ffffff",
        fillType5: "#f8f9fa",
        fillType6: "#ffffff",
        fillType7: "#f8f9fa",
      },
    },
    mermaidPlugin: {
      class: "mermaid my-class", // set additional css classes for parent container
    },
  })
);
