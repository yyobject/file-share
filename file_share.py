#!/usr/bin/env python3
"""
File Share - Upload files to Alibaba Cloud OSS and get shareable URLs.
"""

import argparse
import json
import os
import sys
import tempfile
import zipfile
from datetime import datetime
from pathlib import Path
from urllib.parse import urlparse
import re

try:
    import oss2
except ImportError:
    print(json.dumps({"success": False, "error": "oss2 not installed. Run: pip install oss2"}))
    sys.exit(1)

try:
    import requests
except ImportError:
    print(json.dumps({"success": False, "error": "requests not installed. Run: pip install requests"}))
    sys.exit(1)

VERSION = "2.0.0"


def get_skill_dir():
    """Get the skill directory path."""
    return Path(__file__).parent.resolve()


def load_env_file(path):
    """Parse .env file and return dict."""
    env = {}
    try:
        with open(path, 'r') as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith('#'):
                    continue
                if '=' in line:
                    key, value = line.split('=', 1)
                    key = key.strip()
                    value = value.strip().strip('"').strip("'")
                    env[key] = value
    except FileNotFoundError:
        pass
    return env


def get_config(custom_config_path=None):
    """Load configuration from various sources with priority."""
    config = {}
    
    if custom_config_path:
        env = load_env_file(custom_config_path)
        if not env:
            return None, f"Cannot read config file: {custom_config_path}"
        config = env
    else:
        # Priority from low to high

        # 1. User home directory ~/.file-share.env (lowest priority)
        home_config = Path.home() / '.file-share.env'
        config.update(load_env_file(home_config))
        
        # 2. Skill directory .env
        skill_config = get_skill_dir() / '.env'
        config.update(load_env_file(skill_config))
        
        # 3. Current working directory .env
        cwd_config = Path.cwd() / '.env'
        config.update(load_env_file(cwd_config))
    
    # 4. Environment variables (highest priority)
    env_vars = [
        'OSS_ACCESS_KEY_ID', 'OSS_ACCESS_KEY_SECRET', 
        'OSS_BUCKET_NAME', 'OSS_ENDPOINT',
        'OSS_DOMAIN', 'OSS_PREFIX'
    ]
    for var in env_vars:
        if os.environ.get(var):
            config[var] = os.environ[var]
    
    # Check required fields
    required = ['OSS_ACCESS_KEY_ID', 'OSS_ACCESS_KEY_SECRET', 'OSS_BUCKET_NAME', 'OSS_ENDPOINT']
    missing = [k for k in required if not config.get(k)]
    
    if missing:
        return None, f"Missing config: {', '.join(missing)}"
    
    return config, None


def get_config_sources():
    """Get list of found config sources."""
    sources = []

    home_config = Path.home() / '.file-share.env'
    if home_config.exists():
        sources.append(str(home_config))
    
    skill_config = get_skill_dir() / '.env'
    if skill_config.exists():
        sources.append(str(skill_config))
    
    cwd_config = Path.cwd() / '.env'
    if cwd_config.exists():
        sources.append(".env (current dir)")
    
    if os.environ.get('OSS_ACCESS_KEY_ID'):
        sources.append("environment variables")
    
    return sources


def check_env():
    """Check if configuration is ready."""
    config, _ = get_config()
    
    result = {
        "ready": True,
        "env_vars": {},
        "optional_vars": {},
        "missing": [],
        "suggestions": []
    }
    
    required = ['OSS_ACCESS_KEY_ID', 'OSS_ACCESS_KEY_SECRET', 'OSS_BUCKET_NAME', 'OSS_ENDPOINT']
    optional = ['OSS_DOMAIN', 'OSS_PREFIX']
    
    for key in required:
        has_value = bool(config and config.get(key))
        result["env_vars"][key.lower().replace('oss_', '')] = has_value
        if not has_value:
            result["ready"] = False
            result["missing"].append(key.lower().replace('oss_', ''))
    
    for key in optional:
        result["optional_vars"][key.lower().replace('oss_', '')] = bool(config and config.get(key))
    
    sources = get_config_sources()
    if sources:
        result["config_source"] = " -> ".join(sources)
    
    if result["missing"]:
        result["suggestions"] = [
            f"Missing config: {', '.join(result['missing'])}",
            "Configuration methods (priority low to high):",
            "  1. ~/.file-share.env (global)",
            "  2. <skill-dir>/.env",
            "  3. .env (current directory)",
            "  4. Environment variables (OSS_ACCESS_KEY_ID, etc.)"
        ]
    
    print(json.dumps(result, indent=2))
    
    if result["ready"]:
        print("\n✅ Ready to upload", file=sys.stderr)
        sys.exit(0)
    else:
        print("\n❌ Not ready, please configure as suggested", file=sys.stderr)
        sys.exit(1)


def generate_oss_key(filename, prefix, no_timestamp):
    """Generate OSS object key."""
    base = Path(filename).name
    
    if no_timestamp:
        key = base
    else:
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        stem = Path(base).stem
        suffix = Path(base).suffix
        key = f"{stem}_{timestamp}{suffix}"
    
    if prefix:
        key = f"{prefix.strip('/')}/{key}"
    
    return key


def get_file_url(config, oss_key):
    """Generate file URL."""
    domain = config.get('OSS_DOMAIN', '')
    
    if domain:
        domain = domain.rstrip('/')
        if not domain.startswith('http://') and not domain.startswith('https://'):
            domain = f"https://{domain}"
        return f"{domain}/{oss_key}"
    
    endpoint = config['OSS_ENDPOINT']
    scheme = "https"
    rest = endpoint
    
    if endpoint.startswith('http://'):
        scheme = "http"
        rest = endpoint[7:]
    elif endpoint.startswith('https://'):
        scheme = "https"
        rest = endpoint[8:]
    
    bucket = config['OSS_BUCKET_NAME']
    return f"{scheme}://{bucket}.{rest}/{oss_key}"


def is_url(s):
    """Check if string is a URL."""
    return s.startswith('http://') or s.startswith('https://')


def get_filename_from_url(url):
    """Extract filename from URL."""
    parsed = urlparse(url)
    path = parsed.path
    if not path or path == '/':
        return "downloaded_file"
    filename = Path(path).name
    # Remove query string if present
    if '?' in filename:
        filename = filename.split('?')[0]
    return filename if filename else "downloaded_file"


def download_url(url):
    """Download URL to temporary file."""
    try:
        resp = requests.get(url, stream=True, timeout=60)
        resp.raise_for_status()
    except Exception as e:
        return None, None, f"Failed to download {url}: {e}"
    
    # Get filename
    filename = get_filename_from_url(url)
    
    # Try Content-Disposition header
    cd = resp.headers.get('Content-Disposition', '')
    if 'filename=' in cd:
        match = re.search(r'filename=(["\']?)(.+?)\1(?:;|$)', cd)
        if match:
            filename = match.group(2)
    
    # Create temp file
    suffix = Path(filename).suffix
    tmp = tempfile.NamedTemporaryFile(delete=False, suffix=suffix)
    try:
        for chunk in resp.iter_content(chunk_size=8192):
            tmp.write(chunk)
        tmp.close()
        return tmp.name, filename, None
    except Exception as e:
        tmp.close()
        os.unlink(tmp.name)
        return None, None, f"Failed to save downloaded file: {e}"


def create_zip(files, preserve_path, base_dir, filename_map):
    """Create zip archive from files."""
    tmp = tempfile.NamedTemporaryFile(delete=False, suffix='.zip')
    tmp.close()
    
    try:
        with zipfile.ZipFile(tmp.name, 'w', zipfile.ZIP_DEFLATED) as zf:
            for file_path in files:
                # Determine archive name
                if file_path in filename_map:
                    arcname = filename_map[file_path]
                elif preserve_path and base_dir:
                    try:
                        arcname = os.path.relpath(file_path, base_dir)
                    except ValueError:
                        arcname = Path(file_path).name
                else:
                    arcname = Path(file_path).name
                
                zf.write(file_path, arcname)
        
        return tmp.name, None
    except Exception as e:
        os.unlink(tmp.name)
        return None, f"Failed to create zip: {e}"


def expand_files(patterns, recursive):
    """Expand file patterns to list of files."""
    files = []
    
    for pattern in patterns:
        path = Path(pattern)
        
        if path.is_dir():
            if recursive:
                for f in path.rglob('*'):
                    if f.is_file():
                        files.append(str(f))
            else:
                return None, f"{pattern} is a directory, use --recursive to upload recursively"
        elif '*' in pattern or '?' in pattern:
            matches = list(Path('.').glob(pattern))
            if not matches:
                return None, f"No files matched: {pattern}"
            for m in matches:
                if m.is_file():
                    files.append(str(m))
        else:
            if not path.exists():
                return None, f"File not found: {pattern}"
            files.append(str(path))
    
    if not files:
        return None, "No files to upload"
    
    return files, None


def output_result(result, quiet):
    """Output result in JSON or quiet mode."""
    if quiet:
        if result.get('mode') == 'zip':
            print(result['url'])
        else:
            for r in result.get('results', []):
                print(r['url'])
    else:
        print(json.dumps(result, indent=2))


def output_error(error, quiet):
    """Output error and exit."""
    if quiet:
        print(f"Error: {error}", file=sys.stderr)
    else:
        print(json.dumps({"success": False, "error": error}, indent=2), file=sys.stderr)
    sys.exit(1)


def main():
    parser = argparse.ArgumentParser(
        description='Upload files to OSS and get shareable URLs',
        formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument('files', nargs='*', help='Files or URLs to upload')
    parser.add_argument('--check', action='store_true', help='Check configuration')
    parser.add_argument('--version', action='store_true', help='Show version')
    parser.add_argument('--zip', action='store_true', help='Bundle files into zip before upload')
    parser.add_argument('--zip-name', help='Zip filename (auto-generated by default)')
    parser.add_argument('--prefix', help='OSS path prefix')
    parser.add_argument('--no-timestamp', action='store_true', help="Don't add timestamp suffix")
    parser.add_argument('--recursive', action='store_true', help='Recursively upload directory')
    parser.add_argument('--preserve-path', action='store_true', help='Preserve directory structure when zipping')
    parser.add_argument('--quiet', action='store_true', help='Quiet mode, only output URLs')
    parser.add_argument('--dry-run', action='store_true', help='Preview mode, only show file list')
    parser.add_argument('--config', help='Specify config file path')
    
    args = parser.parse_args()
    
    if args.version:
        print(f"file-share version {VERSION}")
        sys.exit(0)
    
    if args.check:
        check_env()
        return
    
    if not args.files:
        parser.print_help()
        sys.exit(1)
    
    # Separate URLs from local files
    local_args = []
    downloaded_files = []  # (tmp_path, filename, orig_url)
    
    for arg in args.files:
        if is_url(arg):
            if not args.quiet:
                print(f"Downloading: {arg}", file=sys.stderr)
            tmp_path, filename, err = download_url(arg)
            if err:
                output_error(err, args.quiet)
            downloaded_files.append((tmp_path, filename, arg))
        else:
            local_args.append(arg)
    
    # Cleanup function
    def cleanup():
        for tmp_path, _, _ in downloaded_files:
            try:
                os.unlink(tmp_path)
            except:
                pass
    
    try:
        # Expand local files
        files = []
        if local_args:
            files, err = expand_files(local_args, args.recursive)
            if err:
                output_error(err, args.quiet)
        
        # Build filename map for downloaded files
        tmp_to_filename = {}
        for tmp_path, filename, _ in downloaded_files:
            files.append(tmp_path)
            tmp_to_filename[tmp_path] = filename
        
        if not files:
            output_error("No files to upload", args.quiet)
        
        # Preview mode
        if args.dry_run:
            print(f"Will upload {len(files)} files:")
            for f in files:
                if f in tmp_to_filename:
                    print(f"  {tmp_to_filename[f]} (from URL)")
                else:
                    print(f"  {f}")
            if args.zip:
                name = args.zip_name or "archive_<timestamp>.zip"
                print(f"\nZip mode: will bundle as {name}")
            sys.exit(0)
        
        # Get config
        config, err = get_config(args.config)
        if err:
            output_error(err, args.quiet)
        
        # Create OSS client
        auth = oss2.Auth(config['OSS_ACCESS_KEY_ID'], config['OSS_ACCESS_KEY_SECRET'])
        endpoint = config['OSS_ENDPOINT']
        if not endpoint.startswith('http'):
            endpoint = f"https://{endpoint}"
        bucket = oss2.Bucket(auth, endpoint, config['OSS_BUCKET_NAME'])
        
        # Determine prefix
        oss_prefix = config.get('OSS_PREFIX', '')
        if args.prefix:
            oss_prefix = args.prefix
        
        # Calculate base directory
        base_dir = ""
        if args.preserve_path and local_args:
            first_arg = Path(local_args[0])
            if first_arg.is_dir():
                base_dir = str(first_arg)
            else:
                base_dir = str(first_arg.parent)
        
        if args.zip:
            # Zip upload mode
            name = args.zip_name
            auto_generated = not name
            if auto_generated:
                timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
                name = f"archive_{timestamp}.zip"
            
            zip_path, err = create_zip(files, args.preserve_path, base_dir, tmp_to_filename)
            if err:
                output_error(err, args.quiet)
            
            try:
                # Auto-generated name already has timestamp
                oss_key = generate_oss_key(name, oss_prefix, args.no_timestamp or auto_generated)
                bucket.put_object_from_file(oss_key, zip_path)
                
                url = get_file_url(config, oss_key)
                files_included = []
                for f in files:
                    if f in tmp_to_filename:
                        files_included.append(tmp_to_filename[f])
                    elif args.preserve_path and base_dir:
                        try:
                            files_included.append(os.path.relpath(f, base_dir))
                        except ValueError:
                            files_included.append(Path(f).name)
                    else:
                        files_included.append(Path(f).name)
                
                output_result({
                    "success": True,
                    "mode": "zip",
                    "zip_name": name,
                    "files_included": files_included,
                    "url": url
                }, args.quiet)
            finally:
                os.unlink(zip_path)
        else:
            # Separate upload mode
            results = []
            for file_path in files:
                # Use original filename for downloaded files
                display_name = Path(file_path).name
                key_name = file_path
                if file_path in tmp_to_filename:
                    display_name = tmp_to_filename[file_path]
                    key_name = tmp_to_filename[file_path]
                
                oss_key = generate_oss_key(key_name, oss_prefix, args.no_timestamp)
                bucket.put_object_from_file(oss_key, file_path)
                
                url = get_file_url(config, oss_key)
                results.append({"file": display_name, "url": url})
            
            output_result({
                "success": True,
                "mode": "separate",
                "results": results
            }, args.quiet)
    
    finally:
        cleanup()


if __name__ == '__main__':
    main()
