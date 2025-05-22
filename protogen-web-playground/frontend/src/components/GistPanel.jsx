import React, { useState, useEffect } from 'react';
import { FaGithub, FaClone, FaTrash, FaPlus, FaSave, FaFolder, FaFileExport, FaFileImport } from 'react-icons/fa';
import GithubService from '../services/GithubService';

/**
 * Component for managing GitHub Gists
 * @param {Object} props - Component props
 * @param {Function} props.onLoadGist - Callback when a Gist is loaded
 * @param {Function} props.onSaveGist - Callback when a Gist is saved
 * @param {Object} props.currentConfig - Current playground configuration
 * @param {boolean} props.isAuthenticated - Whether the user is authenticated
 */
const GistPanel = ({ onLoadGist, onSaveGist, currentConfig, isAuthenticated }) => {
  const [userGists, setUserGists] = useState([]);
  const [isLoadingGists, setIsLoadingGists] = useState(false);
  const [gistError, setGistError] = useState(null);
  const [loadGistId, setLoadGistId] = useState('');
  const [showLoadForm, setShowLoadForm] = useState(false);
  const [showSaveForm, setShowSaveForm] = useState(false);
  const [saveGistDescription, setSaveGistDescription] = useState('');
  const [saveGistPublic, setSaveGistPublic] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [selectedGistId, setSelectedGistId] = useState(null);

  // Load user's Gists when authenticated
  useEffect(() => {
    if (isAuthenticated) {
      loadUserGists();
    } else {
      setUserGists([]);
    }
  }, [isAuthenticated]);

  // Load user's Gists
  const loadUserGists = async () => {
    try {
      setIsLoadingGists(true);
      setGistError(null);
      const gists = await GithubService.getUserGists();
      
      // Filter to only show protoc-gen-anything Gists
      const protocGists = gists.filter(gist => {
        return gist.files['playground.json'] || 
          Object.keys(gist.files).some(name => name.endsWith('.proto')) ||
          Object.keys(gist.files).some(name => name.endsWith('.tmpl'));
      });
      
      setUserGists(protocGists);
      setIsLoadingGists(false);
    } catch (err) {
      console.error('Error loading Gists:', err);
      setGistError('Error loading Gists: ' + err.message);
      setIsLoadingGists(false);
    }
  };

  // Handle loading a Gist by ID
  const handleLoadGist = async (e) => {
    e.preventDefault();
    if (!loadGistId.trim()) {
      setGistError('Gist ID is required');
      return;
    }
    
    try {
      setIsLoading(true);
      setGistError(null);
      const config = await GithubService.getGist(loadGistId);
      setIsLoading(false);
      setShowLoadForm(false);
      setLoadGistId('');
      if (onLoadGist) {
        onLoadGist(config, loadGistId);
      }
    } catch (err) {
      console.error('Error loading Gist:', err);
      setGistError('Error loading Gist: ' + err.message);
      setIsLoading(false);
    }
  };

  // Handle saving a Gist
  const handleSaveGist = async (e) => {
    e.preventDefault();
    if (!saveGistDescription.trim()) {
      setGistError('Description is required');
      return;
    }
    
    try {
      setIsSaving(true);
      setGistError(null);
      
      let gistData;
      if (selectedGistId) {
        // Update existing Gist
        gistData = await GithubService.updateGist(
          selectedGistId,
          currentConfig,
          saveGistDescription
        );
      } else {
        // Create new Gist
        gistData = await GithubService.createGist(
          currentConfig,
          saveGistDescription,
          saveGistPublic
        );
      }
      
      setIsSaving(false);
      setShowSaveForm(false);
      setSaveGistDescription('');
      setSelectedGistId(null);
      
      // Refresh the Gist list
      loadUserGists();
      
      if (onSaveGist) {
        onSaveGist(gistData);
      }
    } catch (err) {
      console.error('Error saving Gist:', err);
      setGistError('Error saving Gist: ' + err.message);
      setIsSaving(false);
    }
  };

  // Handle selecting a Gist to load
  const handleSelectGist = async (gistId) => {
    try {
      setIsLoading(true);
      setGistError(null);
      const config = await GithubService.getGist(gistId);
      setIsLoading(false);
      if (onLoadGist) {
        onLoadGist(config, gistId);
      }
    } catch (err) {
      console.error('Error loading Gist:', err);
      setGistError('Error loading Gist: ' + err.message);
      setIsLoading(false);
    }
  };

  // Handle selecting a Gist to update
  const handleSelectGistForUpdate = (gist) => {
    setSelectedGistId(gist.id);
    setSaveGistDescription(gist.description || '');
    setShowSaveForm(true);
  };

  // Handle forking a Gist
  const handleForkGist = async (gistId) => {
    if (!isAuthenticated) {
      setGistError('You must be logged in to fork a Gist');
      return;
    }
    
    try {
      setIsLoading(true);
      setGistError(null);
      await GithubService.forkGist(gistId);
      // Refresh the Gist list
      await loadUserGists();
      setIsLoading(false);
    } catch (err) {
      console.error('Error forking Gist:', err);
      setGistError('Error forking Gist: ' + err.message);
      setIsLoading(false);
    }
  };

  // Handle deleting a Gist
  const handleDeleteGist = async (gistId) => {
    if (!window.confirm('Are you sure you want to delete this Gist?')) {
      return;
    }
    
    try {
      setIsLoading(true);
      setGistError(null);
      await GithubService.deleteGist(gistId);
      // Remove from list
      setUserGists(userGists.filter(gist => gist.id !== gistId));
      setIsLoading(false);
    } catch (err) {
      console.error('Error deleting Gist:', err);
      setGistError('Error deleting Gist: ' + err.message);
      setIsLoading(false);
    }
  };

  return (
    <div className="gist-panel">
      <div className="gist-panel-header">
        <h3>GitHub Gists</h3>
        <div className="gist-panel-actions">
          <button 
            onClick={() => setShowLoadForm(true)} 
            title="Load Gist by ID"
            className="action-button"
          >
            <FaFolder /> Load
          </button>
          
          {isAuthenticated && (
            <button 
              onClick={() => {
                setShowSaveForm(true);
                setSelectedGistId(null);
                setSaveGistDescription('');
              }} 
              title="Save as new Gist"
              className="action-button"
            >
              <FaSave /> Save
            </button>
          )}
          
          {isAuthenticated && (
            <button 
              onClick={loadUserGists} 
              title="Refresh Gists"
              className="action-button"
              disabled={isLoadingGists}
            >
              {isLoadingGists ? 'Loading...' : 'Refresh'}
            </button>
          )}
        </div>
      </div>
      
      {gistError && (
        <div className="gist-error">
          {gistError}
        </div>
      )}
      
      {showLoadForm && (
        <div className="gist-form">
          <h4>Load Gist</h4>
          <form onSubmit={handleLoadGist}>
            <input
              type="text"
              value={loadGistId}
              onChange={(e) => setLoadGistId(e.target.value)}
              placeholder="Enter Gist ID"
              autoFocus
              className="gist-input"
            />
            <div className="gist-form-actions">
              <button 
                type="submit" 
                disabled={isLoading} 
                className="gist-form-button"
              >
                {isLoading ? 'Loading...' : 'Load'}
              </button>
              <button 
                type="button" 
                onClick={() => setShowLoadForm(false)} 
                className="gist-form-button cancel"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}
      
      {showSaveForm && (
        <div className="gist-form">
          <h4>{selectedGistId ? 'Update Gist' : 'Save Gist'}</h4>
          <form onSubmit={handleSaveGist}>
            <textarea
              value={saveGistDescription}
              onChange={(e) => setSaveGistDescription(e.target.value)}
              placeholder="Enter Gist description"
              autoFocus
              className="gist-textarea"
              rows={3}
            />
            
            {!selectedGistId && (
              <label className="gist-checkbox-label">
                <input
                  type="checkbox"
                  checked={saveGistPublic}
                  onChange={(e) => setSaveGistPublic(e.target.checked)}
                  className="gist-checkbox"
                />
                Public Gist
              </label>
            )}
            
            <div className="gist-form-actions">
              <button 
                type="submit" 
                disabled={isSaving} 
                className="gist-form-button"
              >
                {isSaving ? 'Saving...' : selectedGistId ? 'Update' : 'Save'}
              </button>
              <button 
                type="button" 
                onClick={() => {
                  setShowSaveForm(false);
                  setSelectedGistId(null);
                }} 
                className="gist-form-button cancel"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}
      
      <div className="gist-list">
        {isLoadingGists ? (
          <div className="gist-loading">Loading Gists...</div>
        ) : userGists.length > 0 ? (
          userGists.map(gist => (
            <div key={gist.id} className="gist-item">
              <div className="gist-item-header">
                <div className="gist-item-title" title={gist.description || 'No description'}>
                  {(gist.description || 'No description').substring(0, 50)}
                  {(gist.description || '').length > 50 ? '...' : ''}
                </div>
                <div className="gist-item-actions">
                  <button 
                    onClick={() => handleSelectGist(gist.id)} 
                    title="Load this Gist"
                    className="gist-item-button"
                  >
                    <FaFileImport />
                  </button>
                  <button 
                    onClick={() => handleSelectGistForUpdate(gist)} 
                    title="Update this Gist"
                    className="gist-item-button"
                  >
                    <FaSave />
                  </button>
                  <button 
                    onClick={() => handleDeleteGist(gist.id)} 
                    title="Delete this Gist"
                    className="gist-item-button delete"
                  >
                    <FaTrash />
                  </button>
                </div>
              </div>
              <div className="gist-item-files">
                {Object.keys(gist.files).length} files
                {' · '}
                <a 
                  href={gist.html_url} 
                  target="_blank" 
                  rel="noopener noreferrer"
                  className="gist-item-link"
                >
                  <FaGithub /> View on GitHub
                </a>
              </div>
            </div>
          ))
        ) : (
          <div className="gist-empty">
            {isAuthenticated ? 'No playground Gists found' : 'Login to view your Gists'}
          </div>
        )}
      </div>
    </div>
  );
};

export default GistPanel;