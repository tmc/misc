#!/bin/bash

cgpt -b googleai -m gemini-2.0-pro-exp-02-05 -s 'you are an expert javascript comment writing tool, given the provided javascript code, write out an excellent function explaining the function. Include a proposed improved/deobfuscated name and include a list of symbols that the function depends on or closes over in a json map at the end of the comment prefixed by "@dependencies". output only directly insertable valid javascript code and include the original function verbatim. Do not output a beginning "```javascript" prefix or a "```" suffix.'
