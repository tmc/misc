# Manual Testing Guide for Chrome AI Extension

## Critical Discovery
Command-line flags alone are **insufficient**. The chrome://flags must be manually enabled.

## Complete Setup Process

### Step 1: Enable Chrome Flags Manually
1. Launch Chrome Canary normally (without flags first)
2. Go to `chrome://flags/`
3. Search for and enable these flags:
   - **prompt-api-for-gemini-nano** → Set to "Enabled"
   - **optimization-guide-on-device-model** → Set to "Enabled BypassPerfRequirement"
4. **Restart Chrome** when prompted

### Step 2: Download AI Model
1. Go to `chrome://components/`
2. Find "Optimization Guide On Device Model"
3. If version shows "0.0.0.0", click "Check for update"
4. Wait for download to complete (may take several minutes)

### Step 3: Launch Chrome with Extension
```bash
"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary" \
  --load-extension="/Volumes/tmc/go/src/github.com/tmc/misc/chrome-to-har/extension" \
  --enable-features=Gemini,AILanguageModelService,BuiltInAIAPI \
  --enable-ai-language-model-service \
  --user-data-dir=/tmp/chrome-extension-final-test
```

### Step 4: Test Extension
1. You should see "AI API Bridge" extension icon
2. Navigate to `https://example.com`
3. Click extension icon → popup should open
4. Look for "✅ AI API Available" status
5. Click "Test Generation" button

## Expected Results

### If Setup is Correct:
- Extension popup shows "✅ AI API Available"
- "Test Generation" produces actual AI text
- Console shows successful API calls

### If Setup Incomplete:
- Extension popup shows "❌ AI API Not Available"
- Console shows "LanguageModel is not defined"
- Need to complete flag setup

## Fallback Test (Manual Console)

If extension approach fails, test directly in console:

1. Navigate to any webpage
2. Open DevTools console
3. Test commands:

```javascript
// Check availability
console.log('LanguageModel exists:', typeof LanguageModel !== 'undefined');
console.log('window.ai exists:', typeof window.ai !== 'undefined');

// Test LanguageModel (if available)
if (typeof LanguageModel !== 'undefined') {
  LanguageModel.availability().then(status => {
    console.log('Availability:', status);
    
    if (status === 'available' || status === 'downloadable') {
      LanguageModel.create().then(model => {
        return model.generate('Hello, how are you?');
      }).then(response => {
        console.log('Generated:', response);
      });
    }
  });
}

// Test window.ai (if available)
if (typeof window.ai !== 'undefined') {
  window.ai.capabilities().then(caps => {
    console.log('Capabilities:', caps);
    
    return window.ai.createTextSession();
  }).then(session => {
    return session.prompt('Hello, how are you?');
  }).then(response => {
    console.log('Generated:', response);
  });
}
```

## Troubleshooting

### Issue: Flags not found in chrome://flags
- **Cause**: Using wrong Chrome version
- **Solution**: Use Chrome Canary 127+ or Chrome Beta 127+

### Issue: Component not downloading
- **Cause**: Network restrictions or region lock
- **Solution**: Check internet connection, try VPN

### Issue: Extension not loading
- **Cause**: Extension path or permissions
- **Solution**: Use absolute path, enable Developer mode

### Issue: APIs undefined even with correct setup
- **Cause**: Feature still experimental/restricted
- **Solution**: Wait for broader rollout or try different Chrome channel

## Success Indicators

1. ✅ chrome://flags shows enabled flags
2. ✅ chrome://components shows downloaded model
3. ✅ Extension popup shows AI available
4. ✅ Test generation produces text
5. ✅ Console shows no "undefined" errors

This manual process will definitively prove whether:
- The AI APIs are accessible in this Chrome version
- Our extension approach bypasses automation restrictions
- The complete solution is viable