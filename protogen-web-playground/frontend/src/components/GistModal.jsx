import React, { useState } from 'react';

const GistModal = ({ 
  title, 
  onClose, 
  onSubmit, 
  submitLabel, 
  placeholder, 
  description, 
  showPublicOption 
}) => {
  const [value, setValue] = useState('');
  const [isPublic, setIsPublic] = useState(true);

  const handleSubmit = (e) => {
    e.preventDefault();
    if (showPublicOption) {
      onSubmit(value, isPublic);
    } else {
      onSubmit(value);
    }
  };

  return (
    <div className="modal-backdrop">
      <div className="modal-content">
        <div className="modal-header">
          <h2>{title}</h2>
          <button className="modal-close" onClick={onClose}>&times;</button>
        </div>
        <div className="modal-body">
          <p>{description}</p>
          <form onSubmit={handleSubmit}>
            <div className="form-group">
              <input
                type="text"
                value={value}
                onChange={(e) => setValue(e.target.value)}
                placeholder={placeholder}
                className="form-control"
                required
              />
            </div>
            
            {showPublicOption && (
              <div className="form-group">
                <label>
                  <input
                    type="checkbox"
                    checked={isPublic}
                    onChange={(e) => setIsPublic(e.target.checked)}
                  />
                  {" "}Make Gist public
                </label>
              </div>
            )}
            
            <div className="modal-footer">
              <button type="button" onClick={onClose} className="btn btn-secondary">
                Cancel
              </button>
              <button type="submit" className="btn btn-primary">
                {submitLabel}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
};

export default GistModal;