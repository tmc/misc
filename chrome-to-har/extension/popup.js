// Enhanced popup script for AI API Bridge
document.addEventListener('DOMContentLoaded', async () => {
  const statusDiv = document.getElementById('status');
  const diagnosticsDiv = document.getElementById('diagnostics');
  const setupHelpDiv = document.getElementById('setup-help');
  const controlsDiv = document.getElementById('controls');
  const resultDiv = document.getElementById('result');
  
  // Check AI API status with diagnostics
  async function checkStatus() {
    try {
      statusDiv.textContent = 'Checking AI API availability...';
      statusDiv.className = 'status unavailable';
      
      // First check basic API existence
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      
      const basicCheck = await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        func: () => ({
          languageModelExists: typeof LanguageModel !== 'undefined',
          windowAiExists: typeof window.ai !== 'undefined',
          chromeAiExists: typeof chrome !== 'undefined' && typeof chrome.ai !== 'undefined',
          userAgent: navigator.userAgent,
          location: window.location.href
        })
      });
      
      const apiStatus = basicCheck[0].result;
      
      // Show diagnostics
      diagnosticsDiv.innerHTML = `
        URL: ${apiStatus.location}<br>
        LanguageModel: ${apiStatus.languageModelExists ? '✅' : '❌'}<br>
        window.ai: ${apiStatus.windowAiExists ? '✅' : '❌'}<br>
        chrome.ai: ${apiStatus.chromeAiExists ? '✅' : '❌'}
      `;
      diagnosticsDiv.style.display = 'block';
      
      if (apiStatus.languageModelExists || apiStatus.windowAiExists || apiStatus.chromeAiExists) {
        // Test API functionality
        await testAPIFunctionality(tab.id, apiStatus);
      } else {
        // No APIs found - show setup help
        showSetupHelp();
      }
      
    } catch (error) {
      statusDiv.innerHTML = `❌ Error: ${error.message}`;
      statusDiv.className = 'status unavailable';
      console.error('Status check error:', error);
    }
  }
  
  // Test API functionality
  async function testAPIFunctionality(tabId, apiStatus) {
    try {
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
      
      const result = funcTest[0].result;
      
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
        showSetupHelp();
      }
    } catch (error) {
      statusDiv.innerHTML = `❌ Functionality test failed: ${error.message}`;
      statusDiv.className = 'status unavailable';
      showSetupHelp();
    }
  }
  
  // Show setup help
  function showSetupHelp() {
    setupHelpDiv.style.display = 'block';
    controlsDiv.style.display = 'none';
  }
  
  // Test text generation
  async function testGeneration() {
    resultDiv.style.display = 'block';
    resultDiv.textContent = 'Generating text...';
    
    try {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      
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
      
      const result = genResult[0].result;
      
      if (result.success) {
        resultDiv.innerHTML = `
          <strong>🎉 Generation Success!</strong><br>
          API: ${result.api}<br>
          Response: "${result.response}"
        `;
      } else {
        resultDiv.textContent = `❌ Generation failed: ${result.error}`;
      }
    } catch (error) {
      resultDiv.textContent = `❌ Generation error: ${error.message}`;
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
  
  // Event listeners
  document.getElementById('checkStatus').addEventListener('click', checkStatus);
  document.getElementById('testGenerate').addEventListener('click', testGeneration);
  document.getElementById('testAvailability').addEventListener('click', testAvailability);
  
  // Initial status check
  await checkStatus();
});