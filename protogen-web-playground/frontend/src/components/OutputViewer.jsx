import React from 'react';
import Editor from '@monaco-editor/react';
import { FaDownload, FaSync, FaClipboard, FaPlay, FaPause } from 'react-icons/fa';

const OutputViewer = ({ 
  value, 
  isLoading, 
  error, 
  wasmLoaded, 
  previewMode, 
  onPreviewModeToggle, 
  onGenerateClick,
  progressInfo 
}) => {
  // Set up Monaco editor options
  const editorOptions = {
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    renderLineHighlight: 'all',
    lineNumbers: 'on',
    readOnly: true,
    automaticLayout: true,
  };

  // Function to download the output as a file
  const handleDownload = () => {
    const blob = new Blob([value], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'generated-output.txt';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  };

  // Function to copy the output to clipboard
  const handleCopy = () => {
    navigator.clipboard.writeText(value).then(() => {
      alert('Output copied to clipboard');
    }).catch(err => {
      console.error('Failed to copy text: ', err);
    });
  };

  return (
    <div className="editor-container">
      <div className="editor-title">
        <span>
          Generated Output {wasmLoaded ? '(WebAssembly)' : ''}
          {previewMode === 'realtime' ? ' - Real-time' : ' - Manual'}
        </span>
        <div className="editor-actions">
          <button 
            onClick={onPreviewModeToggle} 
            title={previewMode === 'realtime' ? 'Switch to manual mode' : 'Switch to real-time mode'}
            className="preview-mode-button"
          >
            {previewMode === 'realtime' ? <FaPause /> : <FaPlay />}
          </button>
          
          {previewMode === 'manual' && (
            <button 
              onClick={onGenerateClick} 
              title="Generate output"
              disabled={isLoading || !wasmLoaded}
              className="generate-button"
            >
              <FaSync className={isLoading ? 'spinning' : ''} />
            </button>
          )}
          
          <button 
            onClick={handleCopy} 
            title="Copy to Clipboard" 
            disabled={isLoading || !value}
            className="action-button"
          >
            <FaClipboard />
          </button>
          
          <button 
            onClick={handleDownload} 
            title="Download Output" 
            disabled={isLoading || !value}
            className="action-button"
          >
            <FaDownload />
          </button>
        </div>
      </div>
      <div className="editor-content">
        {!wasmLoaded && !isLoading && (
          <div className="loading">
            WebAssembly module not loaded yet. Please wait...
          </div>
        )}
        
        {isLoading && (
          <div className="loading">
            {progressInfo ? (
              <div className="progress-info">
                <div className="progress-message">{progressInfo.message || 'Generating output...'}</div>
                {progressInfo.percent !== undefined && (
                  <div className="progress-bar-container">
                    <div 
                      className="progress-bar" 
                      style={{ width: `${progressInfo.percent}%` }}
                    ></div>
                    <div className="progress-percent">{progressInfo.percent}%</div>
                  </div>
                )}
              </div>
            ) : (
              'Generating output...'
            )}
          </div>
        )}
        
        {error && (
          <div className="error">
            {error}
          </div>
        )}
        
        {wasmLoaded && !isLoading && !error && (
          <Editor
            height="100%"
            language="go"
            theme="vs"
            value={value || '// Generated output will appear here'}
            options={editorOptions}
            className="monaco-editor-container"
          />
        )}
      </div>
    </div>
  );
};

export default OutputViewer;