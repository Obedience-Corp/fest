# Fest Configuration Guide

This document describes all available configuration options for fest.

## Configuration File Location

fest stores its configuration at:

```
~/.config/fest/config.json
```

You can override this location by setting the `FEST_CONFIG_DIR` environment variable:

```bash
export FEST_CONFIG_DIR=/path/to/custom/config
```

## Managing Configuration

### Interactive TUI

The recommended way to manage configuration is through the interactive TUI:

```bash
fest system config
```

This opens a menu-driven interface where you can:
- Navigate categories with arrow keys or j/k
- Press Enter to select a category
- Press Esc to go back
- Modify settings through forms

### View Current Configuration

Display the current configuration as JSON:

```bash
fest system config --show
```

### Reset to Defaults

Within the TUI, select "Reset to Defaults" to restore all settings to their default values.

## Configuration Categories

### Behavior Settings

Controls how fest behaves during operations.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `auto_backup` | bool | `false` | Automatically backup files before updates |
| `interactive` | bool | `true` | Prompt for confirmations during operations |
| `use_color` | bool | `true` | Enable colored output (respects `NO_COLOR` env) |
| `verbose` | bool | `false` | Show detailed operation information |
| `editor` | string | `""` | Preferred editor for wizard fill (empty = `$EDITOR` or `vim`) |
| `editor_mode` | string | `"buffer"` | Editor window mode: `buffer`, `tab`, `split`, `hsplit` |
| `editor_flags` | []string | `null` | Custom editor flags (overrides mode) |

### Network Settings

Controls network behavior for template sync operations.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `timeout` | int | `30` | HTTP request timeout in seconds |
| `retry_count` | int | `3` | Number of retry attempts for failed requests |
| `retry_delay` | int | `1` | Delay between retry attempts in seconds |

### TUI Settings

Controls terminal user interface behavior.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `vim_mode` | bool | `false` | Enable vim-style keybindings (j/k navigation) |
| `expand_inputs` | bool | `true` | Auto-expand text areas as content grows |
| `max_input_height` | int | `10` | Maximum lines for expandable text areas |
| `theme` | string | `"adaptive"` | Color theme: `adaptive`, `light`, `dark`, `high-contrast` |

### Repository Settings

Controls where fest fetches methodology templates from.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `url` | string | `"https://github.com/Obedience-Corp/fest"` | GitHub repository URL |
| `branch` | string | `"main"` | Git branch to sync from |
| `path` | string | `"methodology/festivals"` | Path within repository to methodology files |

### Local Path Settings

Controls local file paths for caching and backups.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `cache_dir` | string | `"~/.config/fest/cache"` | Directory for cached template downloads |
| `backup_dir` | string | `".fest-backup"` | Directory for file backups (relative to workspace) |
| `checksum_file` | string | `".fest-checksums.json"` | File for tracking template checksums |

## Environment Variables

fest respects the following environment variables:

| Variable | Description |
|----------|-------------|
| `FEST_CONFIG_DIR` | Override the default configuration directory |
| `NO_COLOR` | Disable colored output (any value disables color) |
| `EDITOR` | Fallback editor if `editor` config is not set |

## Example Configuration

```json
{
  "version": "1.0.0",
  "repository": {
    "url": "https://github.com/Obedience-Corp/fest",
    "branch": "main",
    "path": "methodology/festivals"
  },
  "local": {
    "cache_dir": "/home/user/.config/fest/cache",
    "backup_dir": ".fest-backup",
    "checksum_file": ".fest-checksums.json"
  },
  "behavior": {
    "auto_backup": false,
    "interactive": true,
    "use_color": true,
    "verbose": false,
    "editor": "nvim",
    "editor_mode": "buffer",
    "editor_flags": null
  },
  "network": {
    "timeout": 30,
    "retry_count": 3,
    "retry_delay": 1
  },
  "tui": {
    "vim_mode": true,
    "expand_inputs": true,
    "max_input_height": 10,
    "theme": "adaptive"
  },
  "last_sync": "2025-01-22T10:30:00Z"
}
```

## Theme Options

| Theme | Description |
|-------|-------------|
| `adaptive` | Auto-detects light/dark terminal background (default) |
| `light` | Optimized colors for light terminal backgrounds |
| `dark` | Optimized colors for dark terminal backgrounds |
| `high-contrast` | Maximum visibility for any background |

## Editor Mode Options

| Mode | Description |
|------|-------------|
| `buffer` | Open in clean single window (default) |
| `tab` | Open in new tab |
| `split` | Open in vertical split |
| `hsplit` | Open in horizontal split |
