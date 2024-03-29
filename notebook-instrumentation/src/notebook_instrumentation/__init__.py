import os
from .instrumentation import (
    getid, instrumentor,
    collect_system_context,
    get_notebook_name,
    get_notebook_hash,
)

if os.environ.get("INSTRUMENTATION_DEBUG", None):
    print("🤙 instrumentation is working! 🤙")

def load_ipython_extension(ipython):
    instrumentor.track(None, "shell_initialized", {
        "notebook_name": get_notebook_name(),
    }, context=collect_system_context(), anonymous_id=getid())
    for e in [
        "shell_initialized",
        "pre_run_cell",
        "post_run_cell",
        "pre_execute",
        "post_execute",
    ]:
        # curried function:
        ipython.events.register(e, get_event_handler(e))

def extract_info_from_event(event):
    '''Extracts information from an event object, respecting ipython event objects.'''
    info = {}
    if hasattr(event, "__dict__"):
        for k, v in event.__dict__.items():
            info[k] = str(v)
    if hasattr(event, "items"):
        for k, v in event.items():
            info[k] = str(v)
    if hasattr(event, "info"):
        iinfo = {}
        for k, v in event.info.__dict__.items():
            info[k] = str(v)
        del info["info"]
    return info

def generic_event_handler(event_name, *args):
    event = {}
    properties = {
        "raw_event": repr(event),
        "notebook_name": get_notebook_name(),
        "notebook_hash": get_notebook_hash(),
    }
    if len(args) > 0:
        for k, v in extract_info_from_event(args[0]).items():
            properties[k] = str(v)
    instrumentor.track(None, event_name, properties, context=collect_system_context(), anonymous_id=getid())

def get_event_handler(event_name):
    def event_handler(*args):
        generic_event_handler(event_name, *args)
    return event_handler

