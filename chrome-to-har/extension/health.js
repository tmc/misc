// Health check system for Chrome AI Extension
class HealthCheckManager {
  constructor() {
    this.checks = new Map();
    this.intervals = new Map();
    this.enabled = true;
    this.statusCallbacks = new Set();
    this.overallStatus = 'unknown';
    
    // Default configuration
    this.config = {
      defaultInterval: 30000, // 30 seconds
      defaultTimeout: 5000,   // 5 seconds
      retryAttempts: 3,
      retryDelay: 1000
    };
    
    this.initializeDefaultChecks();
  }
  
  // Initialize default health checks
  initializeDefaultChecks() {
    this.registerCheck('chrome-ai-api', this.checkChromeAIAPI.bind(this), {
      interval: 30000,
      timeout: 5000,
      critical: true,
      description: 'Chrome AI API availability'
    });
    
    this.registerCheck('native-messaging', this.checkNativeMessaging.bind(this), {
      interval: 60000,
      timeout: 10000,
      critical: false,
      description: 'Native messaging connectivity'
    });
    
    this.registerCheck('extension-context', this.checkExtensionContext.bind(this), {
      interval: 15000,
      timeout: 2000,
      critical: true,
      description: 'Extension context validity'
    });
    
    this.registerCheck('memory-usage', this.checkMemoryUsage.bind(this), {
      interval: 45000,
      timeout: 3000,
      critical: false,
      description: 'Memory usage monitoring'
    });
    
    this.registerCheck('api-response-time', this.checkAPIResponseTime.bind(this), {
      interval: 120000,
      timeout: 15000,
      critical: false,
      description: 'AI API response time'
    });
  }
  
  // Register a health check
  registerCheck(name, checkFunction, options = {}) {
    const check = {
      name,
      checkFunction,
      interval: options.interval || this.config.defaultInterval,
      timeout: options.timeout || this.config.defaultTimeout,
      critical: options.critical !== undefined ? options.critical : true,
      description: options.description || name,
      enabled: true,
      lastResult: null,
      lastCheck: null,
      consecutiveFailures: 0,
      totalRuns: 0,
      totalFailures: 0
    };
    
    this.checks.set(name, check);
    console.log(`Health check registered: ${name}`);\n  }
  
  // Enable/disable a health check
  enableCheck(name) {
    const check = this.checks.get(name);
    if (check) {
      check.enabled = true;
      if (this.enabled) {
        this.startCheck(name);
      }
    }
  }
  
  disableCheck(name) {
    const check = this.checks.get(name);
    if (check) {
      check.enabled = false;
      this.stopCheck(name);
    }
  }
  
  // Start all health checks
  start() {
    this.enabled = true;
    for (const [name, check] of this.checks) {
      if (check.enabled) {
        this.startCheck(name);
      }
    }
    console.log('Health check manager started');
  }
  
  // Stop all health checks
  stop() {
    this.enabled = false;
    for (const name of this.checks.keys()) {
      this.stopCheck(name);
    }
    console.log('Health check manager stopped');
  }
  
  // Start a specific health check
  startCheck(name) {
    const check = this.checks.get(name);
    if (!check || !check.enabled) return;
    
    // Clear existing interval
    this.stopCheck(name);
    
    // Run immediately
    this.executeCheck(name);
    
    // Set up interval
    const intervalId = setInterval(() => {
      this.executeCheck(name);
    }, check.interval);
    
    this.intervals.set(name, intervalId);
  }
  
  // Stop a specific health check
  stopCheck(name) {
    const intervalId = this.intervals.get(name);
    if (intervalId) {
      clearInterval(intervalId);
      this.intervals.delete(name);
    }
  }
  
  // Execute a health check
  async executeCheck(name) {
    const check = this.checks.get(name);
    if (!check || !check.enabled) return;
    
    const startTime = Date.now();
    let result;
    
    try {\n      // Create timeout promise
      const timeoutPromise = new Promise((_, reject) => {
        setTimeout(() => reject(new Error('Health check timeout')), check.timeout);
      });
      
      // Race between check function and timeout
      const checkPromise = check.checkFunction();
      result = await Promise.race([checkPromise, timeoutPromise]);
      
      // Ensure result has required properties
      result = {
        status: result.status || 'unknown',
        message: result.message || '',
        details: result.details || {},
        timestamp: Date.now(),
        duration: Date.now() - startTime,
        error: null
      };
      
      check.consecutiveFailures = 0;
      
    } catch (error) {
      result = {
        status: 'unhealthy',
        message: error.message || 'Check failed',
        details: { error: error.toString() },
        timestamp: Date.now(),
        duration: Date.now() - startTime,
        error: error
      };
      
      check.consecutiveFailures++;
      check.totalFailures++;
    }
    
    check.lastResult = result;
    check.lastCheck = Date.now();
    check.totalRuns++;
    
    // Update overall status
    this.updateOverallStatus();
    
    // Notify listeners
    this.notifyStatusChange(name, result);
    
    console.log(`Health check ${name}: ${result.status} (${result.duration}ms)`);
  }
  
  // Update overall health status
  updateOverallStatus() {
    let overallStatus = 'healthy';
    let criticalIssues = 0;
    let degradedServices = 0;
    
    for (const [name, check] of this.checks) {
      if (!check.enabled || !check.lastResult) continue;
      
      const status = check.lastResult.status;
      
      if (check.critical) {
        if (status === 'unhealthy') {
          criticalIssues++;
        } else if (status === 'degraded') {
          degradedServices++;
        }
      }
    }
    
    if (criticalIssues > 0) {
      overallStatus = 'unhealthy';
    } else if (degradedServices > 0) {
      overallStatus = 'degraded';
    }
    
    if (this.overallStatus !== overallStatus) {
      this.overallStatus = overallStatus;
      this.notifyOverallStatusChange(overallStatus);
    }
  }
  
  // Get current status of a specific check
  getStatus(name) {
    const check = this.checks.get(name);
    if (!check) return null;
    
    return {
      name: check.name,
      description: check.description,
      enabled: check.enabled,
      critical: check.critical,
      lastResult: check.lastResult,
      lastCheck: check.lastCheck,
      consecutiveFailures: check.consecutiveFailures,
      totalRuns: check.totalRuns,
      totalFailures: check.totalFailures,
      successRate: check.totalRuns > 0 ? ((check.totalRuns - check.totalFailures) / check.totalRuns * 100).toFixed(1) : 0
    };
  }
  
  // Get all health check statuses
  getAllStatuses() {
    const statuses = {};
    for (const name of this.checks.keys()) {
      statuses[name] = this.getStatus(name);
    }
    return statuses;
  }
  
  // Get overall health status
  getOverallStatus() {
    return {
      status: this.overallStatus,
      timestamp: Date.now(),
      checks: this.getAllStatuses()
    };
  }
  
  // Subscribe to status changes
  onStatusChange(callback) {
    this.statusCallbacks.add(callback);
    return () => this.statusCallbacks.delete(callback);
  }
  
  // Notify status change
  notifyStatusChange(checkName, result) {
    for (const callback of this.statusCallbacks) {
      try {
        callback(checkName, result);
      } catch (error) {
        console.error('Error in status change callback:', error);
      }
    }
  }
  
  // Notify overall status change
  notifyOverallStatusChange(status) {
    for (const callback of this.statusCallbacks) {
      try {
        callback('overall', { status, timestamp: Date.now() });
      } catch (error) {
        console.error('Error in overall status change callback:', error);
      }
    }
  }
  
  // Health check implementations
  
  async checkChromeAIAPI() {
    try {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      
      if (!tab) {
        return {
          status: 'unhealthy',
          message: 'No active tab available',
          details: { reason: 'no_active_tab' }
        };
      }
      
      const result = await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        func: () => {
          const apis = {
            languageModel: typeof LanguageModel !== 'undefined',
            windowAI: typeof window.ai !== 'undefined',
            chromeAI: typeof chrome !== 'undefined' && typeof chrome.ai !== 'undefined'
          };
          
          const available = Object.values(apis).some(Boolean);
          
          return {
            available,
            apis,
            url: window.location.href,
            userAgent: navigator.userAgent.includes('Chrome/')
          };
        }
      });
      
      const apiInfo = result[0].result;
      
      if (apiInfo.available) {
        return {
          status: 'healthy',
          message: 'Chrome AI APIs are available',
          details: apiInfo
        };
      } else {
        return {
          status: 'unhealthy',
          message: 'No Chrome AI APIs detected',
          details: apiInfo
        };
      }
    } catch (error) {
      return {
        status: 'unhealthy',
        message: 'Failed to check Chrome AI APIs',
        details: { error: error.message }
      };
    }
  }
  
  async checkNativeMessaging() {
    try {
      // Check if native messaging is available
      if (!chrome.runtime.connectNative) {
        return {
          status: 'unhealthy',
          message: 'Native messaging API not available',
          details: { reason: 'api_not_available' }
        };
      }
      
      // Try to connect to native host
      const port = chrome.runtime.connectNative('com.github.tmc.chrome_ai_bridge');
      
      return new Promise((resolve) => {
        let resolved = false;
        
        const resolveOnce = (result) => {
          if (!resolved) {
            resolved = true;
            resolve(result);
          }
        };
        
        port.onMessage.addListener((message) => {
          if (message.type === 'pong') {
            resolveOnce({
              status: 'healthy',
              message: 'Native messaging is functional',
              details: { response: message }
            });
          }
        });
        
        port.onDisconnect.addListener(() => {
          const error = chrome.runtime.lastError;
          resolveOnce({
            status: 'unhealthy',
            message: 'Native messaging connection failed',
            details: { error: error ? error.message : 'Unknown error' }
          });
        });
        
        // Send ping
        port.postMessage({ type: 'ping', id: 'health_check' });
        
        // Timeout after 3 seconds
        setTimeout(() => {
          resolveOnce({
            status: 'degraded',
            message: 'Native messaging response timeout',
            details: { reason: 'timeout' }
          });
        }, 3000);
      });
    } catch (error) {
      return {
        status: 'unhealthy',
        message: 'Native messaging check failed',
        details: { error: error.message }
      };
    }
  }
  
  async checkExtensionContext() {
    try {
      // Check if extension context is valid
      const manifest = chrome.runtime.getManifest();
      
      if (!manifest) {
        return {
          status: 'unhealthy',
          message: 'Extension manifest not accessible',
          details: { reason: 'invalid_context' }
        };
      }
      
      // Check if we can access tabs
      const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
      
      if (!tabs || tabs.length === 0) {
        return {
          status: 'degraded',
          message: 'No active tabs accessible',
          details: { reason: 'no_tabs' }
        };
      }
      
      return {
        status: 'healthy',
        message: 'Extension context is valid',
        details: {
          manifest_version: manifest.manifest_version,
          version: manifest.version,
          tabs_count: tabs.length
        }
      };
    } catch (error) {
      return {
        status: 'unhealthy',
        message: 'Extension context check failed',
        details: { error: error.message }
      };
    }
  }
  
  async checkMemoryUsage() {
    try {
      if (!chrome.system || !chrome.system.memory) {
        return {
          status: 'degraded',
          message: 'Memory API not available',
          details: { reason: 'api_not_available' }
        };
      }
      
      const memoryInfo = await chrome.system.memory.getInfo();
      const usedMemory = memoryInfo.capacity - memoryInfo.availableCapacity;
      const usagePercentage = (usedMemory / memoryInfo.capacity) * 100;
      
      let status = 'healthy';
      let message = 'Memory usage is normal';
      
      if (usagePercentage > 90) {
        status = 'unhealthy';
        message = 'Memory usage is critically high';
      } else if (usagePercentage > 80) {
        status = 'degraded';
        message = 'Memory usage is high';
      }
      
      return {
        status,
        message,
        details: {
          total_memory: `${Math.round(memoryInfo.capacity / 1024 / 1024 / 1024)}GB`,
          used_memory: `${Math.round(usedMemory / 1024 / 1024 / 1024)}GB`,
          usage_percentage: `${usagePercentage.toFixed(1)}%`
        }
      };
    } catch (error) {
      return {
        status: 'degraded',
        message: 'Memory check failed',
        details: { error: error.message }
      };
    }
  }
  
  async checkAPIResponseTime() {
    try {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      
      if (!tab) {
        return {
          status: 'unhealthy',
          message: 'No active tab for API test',
          details: { reason: 'no_active_tab' }
        };
      }
      
      const startTime = Date.now();
      
      const result = await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        func: async () => {
          const start = performance.now();
          
          try {
            if (typeof LanguageModel !== 'undefined') {
              const availability = await LanguageModel.availability();
              return {
                success: true,
                api: 'LanguageModel',
                availability,
                duration: performance.now() - start
              };
            } else if (typeof window.ai !== 'undefined') {
              const capabilities = await window.ai.capabilities();
              return {
                success: true,
                api: 'window.ai',
                capabilities,
                duration: performance.now() - start
              };
            } else {
              return {
                success: false,
                error: 'No AI API available',
                duration: performance.now() - start
              };
            }
          } catch (error) {
            return {
              success: false,
              error: error.message,
              duration: performance.now() - start
            };
          }
        }
      });
      
      const testResult = result[0].result;
      const totalTime = Date.now() - startTime;
      
      if (testResult.success) {
        let status = 'healthy';
        let message = 'API response time is good';
        
        if (totalTime > 5000) {
          status = 'unhealthy';
          message = 'API response time is too slow';
        } else if (totalTime > 2000) {
          status = 'degraded';
          message = 'API response time is slow';
        }
        
        return {
          status,
          message,
          details: {
            api: testResult.api,
            response_time: `${totalTime}ms`,
            script_time: `${Math.round(testResult.duration)}ms`
          }
        };
      } else {
        return {
          status: 'unhealthy',
          message: 'API response test failed',
          details: {
            error: testResult.error,
            duration: `${totalTime}ms`
          }
        };
      }
    } catch (error) {
      return {
        status: 'unhealthy',
        message: 'API response time check failed',
        details: { error: error.message }
      };
    }
  }
}

// Create global health check manager
const healthManager = new HealthCheckManager();

// Export for use in other scripts
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { HealthCheckManager, healthManager };
} else {
  window.healthManager = healthManager;
}