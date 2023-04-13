local:
	live-server docs/build/html

venv:
	python -m venv .venv
	python -m pip install sphinx

install-dependencies:
	python -m pip install -r requirements.txt
	npm install -g live-server

build:
	sphinx-build -b html docs/source/ docs/build/html