/**
 * Service for real-time processing with WebWorkers
 */
class RealTimeService {
  constructor() {
    this.worker = null;
    this.isWorkerLoaded = false;
    this.pendingRequests = new Map();
    this.requestId = 0;
    this.listeners = new Map();
  }

  /**
   * Initialize the WebWorker
   * @returns {Promise<void>} Promise that resolves when the worker is initialized
   */
  init() {
    if (this.worker) {
      return Promise.resolve();
    }

    return new Promise((resolve, reject) => {
      try {
        // Create a new Web Worker
        this.worker = new Worker('/wasm-worker.js');
        
        // Set up message handler
        this.worker.onmessage = this.handleWorkerMessage.bind(this);
        
        // Set up error handler
        this.worker.onerror = (error) => {
          console.error('Web Worker error:', error);
          reject(error);
        };
        
        // Initialize the worker
        const requestId = this.getNextRequestId();
        
        this.pendingRequests.set(requestId, {
          resolve,
          reject,
          type: 'init'
        });
        
        this.worker.postMessage({
          type: 'init',
          id: requestId
        });
      } catch (error) {
        console.error('Failed to initialize Web Worker:', error);
        reject(error);
      }
    });
  }

  /**
   * Handle messages from the Web Worker
   * @param {MessageEvent} event - Message event from the worker
   */
  handleWorkerMessage(event) {
    const { type, id, result, error } = event.data;
    
    // Handle pending request
    if (id && this.pendingRequests.has(id)) {
      const request = this.pendingRequests.get(id);
      this.pendingRequests.delete(id);
      
      if (error) {
        request.reject(new Error(error));
      } else {
        request.resolve(result);
      }
    }
    
    // Handle worker status updates
    switch (type) {
      case 'wasm-loaded':
        this.isWorkerLoaded = true;
        this.notifyListeners('status', { loaded: true });
        break;
        
      case 'wasm-error':
        this.notifyListeners('error', { error });
        break;
        
      case 'progress':
        this.notifyListeners('progress', event.data);
        break;
    }
  }

  /**
   * Get the next request ID
   * @returns {number} Next request ID
   */
  getNextRequestId() {
    return ++this.requestId;
  }

  /**
   * Generate output from proto files and templates
   * @param {Object} protoFiles - Map of proto file names to content
   * @param {Object} templates - Map of template file names to content
   * @param {Object} options - Generation options
   * @returns {Promise<Object>} - Map of output file names to content
   */
  generate(protoFiles, templates, options = {}) {
    return this.init()
      .then(() => {
        return new Promise((resolve, reject) => {
          const requestId = this.getNextRequestId();
          
          this.pendingRequests.set(requestId, {
            resolve,
            reject,
            type: 'generate'
          });
          
          // Prepare the input
          const input = {
            protoFiles,
            templates,
            options: {
              continueOnError: options.continueOnError || false,
              verbose: options.verbose || false,
              includeImports: options.includeImports || false,
            },
          };
          
          // Send request to worker
          this.worker.postMessage({
            type: 'generate',
            id: requestId,
            data: input
          });
        });
      });
  }

  /**
   * Add a listener for worker events
   * @param {string} event - Event name
   * @param {Function} callback - Callback function
   * @returns {Function} Function to remove the listener
   */
  addListener(event, callback) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    
    this.listeners.get(event).add(callback);
    
    // Return a function to remove the listener
    return () => {
      if (this.listeners.has(event)) {
        this.listeners.get(event).delete(callback);
      }
    };
  }

  /**
   * Notify all listeners of an event
   * @param {string} event - Event name
   * @param {Object} data - Event data
   */
  notifyListeners(event, data) {
    if (this.listeners.has(event)) {
      for (const callback of this.listeners.get(event)) {
        try {
          callback(data);
        } catch (error) {
          console.error('Error in listener callback:', error);
        }
      }
    }
  }

  /**
   * Terminate the worker
   */
  terminate() {
    if (this.worker) {
      this.worker.terminate();
      this.worker = null;
      this.isWorkerLoaded = false;
    }
  }
}

// Export a singleton instance
export default new RealTimeService();