// Enhanced background service worker for AI API Bridge
console.log('AI API Bridge background script loaded');

// Store for AI API state
let aiAPIState = {
  available: false,
  type: null, // 'LanguageModel' or 'window.ai'
  status: null
};

// Native messaging connection
let nativePort = null;

// Initialize native messaging
function initNativeMessaging() {
  try {
    nativePort = chrome.runtime.connectNative('com.github.tmc.chrome_ai_bridge');
    
    nativePort.onMessage.addListener((message) => {
      console.log('Native message received:', message);
      handleNativeMessage(message);
    });
    
    nativePort.onDisconnect.addListener(() => {
      console.log('Native host disconnected:', chrome.runtime.lastError);
      nativePort = null;
    });
    
    // Send ping to test connection
    nativePort.postMessage({ type: 'ping', id: 'init_ping' });
    console.log('Native messaging initialized');
  } catch (error) {
    console.log('Native messaging not available:', error);
  }
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
        state: aiAPIState 
      });
      break;
  }
});

// Handle AI requests via native messaging
function handleNativeAIRequest(request, sendResponse) {
  const requestId = 'req_' + Date.now();
  
  // Store callback for async response
  pendingAIRequests.set(requestId, sendResponse);
  
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
      sendResponse({ error: 'Native request timeout' });
    }
  }, 30000);
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