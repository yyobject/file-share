---
name: file-share
description: Upload files to cloud storage and get shareable URLs. Supports local files, URLs, and bundling. Auto-detects when generating web content with assets.
---

# File Share

Upload files and get shareable URLs. Uses Python with auto-installed dependencies.

## When to Use This Skill

Use this skill automatically when:

1. **Sharing generated files** - Any file created that needs a public URL (HTML pages, reports, images, charts, data files)

2. **Embedding assets in web content** - Building web pages that need to reference external resources (images, CSS, JS, fonts)

3. **Transferring external resources** - User provides third-party URLs that need to be hosted on their domain

4. **Creating shareable artifacts** - Any output that benefits from permanent, accessible URLs (demos, visualizations, prototypes)

**Key workflow**: When generating multi-file outputs (like HTML with images), upload assets first to get URLs, then embed those URLs in the main file.

## Quick Start

### First-Time Setup

1. **Check configuration**:
   ```bash
   ./file-share --check
   ```

2. **If not configured** (`ready: false`), create `~/.file-share.env` with your Alibaba Cloud OSS credentials:
   ```bash
   cp .env.example ~/.file-share.env
   # Edit ~/.file-share.env with your credentials
   ```
   See Configuration section for details.

3. **Verify configuration**:
   ```bash
   ./file-share --check
   ```

4. **Upload your first file**:
   ```bash
   ./file-share myfile.pdf
   ```

**Important**: Always verify config is ready before attempting uploads.

### Common Commands

```bash
# Check configuration
./file-share --check

# Upload single file
./file-share image.png

# Get URL only (for scripting)
./file-share --quiet document.pdf

# Transfer URL to your domain
./file-share https://example.com/photo.jpg

# Upload multiple files
./file-share file1.txt file2.txt file3.txt

# Bundle files as zip
./file-share --zip report.pdf data.csv images/*.png

# Upload directory
./file-share --recursive ./dist/
```

## Common Workflows

### Workflow 1: Single File

```bash
./file-share document.pdf
```

Output:
```json
{
  "success": true,
  "mode": "separate",
  "results": [{
    "file": "document.pdf",
    "url": "https://my-bucket.oss-cn-hangzhou.aliyuncs.com/document_20240129_143052.pdf"
  }]
}
```

**Parse**: Extract `results[0].url`

*Note: Actual URL format depends on your bucket name, endpoint, and optional custom domain.*

### Workflow 2: Web Page with Assets

1. Upload assets first:
```bash
./file-share logo.png hero.jpg
```

2. Parse URLs from output:
```json
{
  "success": true,
  "mode": "separate",
  "results": [
    {"file": "logo.png", "url": "https://my-bucket.oss-cn-hangzhou.aliyuncs.com/uploads/logo_20240129_143102.png"},
    {"file": "hero.jpg", "url": "https://my-bucket.oss-cn-hangzhou.aliyuncs.com/uploads/hero_20240129_143102.jpg"}
  ]
}
```

3. Generate HTML with embedded URLs:
```html
<img src="https://my-bucket.oss-cn-hangzhou.aliyuncs.com/uploads/logo_20240129_143102.png">
<img src="https://my-bucket.oss-cn-hangzhou.aliyuncs.com/uploads/hero_20240129_143102.jpg">
```

4. Upload HTML:
```bash
./file-share --quiet index.html
```

### Workflow 3: Transfer External URLs

```bash
./file-share https://example.com/photo.jpg https://other.com/asset.png
```

Tool downloads files and re-uploads to user's bucket, returning new URLs.

### Workflow 4: Bundle as Archive

```bash
./file-share --zip report.pdf data.csv images/*.png
```

Output:
```json
{
  "success": true,
  "mode": "zip",
  "zip_name": "archive_20240129_143115.zip",
  "files_included": ["report.pdf", "data.csv", "img1.png", "img2.png"],
  "url": "https://my-bucket.oss-cn-hangzhou.aliyuncs.com/archive_20240129_143115.zip"
}
```

**Parse**: Extract `url` field

## Configuration

### Priority (Low to High)

1. `~/.file-share.env` - Global config (recommended for persistent setup)
2. `<skill-dir>/.env` - Skill directory
3. `.env` - Current working directory
4. Environment variables - Highest priority (e.g., `OSS_ACCESS_KEY_ID`)

Higher priority overrides lower priority values.

### Required Variables

| Variable | Description |
|----------|-------------|
| `OSS_ACCESS_KEY_ID` | Alibaba Cloud AccessKey ID |
| `OSS_ACCESS_KEY_SECRET` | Alibaba Cloud AccessKey Secret |
| `OSS_BUCKET_NAME` | Bucket name |
| `OSS_ENDPOINT` | Region endpoint (e.g., `oss-cn-hangzhou.aliyuncs.com`) |

### Optional Variables

| Variable | Description |
|----------|-------------|
| `OSS_DOMAIN` | Custom domain (uses bucket domain if not set) |
| `OSS_PREFIX` | File path prefix (e.g., `uploads`) |

### Example Config File

Copy `.env.example` to one of the config locations and fill in:

```bash
# Required
OSS_ACCESS_KEY_ID=your-access-key-id
OSS_ACCESS_KEY_SECRET=your-access-key-secret
OSS_BUCKET_NAME=your-bucket-name
OSS_ENDPOINT=oss-cn-hangzhou.aliyuncs.com

# Optional
OSS_DOMAIN=cdn.yourdomain.com
OSS_PREFIX=uploads
```

## Output & Error Handling

### Output Format

**Success - Separate files:**
```json
{
  "success": true,
  "mode": "separate",
  "results": [
    {"file": "file1.txt", "url": "https://..."},
    {"file": "file2.txt", "url": "https://..."}
  ]
}
```

**Success - Zip bundle:**
```json
{
  "success": true,
  "mode": "zip",
  "zip_name": "archive.zip",
  "files_included": ["file1.txt", "file2.txt"],
  "url": "https://..."
}
```

**Error:**
```json
{
  "success": false,
  "error": "Error description"
}
```

### Parsing Logic

```python
import json

result = json.loads(output)

if result.get("success"):
    if result["mode"] == "separate":
        # Extract URLs from results array
        for item in result["results"]:
            url = item["url"]
            # Use URL
    elif result["mode"] == "zip":
        # Single zip URL
        url = result["url"]
        # Use URL
else:
    # Handle error
    error = result.get("error")
    # Take action based on error type
```

### Common Errors

| Error | Cause | Action |
|-------|-------|--------|
| `"Missing config: ..."` | Credentials not configured | Guide user through setup, run `--check` |
| `"InvalidAccessKeyId"` | AccessKey ID is incorrect | Verify OSS_ACCESS_KEY_ID in config |
| `"SignatureDoesNotMatch"` | AccessKey Secret is incorrect | Verify OSS_ACCESS_KEY_SECRET in config |
| `"NoSuchBucket"` | Bucket doesn't exist | Verify OSS_BUCKET_NAME, create bucket if needed |
| `"AccessDenied"` | Insufficient permissions | Check bucket permissions and AccessKey policy |
| `"Failed to download ..."` | URL inaccessible | Verify URL is valid and accessible |
| `"File not found: ..."` | Invalid file path | Check path exists |
| `"... is a directory, use --recursive"` | Directory without flag | Add `--recursive` flag |
| `"Permission denied"` | Script not executable | Run `chmod +x ./file-share` |
| `"oss2 not installed"` | Missing dependency | Run `pip install oss2 requests` |

**Error handling steps:**
1. Parse JSON and check `success` field
2. If `false`, read `error` message
3. Match error pattern to table above
4. Take recommended action
5. For config errors, don't retry until fixed
6. Always inform user with clear guidance

## Command Options

| Option | Description |
|--------|-------------|
| `--check` | Check if configuration is ready |
| `--quiet` | Output URLs only (no JSON), useful for scripting |
| `--zip` | Bundle files into zip before upload |
| `--zip-name NAME` | Specify zip filename (auto-generated by default) |
| `--folder NAME` | Top-level folder name in zip (default: files) |
| `--prefix PATH` | Specify OSS path prefix |
| `--no-timestamp` | Keep original filename (don't add timestamp) |
| `--recursive` | Recursively upload directory |
| `--preserve-path` | Preserve directory structure when zipping |
| `--dry-run` | Preview files without uploading |
| `--config FILE` | Specify custom config file path |
| `--version` | Show version |

### Usage Examples

```bash
# Quiet mode for scripting
URL=$(./file-share --quiet file.txt)
echo "Uploaded to: $URL"

# Keep original filename
./file-share --no-timestamp logo.png

# Upload directory with preserved structure
./file-share --recursive --zip --preserve-path ./src/

# Bundle files with custom top-level folder
./file-share --zip --folder assets image1.png image2.png style.css

# Preview before upload
./file-share --dry-run --recursive ./build/

# Custom config file
./file-share --config /path/to/.env file.txt

# Combine options
./file-share --recursive --zip --quiet ./dist/
```

## Troubleshooting

### Permission Denied

If script is not executable:
```bash
chmod +x ./file-share
```

### Dependencies Not Installing

Manually install:
```bash
pip install oss2 requests
```

### Configuration Issues

Run diagnostics:
```bash
./file-share --check
```

Check config file locations and priority (see Configuration section).

## Notes

- Filenames automatically get timestamp suffix to prevent overwriting (disable with `--no-timestamp`)
- Supports wildcards: `*.txt`, `images/*.png`
- URLs (http/https) are automatically detected and downloaded before upload
- Dependencies auto-install on first run
- Python 3.7+ required
