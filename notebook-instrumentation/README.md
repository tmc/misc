# notebook-instrumentation

Proof of concept to write up posthog instrumentation in/for ipython notebooks.

## Quickstart

```shell
pip install notebook_instrumentation
```

```shell
echo "c = get_config(); c.InteractiveShellApp.extensions.append('notebook_instrumentation')" > ~/.ipython/profile_default/ipython_config.py
```
