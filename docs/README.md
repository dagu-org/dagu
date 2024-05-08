# Dagu Documentation
This is the documentation for the Dagu project.

## Prerequisites
Make sure you have the following installed:

- Python (version >= 3.11)
- Rye

## Getting Started

1. Clone the repository:
```sh
git clone https://github.com/dagu-dev/dagu.git
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

- For English documentation:
```sh
rye run build
```

- For Japanese documentation:
```sh
rye run build-ja
```

The built documentation will be available in the build/html directory.

## Generating Gettext Files (Japanese)
To generate the gettext files for Japanese translation, run:

```shell
rye run gettext-ja
```

This command will generate the necessary gettext files in the `build/html` directory and update the Japanese translation files.

## Adding New Translations

We welcome contributions for adding new translations to the documentation. To add a new translation:

1. Create a new directory for the target language in the `source/locale` directory (e.g., `source/locale/fr` for French).
2. Generate the gettext files for the target language by running: 
  ```shell
  make gettext
  sphinx-intl update -p build/gettext -l <language>
  ```
  Replace `<language>` with the appropriate language code (e.g., `fr` for French).
3. Update the translation files in the source/locale/<language>/LC_MESSAGES directory.
4. Build the documentation for the target language:
```shell
sphinx-build -b html -D language=<language> source/ build/html/<language>
```
Replace `<language>` with the appropriate language code (e.g., `fr` for French).
5. Submit a pull request with your changes.

See [Sphinx Internationalization](https://www.sphinx-doc.org/en/master/usage/advanced/intl.html) for more information on internationalization in Sphinx.

## Dependencies
The project dependencies are managed using Rye and specified in the `pyproject.toml` file. The main dependencies include:

- Sphinx (version 5.3.0)
- sphinx-rtd-theme (version 2.0.0)
- sphinx-autobuild (version >= 2024.4.16)

## Contributing
We welcome contributions to improve the documentation. If you find any issues or have suggestions for enhancements, please feel free to open an issue or submit a pull request.
