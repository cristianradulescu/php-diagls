# PHP Diagnostics LSP Server

A Language Server Protocol (LSP) implementation for PHP that provides dynamic diagnostics using configurable external tools like PHP CS Fixer running in Docker containers.

## Features

- **Docker Integration**: Run PHP CS Fixer and other tools inside Docker containers
- **Diagnostics**: Code analysis and issue detection, run when a file is opened, saved, or changed on disk
- **Document Formatting**: Automatic code formatting using php-cs-fixer or Mago
- **Configurable**: Use `.php-diagls.json` configuration files for project-specific settings

## Installation

1. Clone this repository
2. Build the LSP server:
   ```bash
   make build
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
      "configFile": ".php-cs-fixer.dist.php",
      "format": {
        "enabled": true
      },
      "excludePaths": ["tests", "var", "public"]
    },
    "phpstan": {
      "enabled": false,
      "container": "my-php-container",
      "path": "/usr/local/bin/phpstan",
      "configFile": "phpstan.neon"
    },
    "phplint": {
      "enabled": true,
      "container": "my-php-container",
      "path": "/usr/local/bin/php"
    },
    "mago": {
      "enabled": false,
      "container": "my-php-container",
      "path": "/usr/local/bin/mago",
      "configFile": "mago.toml",
      "commands": ["lint", "analyze"]
    }
  }
}
```

### Configuration Options

- **`enabled`**: Quick status toggle for the diagnostic provider
- **`container`**: Name of the Docker container where the diagnostic provider tool is installed
- **`path`**: Full path to the diagnostic provider executable inside the container
- **`configFile`**: (Optional) Path to the diagnostic provider configuration file inside the container
- **`commands`**: (Optional, `mago` only) Which Mago commands to run for diagnostics: `"lint"`, `"analyze"`, or both (the default)
- **`format.enabled`**: (Optional) Enable document formatting using this provider (`phpcsfixer` or `mago`)
- **`format.timeoutSeconds`**: (Optional) Nb of seconds to allow the formatting process to run 
- **`excludePaths`**: (Optional) List of paths, relative to the project root, to exclude from this provider's diagnostics and formatting. Each entry can be a directory name (e.g. `"tests"`, matched against any path segment), an exact relative path (e.g. `"config/services.php"`), or a glob pattern (e.g. `"src/Legacy/*"`)

> **Note:** Tools like php-cs-fixer only apply their own `Finder`-based `exclude()`/`in()` rules when run **without** an explicit file argument. Since php-diagls always invokes the tool against a specific file, those excludes are never consulted — use the `excludePaths` option above to replicate them (e.g. mirror your `.php-cs-fixer.dist.php` Finder excludes here).

### Mago

The `mago` provider runs [Mago](https://github.com/carthage-software/mago) inside the container. By default each file is checked with both `mago lint` and `mago analyze` (run concurrently); set `commands` to run only one of them. Diagnostics are reported with the source `mago lint` / `mago analyze` and the Mago issue code (e.g. `strict-types`, `invalid-argument`).

- `configFile` is passed as Mago's `--config`; without it, Mago uses `mago.toml` from the project root (the container's working directory).
- `mago analyze` is given only the current file, so it can only resolve classes and functions from other files when your `mago.toml` lists them under `[source] paths` (e.g. `paths = ["src", "tests"]`, plus `includes = ["vendor"]` for dependencies). Without that, expect false `non-existent-class` errors.
- With `format.enabled`, formatting pipes the buffer through `mago format --stdin-input`. The server uses a single formatter and which one it picks is unspecified when several have formatting enabled, so enable it on only one of `phpcsfixer` and `mago`.

The Docker image in `docker/Dockerfile` ships `/usr/local/bin/mago` (see [Docker Image](#docker-image)).

## Docker Image

`docker/Dockerfile` builds a PHP image with every supported tool installed at `/usr/local/bin`: `php`, `composer`, `php-cs-fixer`, `phpstan` and `mago`. Build it with:

```bash
make docker-build
```

Every tool defaults to its latest release. To pin versions, pass any of these variables (they are forwarded as `--build-arg`s):

| Variable | Default | Example |
| --- | --- | --- |
| `PHP_VERSION` | `8.3` | `8.4` (any `php:<version>-cli` tag) |
| `COMPOSER_VERSION` | `latest` | `2.8` (any `composer` image tag) |
| `PHP_CS_FIXER_VERSION` | `latest` | `3.95.25` (the `v` prefix is optional) |
| `PHPSTAN_VERSION` | `latest` | `2.2.14` |
| `MAGO_VERSION` | `latest` | `1.49.0` (any `ghcr.io/carthage-software/mago` tag) |
| `UID` / `GID` | `1000` | user/group id of the in-container `appuser` |
| `DOCKER_IMAGE` | `php-diagls-tools` | name/tag of the built image |

```bash
make docker-build PHP_VERSION=8.4 MAGO_VERSION=1.49.0 PHPSTAN_VERSION=2.2.14 PHP_CS_FIXER_VERSION=3.95.25
# or, without make:
docker build --build-arg MAGO_VERSION=1.49.0 -t php-diagls-tools docker/
```

A version that doesn't exist fails the build. Versions aren't checked against each other: an older tool may refuse to run on a newer `PHP_VERSION` (e.g. php-cs-fixer 3.60 on PHP 8.4). With `latest`, Docker's layer cache keeps whatever was downloaded last time; add `--pull --no-cache` to `docker build` to pick up new releases.

## Document Formatting

The LSP server supports automatic document formatting using php-cs-fixer (described below) or Mago (see [Mago](#mago)). When enabled, you can format PHP files using your editor's format command.

### How It Works

1. **Stdin Processing**: Content is sent to php-cs-fixer via stdin (no temporary files)
2. **Diff Analysis**: php-cs-fixer returns a unified diff of proposed changes
3. **Safe Application**: Changes are applied without modifying files on disk
4. **Container Integration**: Formatting runs inside your specified Docker container

### Enabling Formatting

Add the `format` configuration to your php-cs-fixer provider:

```json
{
  "diagnosticsProviders": {
    "phpcsfixer": {
      "enabled": true,
      "container": "my-php-container",
      "path": "/usr/local/bin/php-cs-fixer",
      "configFile": ".php-cs-fixer.dist.php",
      "format": {
        "enabled": true
      }
    }
  }
}
```

## Usage

### Editor Integration

#### Neovim

```lua
-- lua/lsp/php_diagls.lua
return {
  cmd = { '/path/to/php-diagls' },
  root_markers = { '.git', 'composer.json' },
  filetypes = { 'php'},
}
```

Then in the LSP configuration:

```lua
vim.lsp.enable({'php-diagls'})
```

#### Formatting Commands

Once configured, you can format documents using:

- **Neovim**: `:lua vim.lsp.buf.format()`
