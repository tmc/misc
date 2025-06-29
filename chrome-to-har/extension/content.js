// Content script for AI API Bridge
console.log('AI API Bridge content script loaded');

// Inject script into page context to access AI APIs
const script = document.createElement('script');
script.src = chrome.runtime.getURL('injected.js');
script.onload = function() {
  this.remove();
};
(document.head || document.documentElement).appendChild(script);

// Listen for messages from background script
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === 'EXECUTE_AI_REQUEST') {
    // Forward to injected script via page events
    window.postMessage({
      type: 'AI_BRIDGE_REQUEST',
      data: message.data,
      requestId: Math.random().toString(36)
    }, '*');
    
    // Listen for response from injected script
    const responseHandler = (event) => {
      if (event.source === window && 
          event.data.type === 'AI_BRIDGE_RESPONSE' && 
          event.data.requestId === message.data.requestId) {
        window.removeEventListener('message', responseHandler);
        sendResponse(event.data.result);
      }
    };
    
    window.addEventListener('message', responseHandler);
    return true; // Keep sendResponse alive
  }
});

// Listen for AI API status updates from injected script
window.addEventListener('message', (event) => {
  if (event.source === window && event.data.type === 'AI_API_STATUS_UPDATE') {
    chrome.runtime.sendMessage({
      type: 'AI_API_STATUS',
      data: event.data.status
    });
  }
});

// Check for AI APIs on load
window.addEventListener('load', () => {
  window.postMessage({ type: 'CHECK_AI_API' }, '*');
});