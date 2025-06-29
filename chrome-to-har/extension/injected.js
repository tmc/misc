// Injected script that runs in page context and can access AI APIs
console.log('AI API Bridge injected script loaded');

// AI API wrapper functions
const AIBridge = {
  // Check API availability
  async checkAvailability() {
    const status = {
      languageModel: typeof LanguageModel !== 'undefined',
      windowAI: typeof window.ai !== 'undefined',
      chromeAI: typeof chrome !== 'undefined' && typeof chrome.ai !== 'undefined'
    };
    
    let apiType = null;
    let details = null;
    
    if (status.languageModel) {
      try {
        const availability = await LanguageModel.availability();
        apiType = 'LanguageModel';
        details = { availability };
      } catch (error) {
        details = { error: error.message };
      }
    } else if (status.windowAI) {
      try {
        const capabilities = await window.ai.capabilities();
        apiType = 'window.ai';
        details = { capabilities };
      } catch (error) {
        details = { error: error.message };
      }
    } else if (status.chromeAI) {
      apiType = 'chrome.ai';
      details = { available: true };
    }
    
    return {
      available: status.languageModel || status.windowAI || status.chromeAI,
      type: apiType,
      details,
      status
    };
  },
  
  // Execute AI requests
  async executeRequest(request) {
    try {
      switch (request.action) {
        case 'check_availability':
          return await this.checkAvailability();
          
        case 'generate_text':
          return await this.generateText(request.prompt, request.options);
          
        case 'create_session':
          return await this.createSession(request.options);
          
        default:
          throw new Error(`Unknown action: ${request.action}`);
      }
    } catch (error) {
      return { error: error.message };
    }
  },
  
  // Generate text using available API
  async generateText(prompt, options = {}) {
    if (typeof LanguageModel !== 'undefined') {
      const model = await LanguageModel.create(options);
      return await model.generate(prompt);
    } else if (typeof window.ai !== 'undefined') {
      const session = await window.ai.createTextSession(options);
      return await session.prompt(prompt);
    } else {
      throw new Error('No AI API available');
    }
  },
  
  // Create session for streaming
  async createSession(options = {}) {
    if (typeof LanguageModel !== 'undefined') {
      return await LanguageModel.create(options);
    } else if (typeof window.ai !== 'undefined') {
      return await window.ai.createTextSession(options);
    } else {
      throw new Error('No AI API available');
    }
  }
};

// Listen for requests from content script
window.addEventListener('message', async (event) => {
  if (event.source === window) {
    if (event.data.type === 'CHECK_AI_API') {
      const status = await AIBridge.checkAvailability();
      window.postMessage({
        type: 'AI_API_STATUS_UPDATE',
        status
      }, '*');
    } else if (event.data.type === 'AI_BRIDGE_REQUEST') {
      const result = await AIBridge.executeRequest(event.data.data);
      window.postMessage({
        type: 'AI_BRIDGE_RESPONSE',
        requestId: event.data.requestId,
        result
      }, '*');
    }
  }
});

// Auto-check on load
window.addEventListener('load', async () => {
  const status = await AIBridge.checkAvailability();
  window.postMessage({
    type: 'AI_API_STATUS_UPDATE',
    status
  }, '*');
});