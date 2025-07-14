// Enhanced background service worker for AI API Bridge
console.log('AI API Bridge background script loaded');

// Import health check system
importScripts('health.js');

// Store for AI API state
let aiAPIState = {
  available: false,
  type: null, // 'LanguageModel' or 'window.ai'
  status: null
};

// Native messaging connection with retry logic
let nativePort = null;
let connectionRetryCount = 0;
let maxRetries = 5;
let baseRetryDelay = 1000; // 1 second
let connectionAttemptTimeout = null;

// Initialize native messaging with retry logic
function initNativeMessaging() {
  attemptNativeConnection();
}

// Attempt native connection with exponential backoff
function attemptNativeConnection() {
  if (connectionRetryCount >= maxRetries) {
    console.warn('Max native messaging connection attempts reached');
    return;
  }

  try {
    console.log(`Attempting native connection (attempt ${connectionRetryCount + 1}/${maxRetries})`);
    nativePort = chrome.runtime.connectNative('com.github.tmc.chrome_ai_bridge');
    
    nativePort.onMessage.addListener((message) => {
      console.log('Native message received:', message);
      handleNativeMessage(message);
    });
    
    nativePort.onDisconnect.addListener(() => {
      const error = chrome.runtime.lastError;
      console.log('Native host disconnected:', error);
      nativePort = null;
      
      // Attempt reconnection if not at max retries
      if (connectionRetryCount < maxRetries) {
        scheduleReconnection();
      }
    });
    
    // Send ping to test connection
    nativePort.postMessage({ type: 'ping', id: 'init_ping' });
    console.log('Native messaging connection established');
    
    // Reset retry count on successful connection
    connectionRetryCount = 0;
    
  } catch (error) {
    console.log('Native messaging connection failed:', error);
    scheduleReconnection();
  }
}

// Schedule reconnection with exponential backoff
function scheduleReconnection() {
  if (connectionAttemptTimeout) {
    clearTimeout(connectionAttemptTimeout);
  }
  
  connectionRetryCount++;
  
  if (connectionRetryCount < maxRetries) {
    const delay = baseRetryDelay * Math.pow(2, connectionRetryCount - 1);
    const jitter = Math.random() * 0.1 * delay; // Add 10% jitter
    const totalDelay = delay + jitter;
    
    console.log(`Scheduling reconnection in ${Math.round(totalDelay)}ms (attempt ${connectionRetryCount + 1}/${maxRetries})`);
    
    connectionAttemptTimeout = setTimeout(() => {
      attemptNativeConnection();
    }, totalDelay);
  } else {
    console.error('Native messaging connection failed after all retries');
  }
}

// Force reconnection (for manual retry)
function forceReconnection() {
  connectionRetryCount = 0;
  if (connectionAttemptTimeout) {
    clearTimeout(connectionAttemptTimeout);
  }
  attemptNativeConnection();
}

// Handle messages from native host
function handleNativeMessage(message) {
  switch (message.type) {
    case 'pong':
      console.log('Native host is responsive');
      break;
    case 'ai_response':
      // Forward AI responses to waiting popup
      handleAIResponse(message);
      break;
    case 'error':
      console.error('Native host error:', message.error);
      break;
  }
}

// Handle AI responses from native host
let pendingAIRequests = new Map();

function handleAIResponse(message) {
  if (message.id && pendingAIRequests.has(message.id)) {
    const callback = pendingAIRequests.get(message.id);
    pendingAIRequests.delete(message.id);
    callback(message.data);
  }
}

// Listen for messages from content scripts and popup
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  console.log('Background received message:', message);
  
  switch (message.type) {
    case 'AI_API_STATUS':
      aiAPIState = message.data;
      console.log('AI API Status updated:', aiAPIState);
      sendResponse({ success: true });
      break;
      
    case 'AI_API_REQUEST':
      // Try native messaging first, fallback to tab forwarding
      if (nativePort) {
        handleNativeAIRequest(message.data, sendResponse);
        return true; // Keep sendResponse alive for async
      } else {
        forwardToActiveTab(message.data, sendResponse);
        return true; // Keep sendResponse alive for async
      }
      
    case 'GET_AI_STATUS':
      sendResponse(aiAPIState);
      break;
      
    case 'NATIVE_STATUS':
      sendResponse({ 
        nativeAvailable: nativePort !== null,
        connectionRetries: connectionRetryCount,
        maxRetries: maxRetries,
        state: aiAPIState 
      });
      break;
      
    case 'FORCE_RECONNECT':
      forceReconnection();
      sendResponse({ success: true });
      break;
      
    case 'HEALTH_STATUS':
      if (message.checkName) {
        const status = healthManager.getStatus(message.checkName);
        sendResponse(status);
      } else {
        const overallStatus = healthManager.getOverallStatus();
        sendResponse(overallStatus);
      }
      break;
      
    case 'HEALTH_CHECK_RUN':
      if (message.checkName) {
        healthManager.executeCheck(message.checkName);
        sendResponse({ success: true });
      } else {
        sendResponse({ success: false, error: 'Check name required' });
      }
      break;
      
    case 'HEALTH_CHECK_ENABLE':
      healthManager.enableCheck(message.checkName);
      sendResponse({ success: true });
      break;
      
    case 'HEALTH_CHECK_DISABLE':
      healthManager.disableCheck(message.checkName);
      sendResponse({ success: true });
      break;
  }
});

// Handle AI requests via native messaging with retry logic
function handleNativeAIRequest(request, sendResponse) {
  const requestId = 'req_' + Date.now();
  const maxRequestRetries = 3;
  let requestRetryCount = 0;
  
  function attemptRequest() {
    if (!nativePort) {
      if (requestRetryCount < maxRequestRetries) {
        requestRetryCount++;
        console.log(`No native connection, retrying request ${requestRetryCount}/${maxRequestRetries}`);
        
        // Try to reconnect
        forceReconnection();
        
        // Wait a bit and retry
        setTimeout(() => {
          if (nativePort) {
            attemptRequest();
          } else {
            sendResponse({ error: 'Native messaging unavailable after retries' });
          }
        }, 1000);
        return;
      } else {
        sendResponse({ error: 'Native messaging unavailable' });
        return;
      }
    }
    
    try {
      // Store callback for async response
      pendingAIRequests.set(requestId, (response) => {
        if (response.error && requestRetryCount < maxRequestRetries) {
          requestRetryCount++;
          console.log(`AI request failed, retrying ${requestRetryCount}/${maxRequestRetries}:`, response.error);
          
          // Exponential backoff for request retries
          const delay = 500 * Math.pow(2, requestRetryCount - 1);
          setTimeout(attemptRequest, delay);
        } else {
          sendResponse(response);
        }
      });
      
      // Send to native host
      nativePort.postMessage({
        type: 'ai_request',
        id: requestId,
        data: request
      });
      
      // Timeout after 30 seconds
      setTimeout(() => {
        if (pendingAIRequests.has(requestId)) {
          pendingAIRequests.delete(requestId);
          
          if (requestRetryCount < maxRequestRetries) {
            requestRetryCount++;
            console.log(`AI request timeout, retrying ${requestRetryCount}/${maxRequestRetries}`);
            setTimeout(attemptRequest, 1000);
          } else {
            sendResponse({ error: 'Native request timeout after retries' });
          }
        }
      }, 30000);
      
    } catch (error) {
      console.error('Error sending native message:', error);
      if (requestRetryCount < maxRequestRetries) {
        requestRetryCount++;
        console.log(`Native message error, retrying ${requestRetryCount}/${maxRequestRetries}`);
        setTimeout(attemptRequest, 1000);
      } else {
        sendResponse({ error: 'Native messaging error: ' + error.message });
      }
    }
  }
  
  attemptRequest();
}

// Forward AI requests to active tab
async function forwardToActiveTab(request, sendResponse) {
  try {
    const [activeTab] = await chrome.tabs.query({ active: true, currentWindow: true });
    
    if (!activeTab) {
      sendResponse({ error: 'No active tab found' });
      return;
    }
    
    // Send message to content script in active tab
    chrome.tabs.sendMessage(activeTab.id, {
      type: 'EXECUTE_AI_REQUEST',
      data: request
    }, (response) => {
      if (chrome.runtime.lastError) {
        sendResponse({ error: chrome.runtime.lastError.message });
      } else {
        sendResponse(response);
      }
    });
  } catch (error) {
    sendResponse({ error: error.message });
  }
}

// Initialize when background script starts
chrome.runtime.onStartup.addListener(initNativeMessaging);
chrome.runtime.onInstalled.addListener(initNativeMessaging);

// Initialize immediately
initNativeMessaging();

// Start health monitoring
healthManager.start();

// Health check status monitoring
healthManager.onStatusChange((checkName, result) => {
  console.log(`Health check ${checkName}: ${result.status}`);
  
  // Handle critical failures
  if (result.status === 'unhealthy' && healthManager.checks.get(checkName)?.critical) {
    console.warn(`Critical health check failed: ${checkName}`);
    
    // Attempt recovery actions
    if (checkName === 'native-messaging') {
      console.log('Attempting native messaging reconnection...');
      forceReconnection();
    } else if (checkName === 'chrome-ai-api') {
      console.log('AI API health check failed, updating state');
      aiAPIState.available = false;
    }
  }
});

// Listen for tab updates to check for AI API availability
chrome.tabs.onUpdated.addListener(async (tabId, changeInfo, tab) => {
  if (changeInfo.status === 'complete' && tab.url) {
    // Inject AI detection script
    try {
      await chrome.scripting.executeScript({
        target: { tabId: tabId },
        func: checkAIAPI
      });
    } catch (error) {
      console.log('Could not inject AI detection script:', error);
    }
  }
});

// Function to inject for AI API detection
function checkAIAPI() {
  const aiStatus = {
    languageModel: typeof LanguageModel !== 'undefined',
    windowAI: typeof window.ai !== 'undefined',
    chromeAI: typeof chrome !== 'undefined' && typeof chrome.ai !== 'undefined'
  };
  
  // Send status back to background
  chrome.runtime.sendMessage({
    type: 'AI_API_STATUS',
    data: {
      available: aiStatus.languageModel || aiStatus.windowAI || aiStatus.chromeAI,
      type: aiStatus.languageModel ? 'LanguageModel' : 
            aiStatus.windowAI ? 'window.ai' : 
            aiStatus.chromeAI ? 'chrome.ai' : null,
      details: aiStatus
    }
  });
}