import React, { useState } from 'react';
import Editor from '@monaco-editor/react';
import { FaPlus, FaTrash, FaCode, FaQuestionCircle } from 'react-icons/fa';

const TemplateEditor = ({ value, onChange }) => {
  const [showHelpModal, setShowHelpModal] = useState(false);
  
  // Set up Monaco editor options
  const editorOptions = {
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    renderLineHighlight: 'all',
    lineNumbers: 'on',
    automaticLayout: true,
  };

  const helpContent = (
    <div className="help-content">
      <h3>Go Template Variables</h3>
      <p>These are some of the variables available in your templates:</p>
      <ul>
        <li><code>{'{{.File}}'}</code> - The proto file descriptor</li>
        <li><code>{'{{.Service}}'}</code> - The current service (when applicable)</li>
        <li><code>{'{{.Method}}'}</code> - The current method (when applicable)</li>
        <li><code>{'{{.Message}}'}</code> - The current message (when applicable)</li>
        <li><code>{'{{.Enum}}'}</code> - The current enum (when applicable)</li>
        <li><code>{'{{.Field}}'}</code> - The current field (when applicable)</li>
      </ul>
      <h3>Helper Functions</h3>
      <ul>
        <li><code>{'{{camelCase "some_name"}}'}</code> - Convert to camelCase</li>
        <li><code>{'{{snakeCase "someName"}}'}</code> - Convert to snake_case</li>
        <li><code>{'{{upperFirst "name"}}'}</code> - Uppercase first letter</li>
        <li><code>{'{{methodExtension .Method "some.extension"}}'}</code> - Get method extension</li>
      </ul>
      <p>For more information, see the <a href="https://github.com/tmc/misc/tree/master/protoc-gen-anything" target="_blank" rel="noopener noreferrer">documentation</a>.</p>
    </div>
  );

  return (
    <div className="editor-container">
      <div className="editor-title">
        <span>Go Template</span>
        <div className="editor-actions">
          <button onClick={() => setShowHelpModal(true)} title="Template Help">
            <FaQuestionCircle />
          </button>
        </div>
      </div>
      <div className="editor-content">
        <Editor
          height="100%"
          language="go"
          theme="vs"
          value={value}
          options={editorOptions}
          onChange={onChange}
          className="monaco-editor-container"
        />
      </div>
      
      {showHelpModal && (
        <div className="modal-backdrop">
          <div className="modal-content">
            <div className="modal-header">
              <h2>Template Help</h2>
              <button className="modal-close" onClick={() => setShowHelpModal(false)}>&times;</button>
            </div>
            <div className="modal-body">
              {helpContent}
            </div>
            <div className="modal-footer">
              <button onClick={() => setShowHelpModal(false)}>Close</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default TemplateEditor;