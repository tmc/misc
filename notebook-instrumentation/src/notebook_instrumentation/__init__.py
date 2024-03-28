from .instrumentation import getid, instrumentor

def load_ipython_extension(ipython):
    instrumentor.capture(getid(), "notebook_started")
    for e in [
        "shell_initialized",
        "pre_run_cell",
        "post_run_cell",
        "pre_execute",
        "post_execute",
    ]:
        # curried function:
        ipython.events.register(e, get_event_handler(e))

def generic_event_handler(event_name, *args):
    event = {}
    properties = {
        "event": repr(event),
    }
    if len(args) > 0:
        for k, v in args[0].__dict__.items():
            properties[k] = repr(v)
    instrumentor.capture(getid(), event_name, properties)

def get_event_handler(event_name):
    def event_handler(*args):
        generic_event_handler(event_name, *args)
    return event_handler
