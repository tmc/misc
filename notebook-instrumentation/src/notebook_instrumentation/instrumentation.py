import os
import json
import segment.analytics as analytics

analytics.write_key = os.environ.get("S_WRITE_KEY", "")
analytics.debug = os.environ.get("INSTRUMENTATION_DEBUG", "").lower() in ('true', '1', 't')
analytics.send = not os.environ.get("INSTRUMENTATION_SEND_DISABLED", "").lower() in ('true', '1', 't')

def on_error(error, items):
    print("An error occurred:", error)
analytics.on_error = on_error

instrumentor = analytics

def getid():
    '''Returns a machine-specific anonymous identifier, written and read from ~/.notebook-id'''
    try:
        with open(os.path.expanduser("~/.notebook-id"), "r") as f:
            return f.read().strip()
    except:
        import uuid
        id = str(uuid.uuid1())
        with open(os.path.expanduser("~/.notebook-id"), "w") as f:
            f.write(id)
        return id


def get_notebook_path():
    try:
        import ipykernel
        conninfo = json.loads(ipykernel.get_connection_info())
        return conninfo['jupyter_session']
    except Exception as e:
        return 'unknown'

def get_notebook_name():
    return os.path.split(get_notebook_path())[-1]


def strip_output(nb):
    for ws in nb.worksheets:
        for cell in ws.cells:
            if hasattr(cell, "outputs"):
                cell.outputs = []
            if hasattr(cell, "prompt_number"):
                del cell["prompt_number"]

def strip_notebook_outputs(path):
    try:
        from IPython.nbformat.current import read, write
        nb = open(path, "r").read()
        return strip_output(nb)
    except Exception as e:
        return None

def get_notebook_hash():
    try:
        import hashlib
        return hashlib.md5(open(get_notebook_path()).read().encode()).hexdigest()
    except Exception as e:
        return 'unknown'



def collect_system_context():
    '''Collects system context information.'''
    import platform
    import psutil
    import os

    properties = {
        "app": {  # name, version, build.
            # get notebook information
            "name": f"notebook {get_notebook_name()}",
        },
        "library": {
            "name": "instrumentation",
            "version": "0.0.1",
            "build": "1",
        },
        "platform": platform.platform(),
        "cpu_count": os.cpu_count(),
        "memory": psutil.virtual_memory(),
        "disk": psutil.disk_usage("/"),
    }


# TODO:
# - scoop up account information to .alias
# - scoop up machine information
