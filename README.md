# File Share

A skill for uploading files to cloud storage and getting shareable URLs via Alibaba Cloud OSS.

Perfect for AI assistants that generate files and need to share them with users through accessible URLs.

## Why This Skill?

When AI assistants generate files (HTML pages, images, reports, etc.), they need a way to share them with users. This skill:

- ✅ Uploads files to Alibaba Cloud OSS
- ✅ Returns shareable URLs instantly
- ✅ Handles web assets (images, CSS, JS) for generated HTML
- ✅ Transfers third-party URLs to your domain
- ✅ Bundles multiple files into archives

## Features

- 🚀 **Simple**: One command to upload, get URL back
- 🌐 **URL Transfer**: Download from any URL and re-upload to your domain
- 📦 **Batch Upload**: Multiple files at once, or bundle as zip
- 📁 **Organized Archives**: Auto-organize files into folders within zip archives
- 🐍 **Python-based**: Cross-platform (macOS, Linux, Windows)
- 🔧 **Auto-install**: Dependencies install automatically on first run
- ⚙️ **Flexible Config**: Global or per-project configuration

## Quick Start

### 1. Install

#### As a Skill (Recommended)

Install using your AI assistant's skill manager.

#### Manual Installation

```bash
git clone https://github.com/yyobject/file-share.git
cd file-share
chmod +x file-share
```

### 2. Configure

Create configuration file (recommended: global config):

```bash
cp .env.example ~/.file-share.env
# Edit ~/.file-share.env with your Alibaba Cloud OSS credentials
```

**Required credentials:**
- `OSS_ACCESS_KEY_ID` - Your AccessKey ID
- `OSS_ACCESS_KEY_SECRET` - Your AccessKey Secret
- `OSS_BUCKET_NAME` - Your bucket name
- `OSS_ENDPOINT` - Region endpoint (e.g., `oss-cn-hangzhou.aliyuncs.com`)

**Optional:**
- `OSS_DOMAIN` - Custom domain for URLs
- `OSS_PREFIX` - Path prefix for uploaded files

### 3. Verify

```bash
./file-share --check
```

Expected output:
```json
{
  "ready": true,
  "env_vars": {
    "access_key_id": true,
    "access_key_secret": true,
    "bucket_name": true,
    "endpoint": true
  }
}
```

### 4. Upload

```bash
./file-share myfile.pdf
```

Returns:
```json
{
  "success": true,
  "mode": "separate",
  "results": [{
    "file": "myfile.pdf",
    "url": "https://my-bucket.oss-cn-hangzhou.aliyuncs.com/myfile_20240129_143052.pdf"
  }]
}
```

## Usage Examples

### Single File
```bash
./file-share document.pdf
```

### Transfer URL to Your Domain
```bash
./file-share https://example.com/image.png
```

### Multiple Files
```bash
./file-share file1.txt file2.txt file3.txt
```

### Bundle as Zip
```bash
./file-share --zip report.pdf data.csv images/*.png
```

### Organized Zip with Custom Folder
```bash
# Files organized in "assets/" folder within zip
./file-share --zip --folder assets image1.png image2.png style.css

# Default: files organized in "files/" folder
./file-share --zip image1.png image2.png
```

### Scripting (Quiet Mode)
```bash
URL=$(./file-share --quiet report.pdf)
echo "File uploaded: $URL"
```

### Upload Directory
```bash
./file-share --recursive ./dist/
```

## Configuration Priority

Configuration is loaded from multiple sources (lower to higher priority):

1. `~/.file-share.env` - Global config (recommended)
2. `<skill-dir>/.env` - Skill directory config
3. `.env` - Current working directory
4. Environment variables - Highest priority

Higher priority config overrides lower priority values.

## Requirements

- **Python**: 3.7 or higher
- **Dependencies**: `oss2`, `requests` (auto-installed on first run)
- **Alibaba Cloud OSS**: Active bucket with credentials

## Implementation Details

This skill is Python-based, replacing the previous Go implementation for better:

- **Cross-platform compatibility** - No compilation needed
- **Dependency management** - pip handles everything automatically
- **Maintenance** - Easier to extend and customize
- **Integration** - Better integration with AI assistant environments

The entry script (`file-share`) is a bash wrapper that:
1. Detects Python installation (python3 or python)
2. Auto-installs dependencies if missing
3. Executes the main Python script

## Documentation

- **[SKILL.md](SKILL.md)** - Complete documentation for AI assistants
- **[.env.example](.env.example)** - Configuration template

## Common Options

```bash
--check              # Check configuration status
--quiet              # Output URLs only (no JSON)
--zip                # Bundle files as zip
--folder NAME        # Top-level folder in zip (default: files)
--prefix PATH        # Specify upload path prefix
--no-timestamp       # Keep original filename
--recursive          # Upload directory recursively
--dry-run            # Preview without uploading
--config FILE        # Use custom config file
```

## Troubleshooting

### Permission Denied
```bash
chmod +x ./file-share
```

### Python Not Found
Install Python 3.7+:
- macOS: `brew install python3`
- Ubuntu: `apt install python3`
- Windows: Download from [python.org](https://www.python.org)

### Dependencies Failed to Install
```bash
pip install oss2 requests
```

### Configuration Issues
```bash
./file-share --check
```

Check the output for missing or incorrect configuration values.

## Contributing

Contributions welcome! Please feel free to submit issues or pull requests.

## License

MIT

## Author

[yyobject](https://github.com/yyobject)
