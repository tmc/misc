# Code Examples

## Go
```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
```

## JavaScript
```javascript
function highlightTest() {
    const message = "Hello World";
    console.log(message);
    return true;
}
```

## Python
```python
def test_colors():
    message = "Hello Python"
    print(message)
    return True

# This is a comment
if __name__ == "__main__":
    test_colors()
```

## Bash/Shell
```bash
#!/bin/bash
echo "Hello from bash"
export PATH="/usr/local/bin:$PATH"
for file in *.md; do
    echo "Processing $file"
done
```

## Terminal/Console
```console
$ npm install
$ go build .
$ ./md2html -http :8080
```

## Diff
```diff
- old line
+ new line
  unchanged line
```

## JSON
```json
{
  "name": "md2html",
  "version": "1.0.0",
  "dependencies": {
    "chroma": "^2.0.0"
  }
}
```

## YAML
```yaml
name: md2html
version: 1.0.0
dependencies:
  chroma: ^2.0.0
  goldmark: latest
```

## HTML
```html
<!DOCTYPE html>
<html>
<head>
    <title>Test</title>
</head>
<body>
    <h1>Hello World</h1>
</body>
</html>
```

## CSS
```css
.chroma .kn {
    color: #ff7b72;
    font-weight: bold;
}

.chroma .s {
    color: #a5d6ff;
}
```
