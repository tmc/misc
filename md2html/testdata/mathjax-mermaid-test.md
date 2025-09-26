# MathJax and Mermaid Test

## MathJax Examples

Here's some inline math: $E = mc^2$ and $\alpha + \beta = \gamma$.

Display math:
$$\frac{d}{dx} \int_a^x f(t) dt = f(x)$$

Complex equation:
$$\sum_{n=1}^{\infty} \frac{1}{n^2} = \frac{\pi^2}{6}$$

## Mermaid Diagram

```mermaid
graph TD
    A[Start] --> B{Is it working?}
    B -->|Yes| C[Great!]
    B -->|No| D[Debug]
    D --> B
```

## Another Mermaid Diagram

```mermaid
sequenceDiagram
    participant A as Alice
    participant B as Bob
    A->>B: Hello Bob!
    B->>A: Hello Alice!
```