venv: requirements.in
	python3 -m venv --clear venv
	./venv/bin/pip install pip-tools
	./venv/bin/pip-compile requirements.in
	./venv/bin/pip install -r requirements.txt

test: venv
	./venv/bin/pytest
