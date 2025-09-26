# Template Examples

This directory contains example template variations for testing different appearances and features.

## Usage

To test an example template set, use the `-templates` flag:

```bash
# Test auto-appearance templates (light/dark based on system preference)
./md2html -http :8080 -templates examples/auto-appearance-templates

# Test dark-mode templates (always dark)
./md2html -http :8080 -templates examples/dark-mode-templates

# Use default templates
./md2html -http :8080 -templates templates
```

## Available Examples

### `auto-appearance-templates/`
- Automatically switches between light and dark themes based on system preference
- Uses CSS `prefers-color-scheme` media query
- Mermaid diagrams automatically adapt to theme
- Smooth transitions between themes

### `dark-mode-templates/`
- Always uses dark theme regardless of system preference
- GitHub Dark theme colors
- Optimized for dark theme viewing
- Custom scrollbar styling

## Creating New Examples

To create a new template variant:

1. Create a new directory under `examples/`
2. Override only the templates you want to change
3. Most commonly, you'll only need to override `styles.html`
4. The base templates (`layout`, `header`, `content`, `scripts`) will be used automatically

Example structure:
```
examples/my-custom-theme/
└── styles.html    # Override the styles block
```