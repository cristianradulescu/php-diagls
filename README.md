# PHP Diagnostics LSP Server

A Language Server Protocol (LSP) implementation for PHP that provides dynamic diagnostics using configurable external tools like PHP CS Fixer running in Docker containers.

## Features

- **Docker Integration**: Run PHP CS Fixer and other tools inside Docker containers
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
  "diagnosticsProviders": {
    "phpcsfixer": {
      "enabled": true,
      "container": "my-php-container",
      "path": "/usr/local/bin/php-cs-fixer",
      "configFile": ".php-cs-fixer.dist.php"
    },
    "phpstan": {
      "enabled": false,
      "container": "my-php-container",
      "path": "/usr/local/bin/phpstan",
      "configFile": "phpstan.neon" 
    }
  }
}
```

### Configuration Options

#### PHP CS Fixer (`phpcsfixer`)

- **`enabled`**: Quick status toggle for the diagnostic provider 
- **`container`**: Name of the Docker container where the diagnostic provider tool is installed
- **`path`**: Full path to the diagnostic provider executable inside the container
- **`config`**: (Optional) Path to the diagnostic provider configuration file inside the container

## Usage

### Editor Integration

#### Neovim

**With nvim-lspconfig**

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

** With Neovim's built-in LSP client**

```lua
-- lua/lsp/php_diagls.lua
return {
  cmd = { '<path_to_lsp_binary>' },
  root_markers = { 'composer.json', '.git' },
  filetypes = { 'php'},
}
```

Then in the LSP configuration:

```lua
vim.lsp.enable({ <my_other_lsps>, 'php-diagls'})
```

