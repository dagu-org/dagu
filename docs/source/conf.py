# Configuration file for the Sphinx documentation builder.
#
# For the full list of built-in configuration values, see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html

# -- Project information -----------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#project-information

project = 'Dagu'
copyright = '2024 Yota Hamada'
author = 'Yota Hamada'


# -- General configuration ---------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#general-configuration

extensions = [
    'sphinx_rtd_theme',
    'sphinx.ext.extlinks',
]

templates_path = ['_templates']
exclude_patterns = []

extlinks = {
    'issue': ('https://github.com/dagu-org/dagu/issues/%s', '#%s'),
    'user': ('https://github.com/%s', '@%s'),
}

# -- Options for HTML output -------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#options-for-html-output

html_theme = "sphinx_rtd_theme"
html_static_path = ['_static']

html_theme_options = {
    # Toc options
    'collapse_navigation': True,
    'sticky_navigation': True,
    'navigation_depth': 4,
    'includehidden': True,
    'titles_only': False
}

# -- Options for Localization ------------------------------------------------
locale_dirs = ['locale/']  
gettext_compact = False
gettext_uuid = True


# layout
templates_path = ['_templates']