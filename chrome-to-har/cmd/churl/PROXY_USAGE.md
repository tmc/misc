# Proxy Support in churl

The `churl` command supports both HTTP/HTTPS and SOCKS5 proxies, with optional authentication and bypass lists.

## Basic Usage

### HTTP/HTTPS Proxy

```bash
# Basic HTTP proxy
churl --proxy http://proxy.example.com:8080 https://example.com

# HTTP proxy with authentication
churl --proxy http://proxy.example.com:8080 --proxy-user username:password https://example.com

# HTTPS proxy
churl --proxy https://proxy.example.com:8080 https://example.com
```

### SOCKS5 Proxy

```bash
# Basic SOCKS5 proxy
churl --socks5-proxy socks5://proxy.example.com:1080 https://example.com

# SOCKS5 proxy with authentication
churl --socks5-proxy socks5://proxy.example.com:1080 --proxy-user username:password https://example.com
```

## Advanced Options

### Proxy Bypass List

Skip the proxy for specific hosts:

```bash
# Bypass proxy for localhost and internal domains
churl --proxy http://proxy.example.com:8080 --proxy-bypass "localhost,127.0.0.1,*.internal.com" https://example.com

# Multiple bypass entries
churl --proxy http://proxy.example.com:8080 --proxy-bypass "localhost,*.local,10.0.0.0/8" https://example.com
```

### Authentication

Proxy authentication is supported for both HTTP and SOCKS5 proxies:

```bash
# Username and password authentication
churl --proxy http://proxy.example.com:8080 --proxy-user myuser:mypass https://example.com

# Works with SOCKS5 too
churl --socks5-proxy socks5://proxy.example.com:1080 --proxy-user myuser:mypass https://example.com
```

## Output Formats with Proxy

All output formats work with proxy:

```bash
# HTML output through proxy
churl --proxy http://proxy.example.com:8080 https://example.com

# HAR output through proxy
churl --proxy http://proxy.example.com:8080 --output-format har https://example.com

# JSON output through proxy
churl --proxy http://proxy.example.com:8080 --output-format json https://example.com

# Save to file
churl --proxy http://proxy.example.com:8080 -o output.html https://example.com
```

## Common Use Cases

### Corporate Proxy

```bash
# Corporate proxy with authentication
churl --proxy http://corporate-proxy.company.com:8080 \
      --proxy-user domain\\username:password \
      --proxy-bypass "*.company.com,localhost" \
      https://external-site.com
```

### Testing Through Proxy

```bash
# Test site loading through proxy
churl --proxy http://proxy.example.com:8080 --verbose https://example.com

# Check what headers are sent
churl --proxy http://proxy.example.com:8080 --output-format json https://httpbin.org/headers
```

### SOCKS5 Proxy (e.g., SSH tunnel)

```bash
# Use SSH tunnel as SOCKS5 proxy
# First, set up SSH tunnel: ssh -D 8080 user@server
churl --socks5-proxy socks5://localhost:8080 https://example.com
```

## Troubleshooting

### Verbose Mode

Use `--verbose` to see detailed proxy information:

```bash
churl --proxy http://proxy.example.com:8080 --verbose https://example.com
```

This will show:
- Proxy server being used
- Authentication status
- Bypass list configuration
- Connection details

### Common Issues

1. **Authentication failures**: Ensure username:password format is correct
2. **Connection timeouts**: Check proxy server availability
3. **SSL/TLS issues**: Some proxies may have certificate problems
4. **Bypass not working**: Verify bypass list syntax

### Testing Proxy Configuration

Test if your proxy works:

```bash
# Test basic connectivity
churl --proxy http://proxy.example.com:8080 --verbose https://httpbin.org/ip

# Test authentication
churl --proxy http://proxy.example.com:8080 --proxy-user test:test --verbose https://httpbin.org/ip

# Test bypass
churl --proxy http://proxy.example.com:8080 --proxy-bypass httpbin.org --verbose https://httpbin.org/ip
```

## Security Considerations

1. **Credentials in command line**: Be careful with proxy credentials in command line (visible in process list)
2. **HTTPS through proxy**: Ensure your proxy supports HTTPS CONNECT method
3. **Certificate validation**: Proxy may intercept SSL certificates
4. **Logging**: Proxy servers may log all traffic

## Implementation Notes

- Chrome's `--proxy-server` flag is used internally
- Authentication is handled via Chrome DevTools Protocol
- Bypass list uses Chrome's `--proxy-bypass-list` flag
- SOCKS5 support is native to Chrome
- Proxy authentication works for both HTTP and SOCKS5 proxies

## Exit Codes

- `0`: Success
- `1`: General error (invalid flags, network issues, etc.)
- `2`: Authentication failure
- `3`: Proxy connection failure