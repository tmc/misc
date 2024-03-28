# notebook-instrumentation

Proof of concept to write up posthog instrumentation in/for ipython notebooks.

## POC quickstart

```shell
pip install -e $(pwd)
```

```shell
echo "c = get_config(); c.InteractiveShellApp.extensions.append('notebook_instrumentation')" > ~/.ipython/profile_default/ipython_config.py
```
