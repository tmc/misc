# Chrome Version Compatibility for AI APIs

## Current Status (December 2024)

Based on comprehensive testing and research, here's the compatibility matrix for Chrome AI APIs across different versions and channels:

## Chrome Channels and AI API Support

### Chrome Canary (140+)
**Status**: ✅ Full AI API Support
- **LanguageModel API**: Available with flags
- **window.ai API**: Available with flags  
- **chrome.ai API**: Experimental support
- **Model Download**: Functional via chrome://components
- **Extension Access**: Confirmed working
- **Flags Required**: 
  - `--enable-features=PromptAPIForGeminiNano,OptimizationGuideOnDeviceModel`
  - Manual chrome://flags configuration

### Chrome Beta (127+)
**Status**: ✅ Expected Support (Not Fully Tested)
- **LanguageModel API**: Should be available with flags
- **window.ai API**: Should be available with flags
- **Model Download**: Expected to work
- **Extension Access**: Expected to work
- **Testing Status**: Requires verification

### Chrome Stable (Current)
**Status**: ❌ No AI API Support
- **LanguageModel API**: Not available
- **window.ai API**: Not available
- **Future Timeline**: Expected in Chrome 138+ stable release
- **Current Workaround**: Use Canary or Beta channels

## Version-Specific Features

### Chrome 140+ Canary
```javascript
// All AI APIs available
const hasLanguageModel = typeof LanguageModel !== 'undefined';
const hasWindowAI = typeof window.ai !== 'undefined';
const hasChromeAI = typeof chrome.ai !== 'undefined';

// Expected: All true with proper flags
```

### Chrome 138+ Beta (Expected)
```javascript
// Primary APIs available
const hasLanguageModel = typeof LanguageModel !== 'undefined';
const hasWindowAI = typeof window.ai !== 'undefined';

// Expected: Both true with proper flags
```

### Chrome 127+ Beta (Current)
```javascript
// Limited availability, flags required
const hasLanguageModel = typeof LanguageModel !== 'undefined';
// Expected: true with --enable-features flags
```

## Testing Framework

### Version Detection Script
```javascript
// Detect Chrome version and AI capability
function detectChromeAISupport() {
  const userAgent = navigator.userAgent;
  const chromeMatch = userAgent.match(/Chrome\/(\d+)/);
  const version = chromeMatch ? parseInt(chromeMatch[1]) : 0;
  
  return {
    version: version,
    channel: detectChannel(),
    aiSupport: {
      languageModel: typeof LanguageModel !== 'undefined',
      windowAI: typeof window.ai !== 'undefined',
      chromeAI: typeof chrome !== 'undefined' && typeof chrome.ai !== 'undefined'
    },
    flags: detectFlags(),
    recommendation: getRecommendation(version)
  };
}

function detectChannel() {
  const userAgent = navigator.userAgent;
  if (userAgent.includes('Chrome/140')) return 'canary';
  if (userAgent.includes('beta')) return 'beta';
  return 'stable';
}

function getRecommendation(version) {
  if (version >= 140) return 'Full AI support available';
  if (version >= 127) return 'Use Beta channel for AI features';
  return 'Upgrade to Chrome Beta/Canary for AI APIs';
}
```

## Installation and Setup by Channel

### Chrome Canary Setup
```bash
# macOS
brew install --cask google-chrome-canary

# Launch with AI flags
open -a "Google Chrome Canary" --args \
  --enable-features=PromptAPIForGeminiNano,OptimizationGuideOnDeviceModel \
  --optimization-guide-on-device-model=enabled

# Manual flags setup required at chrome://flags
```

### Chrome Beta Setup
```bash
# macOS  
brew install --cask google-chrome-beta

# Launch with AI flags
open -a "Google Chrome Beta" --args \
  --enable-features=PromptAPIForGeminiNano,OptimizationGuideOnDeviceModel

# Manual flags setup required at chrome://flags
```

## Extension Compatibility

### Manifest V3 Requirements
All Chrome versions supporting AI APIs require Manifest V3 extensions:

```json
{
  "manifest_version": 3,
  "permissions": [
    "activeTab",
    "storage", 
    "nativeMessaging",
    "scripting",
    "tabs"
  ]
}
```

### Extension AI Detection
```javascript
// Universal AI detection for extensions
chrome.scripting.executeScript({
  target: { tabId: tabId },
  func: () => ({
    chromeVersion: navigator.userAgent.match(/Chrome\/(\d+)/)?.[1],
    aiAPIs: {
      languageModel: typeof LanguageModel !== 'undefined',
      windowAI: typeof window.ai !== 'undefined',
      chromeAI: typeof chrome?.ai !== 'undefined'
    }
  })
});
```

## Common Issues by Version

### Chrome 140+ Canary
- **Issue**: Model download may fail initially
- **Solution**: Visit chrome://components, check "Optimization Guide On Device Model"
- **Issue**: Extension permissions may reset
- **Solution**: Reload extension after Chrome updates

### Chrome Beta (127+)
- **Issue**: Inconsistent AI API availability 
- **Solution**: Verify flag configuration after each update
- **Issue**: Different flag names between versions
- **Solution**: Test multiple flag combinations

### Chrome Stable
- **Issue**: No AI API support
- **Solution**: Switch to Beta/Canary channel
- **Timeline**: Wait for Chrome 138+ stable release

## Development Recommendations

### Multi-Version Testing
1. **Test on Canary**: Primary development and testing
2. **Validate on Beta**: Compatibility verification
3. **Plan for Stable**: Prepare for general availability

### Feature Detection
```javascript
// Robust feature detection
function checkAIAvailability() {
  return new Promise((resolve) => {
    const result = {
      supported: false,
      version: 'unknown',
      apis: {},
      recommendations: []
    };
    
    // Version detection
    const chromeMatch = navigator.userAgent.match(/Chrome\/(\d+)/);
    if (chromeMatch) {
      result.version = chromeMatch[1];
    }
    
    // API detection
    result.apis.languageModel = typeof LanguageModel !== 'undefined';
    result.apis.windowAI = typeof window.ai !== 'undefined';
    result.apis.chromeAI = typeof chrome?.ai !== 'undefined';
    
    result.supported = Object.values(result.apis).some(Boolean);
    
    // Recommendations
    if (!result.supported) {
      if (parseInt(result.version) < 127) {
        result.recommendations.push('Upgrade to Chrome Beta (127+)');
      } else {
        result.recommendations.push('Enable AI flags in chrome://flags');
        result.recommendations.push('Download AI model from chrome://components');
      }
    }
    
    resolve(result);
  });
}
```

## Future Roadmap

### Expected Timeline
- **Q1 2025**: Chrome 138+ stable with AI APIs
- **Q2 2025**: Widespread AI API availability
- **Q3 2025**: Enhanced AI capabilities and new APIs

### Preparation Steps
1. Develop and test on current Canary builds
2. Monitor Chrome release notes for API changes
3. Prepare fallback mechanisms for older versions
4. Plan migration strategy for stable release

## Testing Commands

### Automated Version Testing
```bash
# Test current installation
go run cmd/ai-setup-check/main.go

# Test extension compatibility
go run cmd/e2e-test/main.go

# Manual Chrome launch for testing
chrome --version
chrome --enable-features=PromptAPIForGeminiNano
```

This compatibility guide will be updated as new Chrome versions are released and AI API support evolves.