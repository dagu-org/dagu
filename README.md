<div align="center">
  <img src="./assets/images/dagu-logo.webp" width="480" alt="Dagu Logo">
  <h3>A lightweight and powerful workflow engine</h3>
  <p>Self-contained. Language agnostic. Lightweight.</p>
  
  <p>
    <a href="https://docs.dagu.cloud/reference/changelog"><img src="https://img.shields.io/github/release/dagu-org/dagu.svg?style=flat-square" alt="Latest Release"></a>
    <a href="https://github.com/dagu-org/dagu/actions/workflows/ci.yaml"><img src="https://img.shields.io/github/actions/workflow/status/dagu-org/dagu/ci.yaml?style=flat-square" alt="Build Status"></a>
    <a href="https://discord.gg/gpahPUjGRk"><img src="https://img.shields.io/discord/1095289480774172772?style=flat-square&logo=discord" alt="Discord"></a>
    <a href="https://bsky.app/profile/dagu-org.bsky.social"><img src="https://img.shields.io/badge/Bluesky-0285FF?style=flat-square&logo=bluesky&logoColor=white" alt="Bluesky"></a>
  </p>
  
  <p>
    <a href="https://dagu.cloud/">Website</a> |
    <a href="https://docs.dagu.cloud/getting-started/quickstart">Quick Start</a> |
    <a href="https://discord.gg/gpahPUjGRk">Discussions</a>
  </p>
</div>

# Overview - Workflow Engine for Small Teams

Dagu */dah-goo/* is a compact, portable workflow engine implemented in Go. It provides a declarative model for orchestrating command execution across diverse environments, including shell scripts, Python commands, containerized operations, or remote commands.

```yaml
steps:
  - name: step1
    command: sleep 1 && echo "Hello, dagu!"
  - name: step2
    command: sleep 1 && echo "This is a second step"
```

By declaratively defining job processes, complex workflows can be visualized, making troubleshooting and recovery easier. Viewing logs and retrying jobs can be performed from the Web UI, eliminating the need to log into a server via SSH.

It is equipped with many features to meet the detailed requirements of enterprise environments. It operates even in environments without internet access. Being a statically compiled binary, it includes all dependencies, allowing it to run in any environment, including on-premise, cloud, and IoT devices. It is a lightweight workflow engine that meets enterprise requirements.

Note: For a list of features, please refer to the [documentation](https://docs.dagu.cloud/features/).

Workflow jobs are defined as commands. Therefore, legacy scripts that have been in operation for a long time within a company or organization can be used as-is without modification. There is no need to learn a complex new language, and you can start using it right away.

<<<<<<< HEAD
Dagu is designed for enterprise & small teams to easily manage complex workflows. It aims to be an ideal choice for teams that find large-scale, high-cost infrastructure like Airflow to be overkill and are looking for a simpler solution. It requires no database management and only needs a shared filesystem, allowing you to focus on your high-value work.
=======
Dagu is designed for small teams to easily manage complex workflows. It aims to be an ideal choice for teams that find large-scale, high-cost infrastructure like Airflow to be overkill and are looking for a simpler solution. It requires no database management and only needs a shared filesystem, allowing you to focus on your high-value work.
>>>>>>> a7ede1a2 (docs: move documentation section after quickstart)

## Use Cases

Dagu is designed to orchestrate workflows across various domains, particularly those involving multi-step batch jobs and complex data dependencies. Example applications include:

* AI/ML & Data Science - Automating machine learning workflows, including data ingestion, feature engineering, model training, validation, and deployment.
* Geospatial & Environmental Analysis - Processing datasets from sources such as satellites (earth observation), aerial/terrestrial sensors, seismic surveys, and ground-based radar. Common uses include numerical weather prediction and natural resource management.
* Finance & Trading - Implementing time-sensitive workflows for stock market analysis, quantitative modeling, risk assessment, and report generation.
* Medical Imaging & Bioinformatics - Creating pipelines to process and analyze large volumes of medical scans (e.g., MRI, CT) or genomic data for clinical and research purposes.
* Data Engineering (ETL/ELT) - Building, scheduling, and monitoring pipelines for moving and transforming data between systems like databases, data warehouses, and data lakes.
* IoT & Edge Computing - Orchestrating workflows that collect, process, and analyze data from distributed IoT devices, sensors, and edge nodes.
* Media & Content Processing - Automating workflows for video transcoding, image processing, and content delivery, including tasks like format conversion, compression, and metadata extraction.

## Quick Demos

**CLI Demo**: Create and run a simple DAG workflow from the command line.

![Demo CLI](./assets/images/demo-cli.webp)

**Web UI Demo**: Create and manage workflows using the web interface, with real-time monitoring and control.

[Docs on CLI](https://docs.dagu.cloud/overview/cli)

![Demo Web UI](./assets/images/demo-web-ui.webp)

[Docs on Web UI](https://docs.dagu.cloud/overview/web-ui)

## Quick Start

### 1. Install dagu

**npm**:
```bash
# Install via npm
npm install -g dagu
```

**Homebrew**:

```bash
brew install dagu-org/brew/dagu

# Upgrade to latest version
brew upgrade dagu-org/brew/dagu
```

**macOS/Linux**:

```bash
# Install via script
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash
```

**Docker**:

```bash
docker run --rm \
  -v ~/.dagu:/var/lib/dagu \
  -p 8080:8080 \
  ghcr.io/dagu-org/dagu:latest \
  dagu start-all
```

Note: see [documentation](https://docs.dagu.cloud/getting-started/installation) for other methods.

### 2. Create your first workflow

```bash
cat > ./hello.yaml << 'EOF'
steps:
  - name: hello
    command: echo "Hello from Dagu!"
  - name: world  
    command: echo "Running step 2"
EOF
```

### 3. Run the workflow

```bash
dagu start hello.yaml
```

### 4. Check the status and view logs

```bash
dagu status hello
```

### 5. Explore the Web UI

```bash
dagu start-all
```

Visit http://localhost:8080

## Documentation

Full documentation is available at [docs.dagu.cloud](https://docs.dagu.cloud/).

**Helpful Links**:

- [Feature by Examples](https://docs.dagu.cloud/writing-workflows/examples) - Explore useful features with examples
- [Distributed Execution](https://docs.dagu.cloud/features/distributed-execution) - How to run workflows across multiple machines
- [Scheduling](https://docs.dagu.cloud/features/scheduling) - Learn about flexible scheduling options (start, stop, restart) with cron syntax
- [Authentication](https://docs.dagu.cloud/configurations/authentication) - Configure authentication for the Web UI
- [Configuration](https://docs.dagu.cloud/configurations/reference) - Detailed configuration options for customizing Dagu
- [Changelog](https://docs.dagu.cloud/reference/changelog) - Keep up with the latest updates and changes

## Development

### Building from Source

#### Prerequisites
- [Go 1.24+](https://go.dev/doc/install)
- [Node.js](https://nodejs.org/en/download/)
- [pnpm](https://pnpm.io/installation)

#### 1. Clone the repository and build server

```bash
git clone https://github.com/dagu-org/dagu.git && cd dagu
make
```

This will start the dagu server at http://localhost:8080.

#### 2. Run the frontend development server

```bash
cd ui
pnpm install
pnpm dev
```

Navigate to http://localhost:8081 to view the frontend.

## Discussion

For discussions, support, and sharing ideas, join our community on [Discord](https://discord.gg/gpahPUjGRk).

## Recent Updates

Changelog of recent updates can be found in the [Changelog](https://docs.dagu.cloud/reference/changelog) section of the documentation.

## Contributing

We welcome contributions of all kinds! If you have ideas, suggestions, or improvements, please open an issue or submit a pull request.

For detailed contribution guidelines, please refer to our [CONTRIBUTING.md](./CONTRIBUTING.md) file.

## Acknowledgements

### Contributors

<a href="https://github.com/dagu-org/dagu/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=dagu-org/dagu" />
</a>

Thanks to all the contributors who have helped make Dagu better! Your contributions, whether through code, documentation, or feedback, are invaluable to the project.

### Sponsors & Supporters

<a href="https://github.com/Arvintian"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2FArvintian.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@Arvintian"></a>
<a href="https://github.com/yurivish"><img src="https://wsrv.nl/?url=https%3A%2F%2Fgithub.com%2Fyurivish.png&w=128&h=128&fit=cover&mask=circle" width="64" height="64" alt="@yurivish"></a>

Thanks for supporting Dagu’s development! Join our supporters: [GitHub Sponsors](https://github.com/sponsors/dagu-org)

## License

GNU GPLv3 - See [LICENSE](./LICENSE)

---

<div align="center">
  <p>If you find Dagu useful, please ⭐ star this repository</p>
</div>
