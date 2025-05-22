// WebWorker for handling WebAssembly processing in a separate thread

// Import the Go WASM runtime
importScripts('/wasm/wasm_exec.js');

// Store the WebAssembly instance
let wasmInstance = null;
let isWasmLoaded = false;

// Handle messages from the main thread
self.onmessage = async function(event) {
  const { type, id, data } = event.data;
  
  switch (type) {
    case 'init':
      // Initialize WebAssembly if not already loaded
      if (!isWasmLoaded) {
        await initWasm();
        self.postMessage({ type: 'init-complete', id });
      } else {
        self.postMessage({ type: 'init-complete', id });
      }
      break;
      
    case 'generate':
      // Generate output from proto files and templates
      if (!isWasmLoaded) {
        await initWasm();
      }
      
      try {
        const result = generateOutput(data);
        self.postMessage({ type: 'generate-complete', id, result });
      } catch (error) {
        self.postMessage({ type: 'error', id, error: error.message });
      }
      break;
      
    default:
      self.postMessage({ 
        type: 'error', 
        id, 
        error: `Unknown message type: ${type}` 
      });
  }
};

// Initialize WebAssembly
async function initWasm() {
  try {
    // Create a new Go instance
    const go = new Go();
    
    // Fetch the WebAssembly module
    const response = await fetch('/wasm/protogen.wasm');
    const buffer = await response.arrayBuffer();
    const result = await WebAssembly.instantiate(buffer, go.importObject);
    
    // Store the instance and run the Go program
    wasmInstance = result.instance;
    go.run(wasmInstance);
    
    isWasmLoaded = true;
    
    // Notify the main thread that WebAssembly is loaded
    self.postMessage({ type: 'wasm-loaded' });
  } catch (error) {
    self.postMessage({ type: 'wasm-error', error: error.message });
    throw error;
  }
}

// Generate output from proto files and templates
function generateOutput(data) {
  try {
    // Check if the generateFromProto function is available
    if (typeof self.generateFromProto !== 'function') {
      throw new Error('WebAssembly module not properly loaded');
    }
    
    // Convert data to JSON string for the WASM function
    const inputJSON = JSON.stringify(data);
    
    // Call the WASM function
    const outputJSON = self.generateFromProto(inputJSON);
    
    // Parse the result
    const result = JSON.parse(outputJSON);
    
    return result;
  } catch (error) {
    console.error('Error generating output:', error);
    throw error;
  }
}