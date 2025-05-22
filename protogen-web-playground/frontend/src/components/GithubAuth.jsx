import React, { useState, useEffect } from 'react';
import { FaGithub, FaSignOutAlt, FaUser } from 'react-icons/fa';
import GithubService from '../services/GithubService';

/**
 * Component for GitHub authentication
 * @param {Object} props - Component props
 * @param {Function} props.onAuthChange - Callback when auth state changes
 */
const GithubAuth = ({ onAuthChange }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState(null);
  const [showTokenInput, setShowTokenInput] = useState(false);
  const [token, setToken] = useState('');

  // Check if user is authenticated on component mount
  useEffect(() => {
    const checkAuth = async () => {
      if (GithubService.isAuthenticated()) {
        setIsAuthenticated(true);
        try {
          setIsLoading(true);
          const userData = await GithubService.getUser();
          setUser(userData);
          setIsLoading(false);
          if (onAuthChange) {
            onAuthChange(true, userData);
          }
        } catch (err) {
          console.error('Error fetching user data:', err);
          setError('Error fetching user data');
          setIsLoading(false);
          // Token might be invalid, clear it
          handleLogout();
        }
      }
    };
    
    checkAuth();
  }, [onAuthChange]);

  // Handle login with token
  const handleLogin = async (e) => {
    e.preventDefault();
    if (!token.trim()) {
      setError('Token is required');
      return;
    }
    
    try {
      setIsLoading(true);
      GithubService.setToken(token);
      const userData = await GithubService.getUser();
      setUser(userData);
      setIsAuthenticated(true);
      setShowTokenInput(false);
      setToken('');
      setError(null);
      setIsLoading(false);
      if (onAuthChange) {
        onAuthChange(true, userData);
      }
    } catch (err) {
      console.error('Authentication error:', err);
      setError('Invalid token or API rate limit exceeded');
      GithubService.setToken(null);
      setIsLoading(false);
    }
  };

  // Handle logout
  const handleLogout = () => {
    GithubService.setToken(null);
    setIsAuthenticated(false);
    setUser(null);
    setShowTokenInput(false);
    setToken('');
    setError(null);
    if (onAuthChange) {
      onAuthChange(false, null);
    }
  };

  // Toggle token input display
  const toggleTokenInput = () => {
    setShowTokenInput(!showTokenInput);
    setError(null);
  };

  return (
    <div className="github-auth">
      {isAuthenticated ? (
        <div className="github-user">
          {user && (
            <div className="user-info">
              <img 
                src={user.avatar_url} 
                alt={user.login} 
                className="user-avatar" 
                width="24" 
                height="24" 
              />
              <span className="user-name">{user.login}</span>
            </div>
          )}
          <button 
            onClick={handleLogout} 
            className="logout-button"
            title="Logout from GitHub"
          >
            <FaSignOutAlt /> Logout
          </button>
        </div>
      ) : (
        <div className="github-login">
          {showTokenInput ? (
            <form onSubmit={handleLogin} className="token-form">
              <input
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="Enter GitHub token"
                autoFocus
                className="token-input"
              />
              <button type="submit" disabled={isLoading} className="login-button">
                {isLoading ? 'Logging in...' : 'Login'}
              </button>
              <button 
                type="button" 
                onClick={toggleTokenInput} 
                className="cancel-button"
              >
                Cancel
              </button>
              {error && <div className="error-message">{error}</div>}
            </form>
          ) : (
            <button 
              onClick={toggleTokenInput} 
              className="github-button"
              title="Login with GitHub token to save and update Gists"
            >
              <FaGithub /> Login
            </button>
          )}
        </div>
      )}
    </div>
  );
};

export default GithubAuth;