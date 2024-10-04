# List all HTML files in the current directory
HTML_FILES := $(wildcard *.html)

# Generate the list of target MD files
MD_FILES := $(HTML_FILES:.html=.md)

# Default target
all: $(MD_FILES)

# Rule to convert HTML to MD
%.md: %.html
	cat $< | html2md > $@

# Clean target to remove all generated MD files
clean:
	rm -f $(MD_FILES)

# Phony targets
.PHONY: all clean