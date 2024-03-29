venv: pyproject.toml
	python3 -m venv venv
	venv/bin/pip install -e .

.PHONY: test
test: venv/bin/pytest
	venv/bin/pytest

venv/bin/pytest: venv
	venv/bin/pip install pytest ipython
