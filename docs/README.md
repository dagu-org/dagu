# Dagu Documentation

This is the documentation for Dagu.

## Prerequisites

- Python (version >= 3.11)
- Rye

## Getting Started

1. Clone the repository:

```sh
git clone https://github.com/dagu-org/dagu.git
```

2. Navigate to the document directory:

```sh
cd docs
```

3. Install the project dependencies using Rye:

```sh
rye sync
```

## Running Locally

To run the documentation server locally, use the following command:

```sh
rye run serve
```

This will start the sphinx-autobuild server, which will watch for changes in the docs directory and automatically rebuild the documentation.
Open your web browser and visit http://localhost:8000 to view the documentation.

## Building the Documentation

To build the documentation, use one of the following commands:

```sh
rye run build
```

See [Sphinx Internationalization](https://www.sphinx-doc.org/en/master/usage/advanced/intl.html) for more information on internationalization in Sphinx.

## Dependencies

The project dependencies are managed using Rye and specified in the `pyproject.toml` file. The main dependencies include:

- Sphinx (version 5.3.0)
- sphinx-rtd-theme (version 2.0.0)
- sphinx-autobuild (version >= 2024.4.16)

## Contributing

We welcome contributions to improve the documentation. Please feel free to open an issue or submit a pull request.
