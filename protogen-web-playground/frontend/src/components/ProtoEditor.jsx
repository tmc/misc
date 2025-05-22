import React from 'react';
import Editor from '@monaco-editor/react';
import { FaPlus, FaTrash } from 'react-icons/fa';
import { PROTO_LANGUAGE } from '../constants';

const ProtoEditor = ({ value, onChange }) => {
  // Set up Monaco editor options
  const editorOptions = {
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    renderLineHighlight: 'all',
    lineNumbers: 'on',
    automaticLayout: true,
  };

  // Register Proto language for syntax highlighting
  const beforeMount = (monaco) => {
    if (!monaco.languages.getLanguages().some(lang => lang.id === 'proto')) {
      monaco.languages.register({ id: 'proto' });
      monaco.languages.setMonarchTokensProvider('proto', PROTO_LANGUAGE.tokenizer);
    }
  };

  return (
    <div className="editor-container">
      <div className="editor-title">
        <span>Protocol Buffer Schema</span>
        <div className="editor-actions">
          {/* In a more complex version, we'd have file management functionality here */}
        </div>
      </div>
      <div className="editor-content">
        <Editor
          height="100%"
          language="proto"
          theme="vs"
          value={value}
          options={editorOptions}
          onChange={onChange}
          beforeMount={beforeMount}
          className="monaco-editor-container"
        />
      </div>
    </div>
  );
};

export default ProtoEditor;