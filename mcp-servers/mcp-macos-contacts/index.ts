#\!/usr/bin/env node

import { MCPServer, Tool } from '@anthropic-ai/mcp';

// Create a new MCP server
const server = new MCPServer();

// Tool to list contacts
const listContactsTool: Tool = {
  name: 'list_contacts',
  description: 'Lists all contacts in the address book',
  parameters: {},
  handler: async () => {
    // Mock implementation - in a real server, this would query the macOS Contacts API
    return {
      contacts: [
        { id: '1', name: 'John Doe', email: 'john@example.com' },
        { id: '2', name: 'Jane Smith', email: 'jane@example.com' },
        { id: '3', name: 'Bob Johnson', email: 'bob@example.com' },
      ]
    };
  }
};

// Tool to get a specific contact
const getContactTool: Tool = {
  name: 'get_contact',
  description: 'Get details for a specific contact by ID',
  parameters: {
    id: {
      description: 'The ID of the contact to retrieve',
      type: 'string',
      required: true
    }
  },
  handler: async ({ id }) => {
    // Mock implementation - in a real server, this would query the macOS Contacts API
    const contacts = {
      '1': { id: '1', name: 'John Doe', email: 'john@example.com', phone: '555-1234' },
      '2': { id: '2', name: 'Jane Smith', email: 'jane@example.com', phone: '555-5678' },
      '3': { id: '3', name: 'Bob Johnson', email: 'bob@example.com', phone: '555-9012' }
    };
    
    if (\!contacts[id]) {
      throw new Error(`Contact with ID ${id} not found`);
    }
    
    return { contact: contacts[id] };
  }
};

// Register tools with the server
server.registerTool(listContactsTool);
server.registerTool(getContactTool);

// Start the server using stdio transport
server.start({
  transport: 'stdio'
});

console.error('MCP macOS Contacts server running...');
