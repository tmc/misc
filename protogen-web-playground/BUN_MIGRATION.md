# Migrating to Bun

This project has been migrated from npm/Create React App to Bun/Vite for improved performance and development experience.

## What is Bun?

[Bun](https://bun.sh/) is an all-in-one JavaScript runtime and toolkit designed for speed and efficiency. It includes:

- A fast JavaScript runtime
- A package manager compatible with npm packages
- A bundler with built-in optimization
- A test runner and more

## Migration Benefits

1. **Performance**: Bun is significantly faster for installing dependencies, running scripts, and bundling.
2. **Modern Tooling**: Vite provides a faster development experience with instant hot module reloading.
3. **WebAssembly Support**: Better native support for WebAssembly integration.
4. **Developer Experience**: Simplified configuration and faster feedback loops.
5. **Reduced Dependencies**: Fewer dependencies and smaller node_modules folder.

## Major Changes

1. Switched from Create React App to Vite for building/bundling
2. Replaced npm with Bun for package management
3. Updated project structure for Vite conventions
4. Modernized frontend configuration
5. Added dark mode support via CSS variables
6. Improved WebAssembly integration
7. Enhanced React component performance

## Migration Steps

1. Created new Vite-based project configuration
   - vite.config.js
   - index.html (root HTML file)

2. Updated package.json
   - Changed build scripts to use Bun and Vite
   - Updated dependencies to latest versions 
   - Added proper module type

3. Updated Makefile to use Bun commands
   - Added separate targets for development and production

4. Added WebAssembly-specific configuration
   - Configured Vite to properly handle .wasm files
   - Added top-level-await plugin for async WASM loading

5. Enhanced CSS with modern variables and theming
   - Added dark mode support
   - Improved component styling

## How to Run with Bun

### Development mode:

```bash
# Install dependencies
bun install

# Start development server
bun --bun run dev 
```

### Production build:

```bash
# Build for production
bun --bun run build

# Preview production build
bun --bun run preview
```

### Running with Make:

```bash
# Build everything (WASM + frontend + backend)
make

# Run development server
make dev
```

## Troubleshooting

If you encounter issues after migration:

1. **Missing Dependencies**: Run `bun install` to ensure all dependencies are properly installed.

2. **WebAssembly Loading Issues**: Check browser console for errors. Ensure the WASM file is properly built and copied to the correct location.

3. **Styling Problems**: Verify that your browser supports CSS variables. All modern browsers do, but older browsers may require a polyfill.

4. **Monaco Editor Issues**: If the Monaco editor doesn't load properly, try clearing your browser cache or restarting the development server.

5. **Missing Bun**: If you get command not found errors, install Bun with:
   ```bash
   curl -fsSL https://bun.sh/install | bash
   ```