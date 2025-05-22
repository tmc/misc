import React, { useState, useEffect } from 'react';
import SplitPane from 'react-split-pane';
import { BrowserRouter as Router, Routes, Route, useNavigate, useLocation } from 'react-router-dom';
import Header from './components/Header.jsx';
import ProtoEditor from './components/ProtoEditor.jsx';
import TemplateEditor from './components/TemplateEditor.jsx';
import OutputViewer from './components/OutputViewer.jsx';
import SettingsPanel from './components/SettingsPanel.jsx';
import { useDebounce } from './hooks/useDebounce';
import { DEFAULT_PROTO, DEFAULT_TEMPLATE } from './constants';
import WasmService from './services/WasmService';
import GithubService from './services/GithubService';
import RealTimeService from './services/RealTimeService';
import './App.css';

function AppContent() {
  const navigate = useNavigate();
  const location = useLocation();
  
  const [proto, setProto] = useState(DEFAULT_PROTO);
  const [template, setTemplate] = useState(DEFAULT_TEMPLATE);
  const [output, setOutput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState(null);
  const [settings, setSettings] = useState({
    continueOnError: true,
    verbose: false,
    includeImports: true,
  });
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [wasmLoaded, setWasmLoaded] = useState(false);
  const [currentGistId, setCurrentGistId] = useState(null);
  const [previewMode, setPreviewMode] = useState('realtime'); // 'realtime' or 'manual'
  const [progressInfo, setProgressInfo] = useState(null);

  // Debounce input changes to prevent excessive generation
  const debouncedProto = useDebounce(proto, previewMode === 'realtime' ? 1000 : null);
  const debouncedTemplate = useDebounce(template, previewMode === 'realtime' ? 1000 : null);

  // Parse URL parameters on component mount
  useEffect(() => {
    const queryParams = new URLSearchParams(location.search);
    const gistId = queryParams.get('gist');
    
    if (gistId) {
      handleLoadGist(null, gistId);
    }
    
    // Check for compressed state in the URL hash
    if (location.hash && location.hash.length > 1) {
      try {
        const compressedState = location.hash.substring(1);
        const jsonState = JSON.parse(atob(compressedState));
        
        if (jsonState.proto) setProto(jsonState.proto);
        if (jsonState.template) setTemplate(jsonState.template);
        if (jsonState.settings) setSettings(jsonState.settings);
      } catch (err) {
        console.error('Error parsing state from URL:', err);
      }
    }
  }, [location]);

  // Initialize RealTimeService and listen for events
  useEffect(() => {
    // Initialize the real-time service
    RealTimeService.init()
      .then(() => {
        setWasmLoaded(true);
      })
      .catch((err) => {
        console.error('Failed to initialize real-time service:', err);
        setError('Failed to initialize real-time service: ' + err.message);
      });
    
    // Listen for status updates
    const statusListener = RealTimeService.addListener('status', (data) => {
      setWasmLoaded(data.loaded);
    });
    
    // Listen for errors
    const errorListener = RealTimeService.addListener('error', (data) => {
      setError(data.error);
    });
    
    // Listen for progress updates
    const progressListener = RealTimeService.addListener('progress', (data) => {
      setProgressInfo(data);
    });
    
    // Clean up listeners
    return () => {
      statusListener();
      errorListener();
      progressListener();
    };
  }, []);

  // Generate output whenever proto or template changes in real-time mode
  useEffect(() => {
    if (wasmLoaded && previewMode === 'realtime' && debouncedProto && debouncedTemplate) {
      generateOutputFromInputs();
    }
  }, [wasmLoaded, debouncedProto, debouncedTemplate, settings, previewMode]);

  // Get current configuration
  const getCurrentConfig = () => {
    return {
      proto: {
        files: [
          {
            name: 'example.proto',
            content: proto,
          },
        ],
      },
      templates: [
        {
          name: '{{.Message.GoIdent.GoName}}_extension.go.tmpl',
          content: template,
        },
      ],
      options: settings,
    };
  };

  // Function to generate output using WebWorker
  const generateOutputFromInputs = async () => {
    setIsLoading(true);
    setError(null);
    setProgressInfo(null);
    
    try {
      const protoFiles = {
        'example.proto': previewMode === 'realtime' ? debouncedProto : proto
      };
      
      const templates = {
        '{{.Message.GoIdent.GoName}}_extension.go.tmpl': previewMode === 'realtime' ? debouncedTemplate : template
      };
      
      const result = await RealTimeService.generate(protoFiles, templates, settings);
      
      // Combine all output files into a single string
      const combinedOutput = Object.entries(result)
        .map(([filename, content]) => `// ${filename}\n${content}`)
        .join('\n\n');
      
      setOutput(combinedOutput || '// No output generated');
      setIsLoading(false);
    } catch (err) {
      console.error('Error generating output:', err);
      setError('Error generating output: ' + err.message);
      setIsLoading(false);
    }
  };

  // Function to load configuration from a GitHub Gist
  const handleLoadGist = async (config, gistId) => {
    if (!gistId) return;
    
    setIsLoading(true);
    setError(null);
    
    try {
      // If config is not provided, fetch it from GitHub
      let gistConfig = config;
      if (!gistConfig) {
        gistConfig = await GithubService.getGist(gistId);
      }
      
      // Update state with the loaded configuration
      if (gistConfig.proto && gistConfig.proto.files && gistConfig.proto.files.length > 0) {
        setProto(gistConfig.proto.files[0].content);
      }
      
      if (gistConfig.templates && gistConfig.templates.length > 0) {
        setTemplate(gistConfig.templates[0].content);
      }
      
      if (gistConfig.options) {
        setSettings(gistConfig.options);
      }
      
      setCurrentGistId(gistId);
      
      // Update URL to include the Gist ID
      navigate(`?gist=${gistId}`, { replace: true });
      
      setIsLoading(false);
    } catch (err) {
      setError('Error loading from Gist: ' + err.message);
      setIsLoading(false);
    }
  };

  // Function to save configuration to a GitHub Gist
  const handleSaveGist = async (gistData) => {
    // Update current Gist ID
    if (gistData && gistData.id) {
      setCurrentGistId(gistData.id);
      
      // Update URL to include the Gist ID
      navigate(`?gist=${gistData.id}`, { replace: true });
    }
  };

  // Generate a shareable URL with the current state
  const handleShare = () => {
    const state = {
      proto,
      template,
      settings,
    };
    
    // Compress the state into a base64 encoded JSON string
    const compressed = btoa(JSON.stringify(state));
    
    // Create a new URL with the compressed state in the hash
    const url = `${window.location.origin}${window.location.pathname}#${compressed}`;
    
    return url;
  };

  // Toggle preview mode
  const togglePreviewMode = () => {
    const newMode = previewMode === 'realtime' ? 'manual' : 'realtime';
    setPreviewMode(newMode);
    
    // If switching to real-time mode, trigger generation
    if (newMode === 'realtime' && proto && template) {
      generateOutputFromInputs();
    }
  };

  return (
    <div className="App">
      <Header 
        onLoadGist={handleLoadGist}
        onSaveGist={handleSaveGist}
        onSettingsClick={() => setIsSettingsOpen(!isSettingsOpen)}
        currentConfig={getCurrentConfig()}
        onShare={handleShare}
      />
      
      <div className="playground">
        <SplitPane split="vertical" minSize={200} defaultSize="33%">
          <ProtoEditor value={proto} onChange={setProto} />
          
          <SplitPane split="vertical" minSize={200} defaultSize="50%">
            <TemplateEditor value={template} onChange={setTemplate} />
            <OutputViewer 
              value={output} 
              isLoading={isLoading} 
              error={error}
              wasmLoaded={wasmLoaded}
              previewMode={previewMode}
              onPreviewModeToggle={togglePreviewMode}
              onGenerateClick={generateOutputFromInputs}
              progressInfo={progressInfo}
            />
          </SplitPane>
        </SplitPane>
      </div>
      
      {isSettingsOpen && (
        <SettingsPanel 
          settings={settings} 
          onSettingsChange={setSettings}
          previewMode={previewMode}
          onPreviewModeChange={setPreviewMode}
          onClose={() => setIsSettingsOpen(false)}
        />
      )}
    </div>
  );
}

function App() {
  return (
    <Router>
      <Routes>
        <Route path="*" element={<AppContent />} />
      </Routes>
    </Router>
  );
}

export default App;