/**
 * Service for interacting with the WebAssembly-based protoc-gen-anything generator
 */
class WasmService {
  constructor() {
    this.loaded = false;
    this.loadPromise = null;
  }

  /**
   * Load the WebAssembly module
   * @returns {Promise<void>} Promise that resolves when the module is loaded
   */
  load() {
    if (this.loaded) {
      return Promise.resolve();
    }

    if (this.loadPromise) {
      return this.loadPromise;
    }

    this.loadPromise = new Promise((resolve, reject) => {
      // Check if WebAssembly is supported
      if (typeof WebAssembly !== 'object') {
        reject(new Error('WebAssembly is not supported in this browser'));
        return;
      }

      // Set up the Go WASM runtime
      const go = new window.Go();
      
      // Load the WASM file
      WebAssembly.instantiateStreaming(fetch('/wasm/protogen.wasm'), go.importObject)
        .then((result) => {
          // Start the WASM instance
          go.run(result.instance);
          this.loaded = true;
          resolve();
        })
        .catch((err) => {
          console.error('Failed to load WebAssembly module:', err);
          reject(err);
        });
    });

    return this.loadPromise;
  }

  /**
   * Generate output from proto files and templates
   * @param {Object} protoFiles - Map of proto file names to content
   * @param {Object} templates - Map of template file names to content
   * @param {Object} options - Generation options
   * @returns {Promise<Object>} - Map of output file names to content
   */
  async generate(protoFiles, templates, options = {}) {
    await this.load();

    // Check if the generate function is available
    if (typeof window.generateFromProto !== 'function') {
      throw new Error('WebAssembly module not properly loaded');
    }

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

    // Call the WebAssembly function
    try {
      const outputJson = window.generateFromProto(JSON.stringify(input));
      return JSON.parse(outputJson);
    } catch (err) {
      console.error('Error generating output:', err);
      throw err;
    }
  }
}

// Export a singleton instance
export default new WasmService();