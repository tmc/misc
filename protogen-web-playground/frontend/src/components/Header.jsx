import React, { useState } from 'react';
import { FaGithub, FaCog, FaSave, FaFolder, FaShareAlt, FaCode } from 'react-icons/fa';
import GithubAuth from './GithubAuth';
import GistPanel from './GistPanel';

const Header = ({ 
  onLoadGist, 
  onSaveGist, 
  onSettingsClick, 
  currentConfig, 
  onShare 
}) => {
  const [isGistPanelOpen, setIsGistPanelOpen] = useState(false);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState(null);
  const [showShareDialog, setShowShareDialog] = useState(false);
  const [shareUrl, setShareUrl] = useState('');

  // Handle auth state change
  const handleAuthChange = (authenticated, userData) => {
    setIsAuthenticated(authenticated);
    setUser(userData);
  };

  // Handle loading a Gist
  const handleLoadGist = (config, gistId) => {
    if (onLoadGist) {
      onLoadGist(config, gistId);
    }
    setIsGistPanelOpen(false);
  };

  // Handle saving a Gist
  const handleSaveGist = (gistData) => {
    if (onSaveGist) {
      onSaveGist(gistData);
    }
    setIsGistPanelOpen(false);
  };

  // Handle sharing the current configuration
  const handleShareClick = () => {
    // Generate a URL that includes the current state
    if (onShare) {
      const url = onShare();
      setShareUrl(url);
      setShowShareDialog(true);
    }
  };

  // Copy share URL to clipboard
  const copyShareUrl = () => {
    navigator.clipboard.writeText(shareUrl).then(() => {
      // Show a temporary success message
      const copyButton = document.getElementById('copy-url-button');
      const originalText = copyButton.textContent;
      copyButton.textContent = 'Copied!';
      setTimeout(() => {
        copyButton.textContent = originalText;
      }, 2000);
    });
  };

  return (
    <div className="header">
      <div className="logo">
        <FaCode className="logo-icon" />
        <h1>Protoc-Gen-Anything Playground</h1>
      </div>
      <div className="actions">
        <button 
          onClick={() => setIsGistPanelOpen(!isGistPanelOpen)} 
          title="GitHub Gists"
          className={`header-button ${isGistPanelOpen ? 'active' : ''}`}
        >
          <FaGithub /> Gists
        </button>
        <button 
          onClick={handleShareClick} 
          title="Share Playground"
          className="header-button"
        >
          <FaShareAlt /> Share
        </button>
        <button 
          onClick={onSettingsClick} 
          title="Settings"
          className="header-button"
        >
          <FaCog /> Settings
        </button>
        <div className="auth-container">
          <GithubAuth onAuthChange={handleAuthChange} />
        </div>
      </div>

      {isGistPanelOpen && (
        <div className="panel-container">
          <GistPanel 
            onLoadGist={handleLoadGist}
            onSaveGist={handleSaveGist}
            currentConfig={currentConfig}
            isAuthenticated={isAuthenticated}
          />
        </div>
      )}

      {showShareDialog && (
        <div className="share-dialog">
          <div className="share-dialog-content">
            <h3>Share Playground</h3>
            <p>Share this URL to let others use your current configuration:</p>
            <div className="share-url-container">
              <input 
                type="text" 
                value={shareUrl} 
                readOnly 
                className="share-url-input"
                onClick={(e) => e.target.select()}
              />
              <button 
                id="copy-url-button"
                onClick={copyShareUrl}
                className="copy-url-button"
              >
                Copy
              </button>
            </div>
            <div className="share-dialog-actions">
              <button onClick={() => setShowShareDialog(false)} className="close-button">
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default Header;