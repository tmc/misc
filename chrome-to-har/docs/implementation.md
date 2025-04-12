# Chrome-to-HAR Implementation Notes

## Current Status

We have implemented the following components:

1. **Browser Package**: 
   - Created a shared browser management package in `internal/browser`
   - Implemented Chrome launching, navigation, and interaction
   - Added options pattern for browser configuration

2. **Churl Command**:
   - Implemented curl-like functionality running through Chrome
   - Added support for custom headers, basic auth, and different output formats
   - Created documentation for the command

3. **Output Formats**:
   - Added HTML, text, HAR, and JSON output support
   - Implemented basic text extraction from HTML

## Next Steps

1. **Chrome-to-HAR Refactoring**:
   - Move the existing chrome-to-har code to use the new browser package
   - Extract shared code to improve maintainability
   - Implement differential capture mode properly

2. **Churl Enhancements**:
   - Add support for POST data with different content types
   - Improve the text extraction for better plain text output
   - Add extraction of specific elements via CSS selectors
   - Add script injection capabilities
   - Implement better wait strategies

3. **Documentation and Testing**:
   - Add more comprehensive documentation
   - Add more examples and use cases
   - Increase test coverage
   - Add CI/CD integration

## Technical Debt

1. Error handling could be improved in some areas
2. Need to ensure all resources are properly cleaned up
3. Better handling of Chrome crashes and recoveries
4. More robust cookie and header handling

## Future Enhancements

1. **Proxy Support**: Add support for proxying requests through custom proxies
2. **Certificate Handling**: Add support for custom certificates and ignoring certificate errors
3. **Screenshots**: Add capability to capture screenshots of pages or elements
4. **PDF Export**: Add PDF generation from web pages
5. **Network Throttling**: Simulate different network conditions
6. **User Agents**: Easy switching between different user agents
7. **Geolocation**: Spoofing geolocation for testing region-specific content