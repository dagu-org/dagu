# Contributing to Dagu

## 🙌 Welcome

Dagu is an open-source workflow engine designed to simplify workflow orchestration. We welcome contributions from everyone, whether you're a developer, designer, or just passionate about improving the project.

## 🧩 Ways to Contribute

- **Code**: Fix bugs, add features, or improve documentation.
- **Documentation**: Help us improve our docs, tutorials, and examples.
- **Testing**: Write tests, report bugs, or help with QA.
- **Design**: Contribute UI/UX improvements or design assets.
- **Community**: Answer questions, help others, or spread the word about Dagu.

## 🛠️ Setup Instructions

### Prerequisites
- [Go 1.25+](https://go.dev/doc/install)
- [Node.js](https://nodejs.org/en/download/)
- [pnpm](https://pnpm.io/installation)

To get started with contributing, follow these steps:

#### For backend development:

1. **Clone the Repository**:
   ```bash
   git clone https://github.com/dagu-org/dagu.git
   cd dagu
   ```

2. **Build the frontend**:
   ```bash
   make ui
   ```

3. **Start the server**:
   ```bash
   make run
   ```

4. **Run tests**:
   ```bash
   make test
   ```

That's it! The backend server should now be running on `http://localhost:8080`, and you can access the frontend at the same address.

#### For frontend development:

Note: make sure the backend server is running at `http://localhost:8080`.

1. **Navigate to the frontend directory**:
   ```bash
   cd ui
   ```
2. **Install dependencies**:
   ```bash
   pnpm install
   ```
3. **Start the development server**:
   ```bash
   pnpm dev
   ```

That's it! The frontend should now be running on `http://localhost:8081`, and it will automatically reload when you make changes.

## 🎯 Code Style Guidelines

- Keep code simple and readable.
- Write unit tests for new features.

## ✅ Running Tests

Use `make test` to run all tests.

You can run linter checks with:
```bash
make golangci-lint
```

## 📦 Submitting Changes (Pull Requests)

1. Fork the repository.
2. Create a new branch for your feature or bug fix:
   ```bash
   git checkout -b feature/my-feature
   ```
3. Make your changes and commit them:
   ```bash
   git commit -m "Add my feature"
   ```
4. Push your changes to your fork:
   ```bash
   git push origin feature/my-feature
   ```
5. Open a pull request against the `main` branch of the original repository.
6. Provide a clear description of your changes and why they are needed.

## 🐞 Reporting Bugs / Requesting Features

If you find a bug or have a feature request, please open an issue in the [Issues](https://github.com/dagu-org/dagu/issues) section of the repository.

## 🤝 Code of Conduct

This project is covered under the [Go Community Code of Conduct](https://golang.org/conduct).

## 🔗 Helpful Links

- [Dagu's Architecture](https://docs.dagu.cloud/overview/architecture)
- [YAML spec for Dagu](https://docs.dagu.cloud/reference/yaml)
- [All configurations](https://docs.dagu.cloud/configurations/reference#configuration-file)
