# php-diagls
PHP LSP server for diagnostics

### Purpose
Provide diagnostics (errors, warnings, notices) for containerized PHP projects in editors that support the Language Server Protocol (LSP).
The diagnostic tools need to be installed in the container which runs the PHP code or in a separate container.

### Configuration
Create a `.php-diagls.json` configuration file in the project root. Example configuration:
```json
{
    "phpcsfixer": {
        "container": "php-server",
        "executable": "php-cs-fixer",
        "config": ".php-cs-fixer.dist.php"
    },
    "phpstan": {
        "container": "php-server",
        "executable": "phpstan",
        "config": "phpstan.neon"
    }
}
```



# PHP Diagnostics LSP Server

A Language Server Protocol (LSP) implementation for PHP that provides dynamic diagnostics using configurable external tools like PHP CS Fixer running in Docker containers.

## Features

- **Modular Architecture**: Clean separation of concerns with pluggable diagnostic providers
- **Dynamic Diagnostics**: Analyze PHP files using configurable external tools
- **Docker Integration**: Run PHP CS Fixer and other tools inside Docker containers
- **Real-time Analysis**: Get diagnostics when files are opened or modified
- **Automatic Refresh**: Diagnostics update automatically when files are saved or changed externally
- **Configuration Error Notifications**: Get notified directly in your editor about configuration issues (container not running, missing tools, invalid settings)
- **Configurable**: Use `.php-diagls.json` configuration files for project-specific settings

## Installation

1. Clone this repository
2. Build the LSP server:
   ```bash
   go build -o php-diagls cmd/main.go
   ```

## Configuration

Create a `.php-diagls.json` file in your project root directory to configure the diagnostics tools:

```json
{
  "phpcsfixer": {
    "container": "my-php-container",
    "path": "/usr/local/bin/php-cs-fixer",
    "config": ".php-cs-fixer.dist.php"
  }
}
```

### Configuration Options

#### PHP CS Fixer (`phpcsfixer`)

- **`container`**: Name of the Docker container where PHP CS Fixer is installed
- **`path`**: Full path to the PHP CS Fixer executable inside the container
- **`config`**: (Optional) Path to the PHP CS Fixer configuration file inside the container

## Usage

### Running the LSP Server

The server can be run in different modes:

#### Stdin/Stdout Mode (for editors)
```bash
./php-diagls -stdin
```

#### Debug Mode
```bash
./php-diagls
```

### Editor Integration

#### Neovim

Add to your Neovim configuration:

```lua
local lspconfig = require('lspconfig')

lspconfig.php_diagls = {
  cmd = { '/path/to/php-diagls', '-stdin' },
  filetypes = { 'php' },
  root_dir = function(fname)
    return lspconfig.util.find_git_ancestor(fname) or vim.fn.getcwd()
  end,
}

lspconfig.php_diagls.setup{}
```

## Diagnostic Refresh System

The LSP server automatically keeps diagnostics up-to-date even when files are modified externally:

### Automatic Refresh Triggers

- **File Save (`textDocument/didSave`)**: When you save a file in your editor
- **External Changes (`workspace/didChangeWatchedFiles`)**: When files are modified outside your editor
- **Document Changes (`textDocument/didChange`)**: When you edit files in your editor

This ensures diagnostics remain accurate after:
- Running external formatters (e.g., `php-cs-fixer fix file.php`)
- Git operations (checkout, pull, merge, rebase)
- Build scripts that modify files
- Auto-formatters and save hooks
- Manual file editing outside your editor

## Configuration Error Notifications

The LSP server validates your configuration and notifies you directly in the editor about any issues:

### Types of Configuration Errors

- **Container Not Running**: Notifies when Docker containers are not available
- **Missing Tools**: Alerts when PHP CS Fixer, PHPStan, or other tools are not installed
- **Invalid Paths**: Warns about incorrect binary paths or missing config files
- **Configuration Issues**: Validates settings like file paths, etc.

### Error Display

Configuration errors appear as diagnostics in your editor with:
- **Clear descriptions** of what's wrong
- **Actionable suggestions** for fixing issues
- **Detailed information** for troubleshooting
- **Appropriate severity levels** (Error/Warning/Information)

## Diagnostics

### PHP CS Fixer Integration

The server runs PHP CS Fixer with the following command:
```bash
docker exec <container> sh -c "<path> fix <file> --dry-run --diff --format=json 2>/dev/null"
```

This provides:
- Code style violation detection
- Formatting suggestions
- Non-intrusive analysis (dry-run mode)
- Clean JSON output (stderr redirected to /dev/null)
- Complete exit code handling with proper error diagnostics
- Line-specific diagnostic positioning from diff analysis

### Exit Code Handling

The LSP server properly handles all PHP CS Fixer exit codes using bit flags:

- **0**: OK - No fixes needed
- **1**: General error (or PHP minimal requirement not matched) - Stops processing
- **4**: Invalid syntax detected - Creates error diagnostic
- **8**: Files need fixing - Expected in dry-run mode, continues processing
- **16**: Application configuration error - Creates error diagnostic  
- **32**: Fixer configuration error - Creates error diagnostic
- **64**: Internal exception - Creates error diagnostic

Exit codes can be combined (e.g., exit code 12 = 4 + 8 means invalid syntax AND fixes needed).

### Line-Specific Diagnostics

The LSP server parses PHP CS Fixer's unified diff output to provide accurate line positioning:

- **Removed lines (-)**: Shows "Code style: formatting required" at the exact problematic line
- **Added lines (+)**: Shows "Code style: missing 'content'" at the location where content should be inserted
- **Context lines**: Used for accurate line number tracking
- **Fallback**: If diff parsing fails, shows general diagnostic at line 1

This ensures diagnostics appear exactly where the code style issues are located, not just at the beginning of the file.

