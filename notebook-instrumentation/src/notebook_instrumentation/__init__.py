import os
from .instrumentation import getid, instrumentor

if os.environ.get("INSTRUMENTATION_DEBUG", None):
    print("🤙 instrumentation is working! 🤙")

def load_ipython_extension(ipython):
    instrumentor.track(getid(), "notebook_started")
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
            info[k] = repr(v)
    if hasattr(event, "items"):
        for k, v in event.items():
            info[k] = repr(v)
    if hasattr(event, "info"):
        iinfo = {}
        for k, v in event.info.__dict__.items():
            info[k] = repr(v)
        del info["info"]
    return info

def generic_event_handler(event_name, *args):
    event = {}
    properties = {
        "raw_event": repr(event),
    }
    if len(args) > 0:
        for k, v in args[0].__dict__.items():
            properties[k] = repr(v)
        for k, v in args[0].getattr('info', {}).items():
            properties[k] = repr(v)
    instrumentor.track(getid(), event_name, properties)

def get_event_handler(event_name):
    def event_handler(*args):
        generic_event_handler(event_name, *args)
    return event_handler
