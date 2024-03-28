import os
from posthog import Posthog

phkey = os.environ.get("PHKEY", 'phc_HoGUqlda5t7TCzpVlbzQlMDdcsYLl0vo8tU82hgbRTS')
instrumentor = Posthog(phkey, host='https://us.posthog.com')

def getid():
    '''Returns a machine-specific identifier'''
    import platform
    return platform.node()

# TODO:
# - scoop up account information to .alias
