import { defineConfig } from "vitepress";

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Documentation | Dagu",
  description: "Dagu explanation and usage information",
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: "Home", link: "/" },
      { text: "Getting Started", link: "/getting-started/" },
      { text: "Writing Workflows", link: "/writing-workflows/" },
      { text: "Features", link: "/features/" },
      { text: "API Reference", link: "/reference/api" },
    ],

    sidebar: [
      {
        text: "Getting Started",
        items: [
          { text: "Introduction", link: "/getting-started/" },
          { text: "Installation", link: "/getting-started/installation" },
          { text: "Quick Start", link: "/getting-started/quickstart" },
        ],
      },
      {
        text: "Overview",
        items: [
          { text: "Architecture", link: "/overview/architecture" },
          { text: "Command Line Interface", link: "/overview/cli" },
          { text: "Web UI", link: "/overview/web-ui" },
          { text: "REST API", link: "/overview/api" },
        ],
      },
      {
        text: "Writing Workflows",
        items: [
          { text: "Introduction", link: "/writing-workflows/" },
          { text: "Basic DAG", link: "/writing-workflows/basic-dag" },
          { text: "Container", link: "/writing-workflows/container" },
          { text: "Variables", link: "/writing-workflows/variables" },
          { text: "Step Execution", link: "/writing-workflows/step-execution" },
          { text: "Conditional Logic", link: "/writing-workflows/conditional-logic" },
          { text: "Lifecycle Hooks", link: "/writing-workflows/lifecycle-hooks" },
          { text: "Local DAGs", link: "/writing-workflows/local-dags" },
          { text: "Examples", link: "/writing-workflows/examples" },
        ],
      },
      {
        text: "Features",
        items: [
          { text: "Overview", link: "/features/" },
          { text: "Scheduling", link: "/features/scheduling" },
          { text: "Execution Control", link: "/features/execution-control" },
          { text: "Data Flow", link: "/features/data-flow" },
          { text: "Queues", link: "/features/queues" },
          { text: "Notifications", link: "/features/notifications" },
          { text: "OpenTelemetry", link: "/features/opentelemetry" },
          { text: "Distributed Execution", link: "/features/distributed-execution" },
          { text: "Worker Labels", link: "/features/worker-labels" },
          {
            text: "Executors",
            items: [
              { text: "Shell", link: "/features/executors/shell" },
              { text: "Docker", link: "/features/executors/docker" },
              { text: "SSH", link: "/features/executors/ssh" },
              { text: "HTTP", link: "/features/executors/http" },
              { text: "Mail", link: "/features/executors/mail" },
              { text: "JQ", link: "/features/executors/jq" },
            ],
          },
        ],
      },
      {
        text: "Configuration",
        items: [
          { text: "Server", link: "/configurations/server" },
          { text: "Scheduler", link: "/configurations/scheduler" },
          { text: "Operations", link: "/configurations/operations" },
          { text: "Reference", link: "/configurations/reference" },
        ],
      },
      {
        text: "Reference",
        items: [
          { text: "CLI Commands", link: "/reference/cli" },
          { text: "REST API", link: "/reference/api" },
          { text: "YAML Format", link: "/reference/yaml" },
          { text: "Environment Variables", link: "/reference/environment-variables" },
        ],
      },
    ],

    socialLinks: [{ icon: "github", link: "https://github.com/dagu-org/dagu" }],
  },
});
