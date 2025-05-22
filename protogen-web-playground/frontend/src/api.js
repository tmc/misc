import axios from 'axios';

const API_BASE_URL = '/api';

/**
 * Generate output from proto files and templates
 * @param {Object} config - Configuration object
 * @param {Array} config.protoFiles - Array of proto file objects (name, content)
 * @param {Array} config.templates - Array of template objects (name, content)
 * @param {Object} config.options - Generation options
 * @returns {Promise<Object>} - Generation result
 */
export const generateOutput = async (config) => {
  try {
    const response = await axios.post(`${API_BASE_URL}/generate`, config);
    return response.data;
  } catch (error) {
    console.error('Error generating output:', error);
    throw error;
  }
};

/**
 * Load configuration from a GitHub Gist
 * @param {string} gistId - The GitHub Gist ID
 * @returns {Promise<Object>} - Loaded configuration
 */
export const loadFromGist = async (gistId) => {
  try {
    const response = await axios.get(`${API_BASE_URL}/github/gists/${gistId}`);
    return response.data;
  } catch (error) {
    console.error('Error loading from Gist:', error);
    throw error;
  }
};

/**
 * Save configuration to a GitHub Gist
 * @param {Object} config - Configuration to save
 * @param {string} description - Gist description
 * @param {boolean} isPublic - Whether the Gist should be public
 * @returns {Promise<Object>} - Created Gist information
 */
export const saveToGist = async (config, description, isPublic) => {
  try {
    const response = await axios.post(`${API_BASE_URL}/github/gists`, {
      config,
      description,
      public: isPublic,
    });
    return response.data;
  } catch (error) {
    console.error('Error saving to Gist:', error);
    throw error;
  }
};

/**
 * Get example templates
 * @returns {Promise<Array>} - Array of example templates
 */
export const getExampleTemplates = async () => {
  try {
    const response = await axios.get(`${API_BASE_URL}/templates/examples`);
    return response.data;
  } catch (error) {
    console.error('Error getting example templates:', error);
    throw error;
  }
};

/**
 * Validate a template
 * @param {string} template - Template to validate
 * @returns {Promise<Object>} - Validation result
 */
export const validateTemplate = async (template) => {
  try {
    const response = await axios.post(`${API_BASE_URL}/templates/validate`, { template });
    return response.data;
  } catch (error) {
    console.error('Error validating template:', error);
    throw error;
  }
};