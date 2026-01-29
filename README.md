# File Share

A Cursor/Codex skill for converting local files to shareable URLs via Alibaba Cloud OSS.

## Features

- **Upload any file** and get a shareable URL
- **Transfer URLs** from third-party sources to your own domain
- **Batch upload** multiple files at once
- **Zip bundle** multiple files into one archive
- **Cross-platform** pre-compiled binaries (macOS, Linux, Windows)
- **No dependencies** required

## Installation

### As a Cursor Skill

```bash
# Install from GitHub
# (Follow your skill installer instructions)
```

### Manual

```bash
git clone https://github.com/YOUR_USERNAME/file-share.git
cd file-share
cp .env.example .env
# Edit .env with your OSS credentials
```

## Configuration

Copy `.env.example` to `.env` and fill in your Alibaba Cloud OSS credentials:

```bash
OSS_ACCESS_KEY_ID=your-access-key-id
OSS_ACCESS_KEY_SECRET=your-access-key-secret
OSS_BUCKET_NAME=your-bucket-name
OSS_ENDPOINT=oss-cn-hangzhou.aliyuncs.com
OSS_DOMAIN=your-custom-domain.com  # Optional
OSS_PREFIX=uploads                  # Optional
```

## Usage

```bash
# Upload a file
./file-share image.png

# Upload from URL (transfer to your domain)
./file-share https://example.com/image.png

# Upload multiple files
./file-share file1.txt file2.txt file3.txt

# Bundle as zip
./file-share --zip file1.txt file2.txt

# Quiet mode (only output URLs)
./file-share --quiet image.png
```

See [SKILL.md](SKILL.md) for full documentation.

## Building from Source

```bash
./build.sh
```

Requires Go 1.19+.

## License

MIT
