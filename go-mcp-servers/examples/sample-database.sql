-- Sample database schema for testing the database MCP server

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login DATETIME
);

-- Create posts table
CREATE TABLE IF NOT EXISTS posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    content TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users (id)
);

-- Create comments table
CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (post_id) REFERENCES posts (id),
    FOREIGN KEY (user_id) REFERENCES users (id)
);

-- Insert sample data
INSERT OR IGNORE INTO users (username, email) VALUES 
    ('alice', 'alice@example.com'),
    ('bob', 'bob@example.com'),
    ('charlie', 'charlie@example.com');

INSERT OR IGNORE INTO posts (user_id, title, content) VALUES 
    (1, 'Welcome to our blog!', 'This is the first post on our new blog platform.'),
    (2, 'Tips for productivity', 'Here are some great tips for staying productive while working from home.'),
    (1, 'Tech trends 2024', 'A look at the most important technology trends for this year.');

INSERT OR IGNORE INTO comments (post_id, user_id, content) VALUES 
    (1, 2, 'Great start! Looking forward to more content.'),
    (1, 3, 'Nice design on the blog platform.'),
    (2, 1, 'These tips are really helpful, thanks!'),
    (2, 3, 'I especially like the tip about time blocking.'),
    (3, 2, 'Interesting predictions. I think AI will be even bigger.');