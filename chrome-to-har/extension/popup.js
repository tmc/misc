// Enhanced popup script for AI API Bridge with error recovery

// Error recovery configuration
const errorRecoveryConfig = {
  maxRetries: 3,
  baseDelay: 1000,
  maxDelay: 10000,
  retryableErrors: [
    'Extension context invalidated',
    'Chrome extension was invalidated',
    'timeout',
    'connection lost',
    'native messaging unavailable',
    'could not establish connection'
  ]
};

// Graceful degradation modes
const degradationModes = {
  FULL_FUNCTIONALITY: 'full',
  LIMITED_FUNCTIONALITY: 'limited',
  FALLBACK_MODE: 'fallback',
  OFFLINE_MODE: 'offline'
};

let currentMode = degradationModes.FULL_FUNCTIONALITY;
let reconnectAttempts = 0;
let maxReconnectAttempts = 5;

// Utility functions for error recovery
function isRetryableError(error) {
  if (!error || !error.message) return false;
  return errorRecoveryConfig.retryableErrors.some(retryable => 
    error.message.toLowerCase().includes(retryable.toLowerCase())
  );
}

function calculateDelay(attempt) {
  const delay = Math.min(
    errorRecoveryConfig.baseDelay * Math.pow(2, attempt),
    errorRecoveryConfig.maxDelay
  );
  // Add jitter to prevent thundering herd
  return delay + (Math.random() * 0.1 * delay);
}

async function retryWithBackoff(fn, context = 'operation') {
  let lastError = null;
  
  for (let attempt = 0; attempt < errorRecoveryConfig.maxRetries; attempt++) {
    try {
      if (attempt > 0) {
        const delay = calculateDelay(attempt - 1);
        console.log(`Retrying ${context} (attempt ${attempt + 1}/${errorRecoveryConfig.maxRetries}) after ${Math.round(delay)}ms`);
        await new Promise(resolve => setTimeout(resolve, delay));
      }
      
      return await fn();
    } catch (error) {
      lastError = error;
      console.error(`${context} failed (attempt ${attempt + 1}):`, error);
      
      if (!isRetryableError(error)) {
        throw error; // Don't retry non-retryable errors
      }
    }
  }
  
  throw lastError;
}

function setDegradationMode(mode) {
  currentMode = mode;
  updateUIForMode(mode);
}

function updateUIForMode(mode) {
  const modeIndicator = document.getElementById('mode-indicator') || createModeIndicator();
  
  switch (mode) {
    case degradationModes.FULL_FUNCTIONALITY:
      modeIndicator.textContent = '';
      modeIndicator.className = 'mode-indicator';
      break;
    case degradationModes.LIMITED_FUNCTIONALITY:
      modeIndicator.textContent = '⚠️ Limited functionality';
      modeIndicator.className = 'mode-indicator warning';
      break;
    case degradationModes.FALLBACK_MODE:
      modeIndicator.textContent = '🔄 Fallback mode';
      modeIndicator.className = 'mode-indicator fallback';
      break;
    case degradationModes.OFFLINE_MODE:
      modeIndicator.textContent = '📱 Offline mode';
      modeIndicator.className = 'mode-indicator offline';
      break;
  }
}

function createModeIndicator() {
  const indicator = document.createElement('div');
  indicator.id = 'mode-indicator';
  indicator.className = 'mode-indicator';
  document.body.insertBefore(indicator, document.body.firstChild);
  return indicator;
}

document.addEventListener('DOMContentLoaded', async () => {
  const statusDiv = document.getElementById('status');
  const diagnosticsDiv = document.getElementById('diagnostics');
  const setupHelpDiv = document.getElementById('setup-help');
  const controlsDiv = document.getElementById('controls');
  const resultDiv = document.getElementById('result');
  
  // Check AI API status with diagnostics and error recovery
  async function checkStatus() {
    try {
      statusDiv.textContent = 'Checking AI API availability...';
      statusDiv.className = 'status unavailable';
      
      const apiStatus = await retryWithBackoff(async () => {
        // First check basic API existence
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        
        if (!tab || !tab.id) {
          throw new Error('No active tab available');
        }
        
        const basicCheck = await chrome.scripting.executeScript({
          target: { tabId: tab.id },
          func: () => ({
            languageModelExists: typeof LanguageModel !== 'undefined',
            windowAiExists: typeof window.ai !== 'undefined',
            chromeAiExists: typeof chrome !== 'undefined' && typeof chrome.ai !== 'undefined',
            userAgent: navigator.userAgent,
            location: window.location.href,
            timestamp: Date.now()
          })
        });
        
        if (!basicCheck || !basicCheck[0] || !basicCheck[0].result) {
          throw new Error('Failed to get API status from content script');
        }
        
        return { ...basicCheck[0].result, tabId: tab.id };
      }, 'API status check');
      
      // Show diagnostics
      diagnosticsDiv.innerHTML = `
        URL: ${apiStatus.location}<br>
        LanguageModel: ${apiStatus.languageModelExists ? '✅' : '❌'}<br>
        window.ai: ${apiStatus.windowAiExists ? '✅' : '❌'}<br>
        chrome.ai: ${apiStatus.chromeAiExists ? '✅' : '❌'}<br>
        <small>Checked at: ${new Date(apiStatus.timestamp).toLocaleTimeString()}</small>
      `;
      diagnosticsDiv.style.display = 'block';
      
      if (apiStatus.languageModelExists || apiStatus.windowAiExists || apiStatus.chromeAiExists) {
        // Test API functionality
        await testAPIFunctionality(apiStatus.tabId, apiStatus);
        setDegradationMode(degradationModes.FULL_FUNCTIONALITY);
      } else {
        // No APIs found - show setup help
        showSetupHelp();
        setDegradationMode(degradationModes.FALLBACK_MODE);
      }
      
      // Reset reconnect attempts on success
      reconnectAttempts = 0;
      
    } catch (error) {
      console.error('Status check error:', error);
      await handleStatusCheckError(error);
    }
  }
  
  // Handle errors during status check with progressive degradation
  async function handleStatusCheckError(error) {
    reconnectAttempts++;
    
    if (reconnectAttempts < maxReconnectAttempts && isRetryableError(error)) {
      statusDiv.innerHTML = `⚠️ Connection issue (attempt ${reconnectAttempts}/${maxReconnectAttempts}). Retrying...`;
      statusDiv.className = 'status retrying';
      setDegradationMode(degradationModes.LIMITED_FUNCTIONALITY);
      
      const delay = calculateDelay(reconnectAttempts - 1);
      setTimeout(() => checkStatus(), delay);
    } else {
      // Maximum retries reached or non-retryable error
      statusDiv.innerHTML = `❌ Error: ${error.message}`;
      statusDiv.className = 'status unavailable';
      
      if (reconnectAttempts >= maxReconnectAttempts) {
        setDegradationMode(degradationModes.OFFLINE_MODE);
        showOfflineMode();
      } else {
        setDegradationMode(degradationModes.FALLBACK_MODE);
        showSetupHelp();
      }
    }
  }
  
  // Show offline mode with limited functionality
  function showOfflineMode() {
    setupHelpDiv.innerHTML = `
      <h3>🔌 Connection Lost</h3>
      <p>Unable to connect to AI APIs after multiple attempts.</p>
      <p>You can still:</p>
      <ul>
        <li>View extension settings</li>
        <li>Access help documentation</li>
        <li>Try manual reconnection</li>
      </ul>
      <button id="forceReconnect">Try Again</button>
    `;
    setupHelpDiv.style.display = 'block';
    controlsDiv.style.display = 'none';
    
    document.getElementById('forceReconnect').addEventListener('click', () => {
      reconnectAttempts = 0;
      checkStatus();
    });
  }
  
  // Test API functionality with enhanced error handling
  async function testAPIFunctionality(tabId, apiStatus) {
    try {
      const result = await retryWithBackoff(async () => {
        const funcTest = await chrome.scripting.executeScript({
          target: { tabId: tabId },
          func: async (apiStatus) => {
            try {
              if (apiStatus.languageModelExists) {
                const availability = await LanguageModel.availability();
                return { api: 'LanguageModel', availability, success: true };
              } else if (apiStatus.windowAiExists) {
                const capabilities = await window.ai.capabilities();
                return { api: 'window.ai', capabilities, success: true };
              } else if (apiStatus.chromeAiExists) {
                return { api: 'chrome.ai', detected: true, success: true };
              }
              return { success: false, error: 'No API available' };
            } catch (error) {
              return { success: false, error: error.message };
            }
          },
          args: [apiStatus]
        });
        
        if (!funcTest || !funcTest[0]) {
          throw new Error('Failed to execute API functionality test');
        }
        
        return funcTest[0].result;
      }, 'API functionality test');
      
      if (result.success) {
        statusDiv.innerHTML = `
          <strong>✅ AI API Available!</strong><br>
          Type: ${result.api}<br>
          ${result.availability ? `Status: ${result.availability}` : ''}
          ${result.capabilities ? 'Capabilities detected' : ''}
        `;
        statusDiv.className = 'status available';
        controlsDiv.style.display = 'block';
        setupHelpDiv.style.display = 'none';
      } else {
        statusDiv.innerHTML = `❌ AI API Error: ${result.error}`;
        statusDiv.className = 'status unavailable';
        setDegradationMode(degradationModes.LIMITED_FUNCTIONALITY);
        showSetupHelp();
      }
    } catch (error) {
      console.error('API functionality test failed:', error);
      statusDiv.innerHTML = `❌ Functionality test failed: ${error.message}`;
      statusDiv.className = 'status unavailable';
      setDegradationMode(degradationModes.LIMITED_FUNCTIONALITY);
      showSetupHelp();
    }
  }
  
  // Show setup help with mode-specific guidance
  function showSetupHelp() {
    let helpContent = `
      <h3>🔧 Setup Required</h3>
      <p>AI APIs are not available. Please follow these steps:</p>
      <ol>
        <li>Enable Chrome AI flags</li>
        <li>Download the AI model</li>
        <li>Restart Chrome</li>
        <li>Refresh this extension</li>
      </ol>
    `;
    
    if (currentMode === degradationModes.LIMITED_FUNCTIONALITY) {
      helpContent += `
        <div class="warning">
          <strong>⚠️ Limited Functionality Mode</strong><br>
          Some features may be unavailable. The extension will continue trying to restore full functionality.
        </div>
      `;
    }
    
    setupHelpDiv.innerHTML = helpContent;
    setupHelpDiv.style.display = 'block';
    controlsDiv.style.display = 'none';
  }
  
  // Test text generation with graceful degradation
  async function testGeneration() {
    resultDiv.style.display = 'block';
    resultDiv.textContent = 'Generating text...';
    
    try {
      const result = await retryWithBackoff(async () => {
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        
        if (!tab || !tab.id) {
          throw new Error('No active tab available');
        }
        
        const genResult = await chrome.scripting.executeScript({
          target: { tabId: tab.id },
          func: async () => {
            try {
              if (typeof LanguageModel !== 'undefined') {
                const model = await LanguageModel.create();
                const response = await model.generate('Hello! Please introduce yourself briefly.');
                return { success: true, response, api: 'LanguageModel' };
              } else if (typeof window.ai !== 'undefined') {
                const session = await window.ai.createTextSession();
                const response = await session.prompt('Hello! Please introduce yourself briefly.');
                return { success: true, response, api: 'window.ai' };
              } else {
                return { success: false, error: 'No AI API available' };
              }
            } catch (error) {
              return { success: false, error: error.message };
            }
          }
        });
        
        if (!genResult || !genResult[0]) {
          throw new Error('Failed to execute generation test');
        }
        
        return genResult[0].result;
      }, 'text generation');
      
      if (result.success) {
        resultDiv.innerHTML = `
          <strong>🎉 Generation Success!</strong><br>
          API: ${result.api}<br>
          Response: "${result.response}"
        `;
        setDegradationMode(degradationModes.FULL_FUNCTIONALITY);
      } else {
        resultDiv.textContent = `❌ Generation failed: ${result.error}`;
        if (result.error.includes('model not ready')) {
          setDegradationMode(degradationModes.LIMITED_FUNCTIONALITY);
          resultDiv.innerHTML += '<br><small>Try downloading the AI model from chrome://components</small>';
        }
      }
    } catch (error) {
      console.error('Generation test failed:', error);
      resultDiv.innerHTML = `❌ Generation error: ${error.message}`;
      
      if (currentMode === degradationModes.FULL_FUNCTIONALITY) {
        setDegradationMode(degradationModes.LIMITED_FUNCTIONALITY);
      }
      
      // Provide helpful error-specific guidance
      if (error.message.includes('Extension context invalidated')) {
        resultDiv.innerHTML += '<br><small>Please refresh the extension and try again</small>';
      } else if (error.message.includes('timeout')) {
        resultDiv.innerHTML += '<br><small>Request timed out. The AI model may be initializing</small>';
      }
    }
  }
  
  // Test availability details
  async function testAvailability() {
    resultDiv.style.display = 'block';
    resultDiv.textContent = 'Testing availability...';
    
    try {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      
      const availResult = await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        func: async () => {
          const results = {};
          
          if (typeof LanguageModel !== 'undefined') {
            try {
              results.languageModel = await LanguageModel.availability();
            } catch (e) {
              results.languageModel = `Error: ${e.message}`;
            }
          }
          
          if (typeof window.ai !== 'undefined') {
            try {
              results.windowAi = await window.ai.capabilities();
            } catch (e) {
              results.windowAi = `Error: ${e.message}`;
            }
          }
          
          return results;
        }
      });
      
      const result = availResult[0].result;
      resultDiv.innerHTML = `
        <strong>Availability Details:</strong><br>
        ${JSON.stringify(result, null, 2)}
      `;
    } catch (error) {
      resultDiv.textContent = `❌ Availability test error: ${error.message}`;
    }
  }
  
  // Open chrome://flags
  document.getElementById('openFlags').addEventListener('click', () => {
    chrome.tabs.create({ url: 'chrome://flags/#prompt-api-for-gemini-nano' });
  });
  
  // Open chrome://components
  document.getElementById('openComponents').addEventListener('click', () => {
    chrome.tabs.create({ url: 'chrome://components/' });
  });
  
  // Enhanced event listeners with error handling
  document.getElementById('checkStatus').addEventListener('click', async () => {
    try {
      await checkStatus();
    } catch (error) {
      console.error('Status check failed:', error);
      resultDiv.textContent = `Error during status check: ${error.message}`;
      resultDiv.style.display = 'block';
    }
  });
  
  document.getElementById('testGenerate').addEventListener('click', async () => {
    try {
      await testGeneration();
    } catch (error) {
      console.error('Generation test failed:', error);
      resultDiv.textContent = `Error during generation test: ${error.message}`;
      resultDiv.style.display = 'block';
    }
  });
  
  document.getElementById('testAvailability').addEventListener('click', async () => {
    try {
      await testAvailability();
    } catch (error) {
      console.error('Availability test failed:', error);
      resultDiv.textContent = `Error during availability test: ${error.message}`;
      resultDiv.style.display = 'block';
    }
  });
  
  // Health check interval for ongoing monitoring
  let healthCheckInterval = null;
  
  function startHealthCheck() {
    if (healthCheckInterval) clearInterval(healthCheckInterval);
    
    healthCheckInterval = setInterval(async () => {
      try {
        // Light health check - just verify tab access
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        if (!tab) {
          throw new Error('No active tab');
        }
        
        // If we're in degraded mode, try to recover
        if (currentMode !== degradationModes.FULL_FUNCTIONALITY) {
          console.log('Attempting to recover from degraded mode...');
          await checkStatus();
        }
      } catch (error) {
        console.warn('Health check failed:', error);
        if (currentMode === degradationModes.FULL_FUNCTIONALITY) {
          setDegradationMode(degradationModes.LIMITED_FUNCTIONALITY);
        }
      }
    }, 30000); // Check every 30 seconds
  }
  
  function stopHealthCheck() {
    if (healthCheckInterval) {
      clearInterval(healthCheckInterval);
      healthCheckInterval = null;
    }
  }
  
  // Cleanup on popup close
  window.addEventListener('beforeunload', stopHealthCheck);
  
  // Initial status check
  await checkStatus();
  
  // Start health monitoring
  startHealthCheck();
});