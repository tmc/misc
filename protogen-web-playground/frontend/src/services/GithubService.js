/**
 * Service for interacting with GitHub Gists
 */
class GithubService {
  constructor() {
    this.apiBase = 'https://api.github.com';
    this.token = localStorage.getItem('github_token');
  }

  /**
   * Set the GitHub access token
   * @param {string} token - GitHub access token
   */
  setToken(token) {
    this.token = token;
    if (token) {
      localStorage.setItem('github_token', token);
    } else {
      localStorage.removeItem('github_token');
    }
  }

  /**
   * Get the GitHub access token
   * @returns {string} GitHub access token
   */
  getToken() {
    return this.token;
  }

  /**
   * Check if the user is authenticated
   * @returns {boolean} True if authenticated
   */
  isAuthenticated() {
    return !!this.token;
  }

  /**
   * Create headers for GitHub API requests
   * @param {boolean} includeContentType - Whether to include Content-Type header
   * @returns {Object} Headers object
   */
  createHeaders(includeContentType = true) {
    const headers = {};
    
    if (this.token) {
      headers.Authorization = `token ${this.token}`;
    }
    
    if (includeContentType) {
      headers['Content-Type'] = 'application/json';
    }
    
    headers.Accept = 'application/vnd.github.v3+json';
    
    return headers;
  }

  /**
   * Get user information
   * @returns {Promise<Object>} User information
   */
  async getUser() {
    if (!this.token) {
      throw new Error('Authentication required');
    }
    
    const response = await fetch(`${this.apiBase}/user`, {
      method: 'GET',
      headers: this.createHeaders(false),
    });
    
    if (!response.ok) {
      throw new Error(`GitHub API error: ${response.status} ${response.statusText}`);
    }
    
    return response.json();
  }

  /**
   * Get a Gist by ID
   * @param {string} gistId - Gist ID
   * @returns {Promise<Object>} Gist data
   */
  async getGist(gistId) {
    const response = await fetch(`${this.apiBase}/gists/${gistId}`, {
      method: 'GET',
      headers: this.createHeaders(false),
    });
    
    if (!response.ok) {
      throw new Error(`GitHub API error: ${response.status} ${response.statusText}`);
    }
    
    const gistData = await response.json();
    
    // Parse and extract playground configuration
    return this.parseGistToPlaygroundConfig(gistData);
  }

  /**
   * Create a new Gist
   * @param {Object} config - Playground configuration
   * @param {string} description - Gist description
   * @param {boolean} isPublic - Whether the Gist should be public
   * @returns {Promise<Object>} Created Gist information
   */
  async createGist(config, description, isPublic = true) {
    if (!this.token) {
      throw new Error('Authentication required');
    }
    
    const files = this.createGistFiles(config);
    
    const response = await fetch(`${this.apiBase}/gists`, {
      method: 'POST',
      headers: this.createHeaders(),
      body: JSON.stringify({
        description,
        public: isPublic,
        files,
      }),
    });
    
    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`GitHub API error: ${response.status} ${response.statusText}\n${errorText}`);
    }
    
    return response.json();
  }

  /**
   * Update an existing Gist
   * @param {string} gistId - Gist ID
   * @param {Object} config - Playground configuration
   * @param {string} description - Gist description
   * @returns {Promise<Object>} Updated Gist information
   */
  async updateGist(gistId, config, description) {
    if (!this.token) {
      throw new Error('Authentication required');
    }
    
    const files = this.createGistFiles(config);
    
    const response = await fetch(`${this.apiBase}/gists/${gistId}`, {
      method: 'PATCH',
      headers: this.createHeaders(),
      body: JSON.stringify({
        description,
        files,
      }),
    });
    
    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`GitHub API error: ${response.status} ${response.statusText}\n${errorText}`);
    }
    
    return response.json();
  }

  /**
   * Parse a Gist to playground configuration
   * @param {Object} gistData - Gist data from GitHub API
   * @returns {Object} Playground configuration
   */
  parseGistToPlaygroundConfig(gistData) {
    const config = {
      proto: {
        files: [],
      },
      templates: [],
      options: {
        continueOnError: true,
        verbose: false,
        includeImports: true,
      },
    };
    
    // Check if there's a playground.json file
    if (gistData.files['playground.json']) {
      try {
        const playgroundJson = JSON.parse(gistData.files['playground.json'].content);
        
        // Merge the parsed config
        if (playgroundJson.options) {
          config.options = { ...config.options, ...playgroundJson.options };
        }
        
        // Only use proto files and templates from playground.json if they exist
        if (playgroundJson.proto && playgroundJson.proto.files && playgroundJson.proto.files.length > 0) {
          config.proto.files = playgroundJson.proto.files;
        }
        
        if (playgroundJson.templates && playgroundJson.templates.length > 0) {
          config.templates = playgroundJson.templates;
        }
      } catch (error) {
        console.error('Error parsing playground.json:', error);
      }
    }
    
    // If we didn't find proto files or templates in playground.json, extract them from the Gist files
    if (config.proto.files.length === 0) {
      for (const [filename, file] of Object.entries(gistData.files)) {
        if (filename.endsWith('.proto')) {
          config.proto.files.push({
            name: filename,
            content: file.content,
          });
        }
      }
    }
    
    if (config.templates.length === 0) {
      for (const [filename, file] of Object.entries(gistData.files)) {
        if (filename.endsWith('.tmpl')) {
          config.templates.push({
            name: filename,
            content: file.content,
          });
        }
      }
    }
    
    return config;
  }

  /**
   * Create Gist files from playground configuration
   * @param {Object} config - Playground configuration
   * @returns {Object} Gist files object
   */
  createGistFiles(config) {
    const files = {};
    
    // Add playground.json
    files['playground.json'] = {
      content: JSON.stringify(config, null, 2),
    };
    
    // Add proto files
    if (config.proto && config.proto.files) {
      for (const file of config.proto.files) {
        files[file.name] = {
          content: file.content,
        };
      }
    }
    
    // Add template files
    if (config.templates) {
      for (const template of config.templates) {
        files[template.name] = {
          content: template.content,
        };
      }
    }
    
    // Add README.md
    const protoFilesList = config.proto.files.map(file => `- \`${file.name}\``).join('\n');
    const templatesList = config.templates.map(template => `- \`${template.name}\``).join('\n');
    
    files['README.md'] = {
      content: `# Protoc-Gen-Anything Playground Configuration

This Gist contains configuration for the Protoc-Gen-Anything Playground.

## Proto Files
${protoFilesList}

## Templates
${templatesList}

## Usage

To use this configuration, visit the playground and load it using the Gist ID:
https://protogen-playground.example.com/?gist=${GistID_PLACEHOLDER}

Or use the direct link:
[Open in Playground](https://protogen-playground.example.com/?gist=${GistID_PLACEHOLDER})
`,
    };
    
    return files;
  }

  /**
   * Get user's Gists
   * @param {number} page - Page number (1-based)
   * @param {number} perPage - Items per page
   * @returns {Promise<Array>} List of Gists
   */
  async getUserGists(page = 1, perPage = 30) {
    if (!this.token) {
      throw new Error('Authentication required');
    }
    
    const response = await fetch(`${this.apiBase}/gists?page=${page}&per_page=${perPage}`, {
      method: 'GET',
      headers: this.createHeaders(false),
    });
    
    if (!response.ok) {
      throw new Error(`GitHub API error: ${response.status} ${response.statusText}`);
    }
    
    return response.json();
  }

  /**
   * Delete a Gist
   * @param {string} gistId - Gist ID
   * @returns {Promise<boolean>} Success status
   */
  async deleteGist(gistId) {
    if (!this.token) {
      throw new Error('Authentication required');
    }
    
    const response = await fetch(`${this.apiBase}/gists/${gistId}`, {
      method: 'DELETE',
      headers: this.createHeaders(false),
    });
    
    return response.ok;
  }

  /**
   * Fork a Gist
   * @param {string} gistId - Gist ID
   * @returns {Promise<Object>} Forked Gist information
   */
  async forkGist(gistId) {
    if (!this.token) {
      throw new Error('Authentication required');
    }
    
    const response = await fetch(`${this.apiBase}/gists/${gistId}/forks`, {
      method: 'POST',
      headers: this.createHeaders(),
    });
    
    if (!response.ok) {
      throw new Error(`GitHub API error: ${response.status} ${response.statusText}`);
    }
    
    return response.json();
  }
}

// Export a singleton instance
export default new GithubService();