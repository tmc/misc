import React from 'react';
import { FaPlay, FaPause } from 'react-icons/fa';

const SettingsPanel = ({ 
  settings, 
  onSettingsChange, 
  previewMode, 
  onPreviewModeChange, 
  onClose 
}) => {
  const handleChange = (e) => {
    const { name, type, checked, value } = e.target;
    onSettingsChange({
      ...settings,
      [name]: type === 'checkbox' ? checked : value,
    });
  };

  const handlePreviewModeChange = (mode) => {
    if (onPreviewModeChange) {
      onPreviewModeChange(mode);
    }
  };

  return (
    <div className="settings-panel">
      <div className="settings-panel-header">
        <h3>Settings</h3>
        <button onClick={onClose} className="close-button">×</button>
      </div>
      
      <div className="settings-section">
        <h4>Preview Mode</h4>
        <div className="preview-mode-options">
          <button 
            className={`preview-option ${previewMode === 'realtime' ? 'active' : ''}`}
            onClick={() => handlePreviewModeChange('realtime')}
          >
            <FaPlay /> Real-time
          </button>
          <button 
            className={`preview-option ${previewMode === 'manual' ? 'active' : ''}`}
            onClick={() => handlePreviewModeChange('manual')}
          >
            <FaPause /> Manual
          </button>
        </div>
        <p className="preview-mode-description">
          {previewMode === 'realtime' 
            ? 'Output updates automatically as you type' 
            : 'Click Generate button to update output'}
        </p>
      </div>
      
      <div className="settings-section">
        <h4>Generation Options</h4>
        <div className="settings-content">
          <label className="setting-checkbox">
            <input
              type="checkbox"
              name="continueOnError"
              checked={settings.continueOnError}
              onChange={handleChange}
            />
            <span className="setting-label">Continue on errors</span>
          </label>
          <p className="setting-description">
            Continue generation even if errors occur in some files
          </p>
          
          <label className="setting-checkbox">
            <input
              type="checkbox"
              name="verbose"
              checked={settings.verbose}
              onChange={handleChange}
            />
            <span className="setting-label">Verbose output</span>
          </label>
          <p className="setting-description">
            Show detailed logs during generation
          </p>
          
          <label className="setting-checkbox">
            <input
              type="checkbox"
              name="includeImports"
              checked={settings.includeImports}
              onChange={handleChange}
            />
            <span className="setting-label">Include imports</span>
          </label>
          <p className="setting-description">
            Process imported proto files in addition to the main files
          </p>
        </div>
      </div>
      
      <div className="settings-section">
        <h4>WebAssembly Info</h4>
        <div className="wasm-info">
          <p>
            This playground runs entirely in your browser using WebAssembly.
            Your proto files and templates never leave your computer.
          </p>
          <ul>
            <li>Fast generation without server round-trips</li>
            <li>Works offline after initial load</li>
            <li>Enhanced privacy and security</li>
          </ul>
        </div>
      </div>
    </div>
  );
};

export default SettingsPanel;