from IPython.core.magic import register_cell_magic

def instrument_cell(cell_content):
    # Your instrumentation code here
    print("Executing cell:")
    print(cell_content)
    # Generate instrumentation data or perform any desired actions

@register_cell_magic
def instrument(line, cell):
    instrument_cell(cell)
    return cell

def load_ipython_extension(ipython):
    ipython.register_magic_function(instrument, 'cell')
