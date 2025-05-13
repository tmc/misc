# HTML to Markdown Tool Comparison

This document compares various HTML to Markdown conversion tools, including html2md.

## Feature Comparison

| Feature | html2md | Pandoc | Turndown (JS) | html2text (Python) |
|---------|---------|--------|--------------|-------------------|
| Tables | ✅ Good | ✅ Good | ❌ Poor | ✅ Average |
| Code Blocks | ✅ Good | ✅ Good | ✅ Good | ⚠️ Extra spacing |
| Nested Lists | ✅ Good | ✅ Good | ✅ Good | ✅ Good |
| Images | ✅ Markdown | ⚠️ HTML | ✅ Markdown | ✅ Markdown |
| Horizontal Rules | ✅ Good | ✅ Different style | ✅ Good | ✅ Good |
| Links | ✅ Good | ✅ Good | ✅ Good | ✅ Good |
| HTML Sanitization | ✅ Optional | ❌ No | ❌ No | ❌ No |
| GitHub Flavored Markdown | ✅ Built-in | ✅ Optional flag | ⚠️ Via plugins | ❌ No |
| Task Lists | ✅ Supported | ✅ Supported | ❌ Requires plugin | ❌ Not supported |
| Strikethrough | ✅ Supported | ✅ Supported | ⚠️ Via plugins | ❌ Not supported |
| Heading Style | # Headers | # Headers | === Underlines | # Headers |
| Integration | Go | Many | JavaScript | Python |
| CLI Tool | ✅ Yes | ✅ Yes | ❌ No (library) | ✅ Yes |

## Output Examples

### html2md (with GFM support)

```markdown
# Heading

This is **bold text** and this is _italic text_.

Task list:
- [x] Completed task
- [ ] Incomplete task

| Header 1 | Header 2 |
| -------- | -------- |
| Row 1    | Row 2    |

~~Strikethrough text~~
```

### Pandoc (with GFM flag)

```markdown
# Heading

This is **bold text** and this is *italic text*.

Task list:
- [x] Completed task
- [ ] Incomplete task

| Header 1 | Header 2 |
|----------|----------|
| Row 1    | Row 2    |

~~Strikethrough text~~
```

### Turndown (JavaScript)

```markdown
Heading
=======

This is **bold text** and this is _italic text_.

Task list:
- [x] Completed task
- [ ] Incomplete task

Header 1

Header 2

Row 1

Row 2

~~Strikethrough text~~
```

### html2text (Python)

```markdown
# Heading

This is **bold text** and this is _italic text_.

Task list:
- [x] Completed task
- [ ] Incomplete task

Header 1 | Header 2  
---|---  
Row 1 | Row 2  

~~Strikethrough text~~
```

## Conclusion

html2md offers a well-rounded solution with good handling of all HTML elements and the added benefit of optional HTML sanitization. It produces clean, GitHub-flavored Markdown output with proper code block, table, and list formatting. The built-in support for GitHub Flavored Markdown means features like task lists, tables, and strikethrough are supported natively without additional configuration. 