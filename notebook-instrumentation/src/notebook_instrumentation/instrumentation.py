import os
import segment.analytics as analytics

analytics.write_key = os.environ.get("S_WRITE_KEY", "")
analytics.debug = os.environ.get("INSTRUMENTATION_DEBUG", "").lower() in ('true', '1', 't')
analytics.send = os.environ.get("INSTRUMENTATION_SEND_DISABLED", "").lower() in ('true', '1', 't')

def on_error(error, items):
    print("An error occurred:", error)
analytics.on_error = on_error

instrumentor = analytics

def getid():
    '''Returns a machine-specific identifier'''
    import platform
    return platform.node()

# TODO:
# - scoop up account information to .alias
# - scoop up machine information
