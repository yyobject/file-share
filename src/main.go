package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type Config struct {
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	BucketName      string `json:"bucket_name"`
	Endpoint        string `json:"endpoint"`
	Domain          string `json:"domain,omitempty"`
	Prefix          string `json:"prefix,omitempty"`
}

// Version number
var Version = "1.2.0"

type UploadResult struct {
	File string `json:"file"`
	URL  string `json:"url"`
}

type Result struct {
	Success       bool           `json:"success"`
	Mode          string         `json:"mode"`
	ZipName       string         `json:"zip_name,omitempty"`
	FilesIncluded []string       `json:"files_included,omitempty"`
	URL           string         `json:"url,omitempty"`
	Results       []UploadResult `json:"results,omitempty"`
	Error         string         `json:"error,omitempty"`
}

type CheckResult struct {
	Ready        bool            `json:"ready"`
	EnvVars      map[string]bool `json:"env_vars"`
	OptionalVars map[string]bool `json:"optional_vars"`
	Missing      []string        `json:"missing"`
	Suggestions  []string        `json:"suggestions"`
	ConfigSource string          `json:"config_source,omitempty"`
}

// Get skill directory path
func getSkillDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	// Binary is in bin/, skill directory is its parent
	return filepath.Dir(filepath.Dir(exe))
}

// Parse .env file
func loadEnvFile(path string) map[string]string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes
			value = strings.Trim(value, `"'`)
			env[key] = value
		}
	}
	return env
}

// Create config from .env file
func configFromEnvFile(env map[string]string) *Config {
	if env == nil {
		return nil
	}
	return &Config{
		AccessKeyID:     env["OSS_ACCESS_KEY_ID"],
		AccessKeySecret: env["OSS_ACCESS_KEY_SECRET"],
		BucketName:      env["OSS_BUCKET_NAME"],
		Endpoint:        env["OSS_ENDPOINT"],
		Domain:          env["OSS_DOMAIN"],
		Prefix:          env["OSS_PREFIX"],
	}
}

// Merge configs, high priority overrides low
func mergeConfig(low, high *Config) *Config {
	if low == nil {
		return high
	}
	if high == nil {
		return low
	}
	result := *low
	if high.AccessKeyID != "" {
		result.AccessKeyID = high.AccessKeyID
	}
	if high.AccessKeySecret != "" {
		result.AccessKeySecret = high.AccessKeySecret
	}
	if high.BucketName != "" {
		result.BucketName = high.BucketName
	}
	if high.Endpoint != "" {
		result.Endpoint = high.Endpoint
	}
	if high.Domain != "" {
		result.Domain = high.Domain
	}
	if high.Prefix != "" {
		result.Prefix = high.Prefix
	}
	return &result
}

// Get config sources description
func getConfigSources() []string {
	var sources []string

	// 1. User home directory ~/.oss-upload.env
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".oss-upload.env")
		if _, err := os.Stat(path); err == nil {
			sources = append(sources, path)
		}
	}

	// 2. Skill directory .env
	if skillDir := getSkillDir(); skillDir != "" {
		path := filepath.Join(skillDir, ".env")
		if _, err := os.Stat(path); err == nil {
			sources = append(sources, path)
		}
	}

	// 3. Current directory .env
	if _, err := os.Stat(".env"); err == nil {
		sources = append(sources, ".env (current dir)")
	}

	// 4. Environment variables
	if os.Getenv("OSS_ACCESS_KEY_ID") != "" {
		sources = append(sources, "environment variables")
	}

	return sources
}

func getConfig() (*Config, error) {
	var config *Config

	// If custom config file specified, use it
	if customConfigPath != "" {
		envConfig := configFromEnvFile(loadEnvFile(customConfigPath))
		if envConfig == nil {
			return nil, fmt.Errorf("cannot read config file: %s", customConfigPath)
		}
		config = envConfig
	} else {
		// Load configs from low to high priority

		// 1. User home directory ~/.oss-upload.env (lowest priority)
		if home, err := os.UserHomeDir(); err == nil {
			envConfig := configFromEnvFile(loadEnvFile(filepath.Join(home, ".oss-upload.env")))
			config = mergeConfig(config, envConfig)
		}

		// 2. Skill directory .env
		if skillDir := getSkillDir(); skillDir != "" {
			envConfig := configFromEnvFile(loadEnvFile(filepath.Join(skillDir, ".env")))
			config = mergeConfig(config, envConfig)
		}

		// 3. Current working directory .env
		envConfig := configFromEnvFile(loadEnvFile(".env"))
		config = mergeConfig(config, envConfig)
	}

	// 4. Environment variables (highest priority, always check)
	envVarConfig := &Config{
		AccessKeyID:     os.Getenv("OSS_ACCESS_KEY_ID"),
		AccessKeySecret: os.Getenv("OSS_ACCESS_KEY_SECRET"),
		BucketName:      os.Getenv("OSS_BUCKET_NAME"),
		Endpoint:        os.Getenv("OSS_ENDPOINT"),
		Domain:          os.Getenv("OSS_DOMAIN"),
		Prefix:          os.Getenv("OSS_PREFIX"),
	}
	config = mergeConfig(config, envVarConfig)

	// Check required fields
	if config == nil {
		config = &Config{}
	}

	var missing []string
	if config.AccessKeyID == "" {
		missing = append(missing, "access_key_id")
	}
	if config.AccessKeySecret == "" {
		missing = append(missing, "access_key_secret")
	}
	if config.BucketName == "" {
		missing = append(missing, "bucket_name")
	}
	if config.Endpoint == "" {
		missing = append(missing, "endpoint")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing config: %s", strings.Join(missing, ", "))
	}

	return config, nil
}

func checkEnv() {
	// Try to get config
	config, _ := getConfig()

	result := CheckResult{
		Ready:        true,
		EnvVars:      map[string]bool{},
		OptionalVars: map[string]bool{},
		Missing:      []string{},
		Suggestions:  []string{},
	}

	// Check required config items
	if config != nil {
		result.EnvVars["access_key_id"] = config.AccessKeyID != ""
		result.EnvVars["access_key_secret"] = config.AccessKeySecret != ""
		result.EnvVars["bucket_name"] = config.BucketName != ""
		result.EnvVars["endpoint"] = config.Endpoint != ""
		result.OptionalVars["domain"] = config.Domain != ""
		result.OptionalVars["prefix"] = config.Prefix != ""
	} else {
		result.EnvVars["access_key_id"] = false
		result.EnvVars["access_key_secret"] = false
		result.EnvVars["bucket_name"] = false
		result.EnvVars["endpoint"] = false
		result.OptionalVars["domain"] = false
		result.OptionalVars["prefix"] = false
	}

	for k, v := range result.EnvVars {
		if !v {
			result.Ready = false
			result.Missing = append(result.Missing, k)
		}
	}

	// Show found config sources
	sources := getConfigSources()
	if len(sources) > 0 {
		result.ConfigSource = strings.Join(sources, " -> ")
	}

	if len(result.Missing) > 0 {
		result.Suggestions = append(result.Suggestions,
			fmt.Sprintf("Missing config: %s", strings.Join(result.Missing, ", ")))
		result.Suggestions = append(result.Suggestions,
			"Configuration methods (priority low to high):")
		result.Suggestions = append(result.Suggestions,
			"  1. ~/.oss-upload.env (global)")
		result.Suggestions = append(result.Suggestions,
			"  2. <skill-dir>/.env")
		result.Suggestions = append(result.Suggestions,
			"  3. .env (current directory)")
		result.Suggestions = append(result.Suggestions,
			"  4. Environment variables (OSS_ACCESS_KEY_ID, etc.)")
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(output))

	if result.Ready {
		fmt.Fprintln(os.Stderr, "\n✅ Ready to upload")
		os.Exit(0)
	} else {
		fmt.Fprintln(os.Stderr, "\n❌ Not ready, please configure as suggested")
		os.Exit(1)
	}
}

func generateOSSKey(filename, prefix string, noTimestamp bool) string {
	base := filepath.Base(filename)
	var key string

	if noTimestamp {
		key = base
	} else {
		timestamp := time.Now().Format("20060102_150405")
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		key = fmt.Sprintf("%s_%s%s", name, timestamp, ext)
	}

	if prefix != "" {
		key = strings.Trim(prefix, "/") + "/" + key
	}
	return key
}

func getFileURL(config *Config, ossKey string) string {
	if config.Domain != "" {
		domain := strings.TrimRight(config.Domain, "/")
		// Add https:// if no protocol specified
		if !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
			domain = "https://" + domain
		}
		return fmt.Sprintf("%s/%s", domain, ossKey)
	}

	endpoint := config.Endpoint
	scheme := "https"
	rest := endpoint

	if strings.HasPrefix(endpoint, "http://") {
		scheme = "http"
		rest = strings.TrimPrefix(endpoint, "http://")
	} else if strings.HasPrefix(endpoint, "https://") {
		scheme = "https"
		rest = strings.TrimPrefix(endpoint, "https://")
	}

	return fmt.Sprintf("%s://%s.%s/%s", scheme, config.BucketName, rest, ossKey)
}

func createZip(files []string, zipName string, preservePath bool, baseDir string, filenameMap map[string]string) (string, error) {
	tmpFile, err := os.CreateTemp("", "oss-upload-*.zip")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()

	// Use closure for cleanup
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	zipWriter := zip.NewWriter(tmpFile)

	for _, file := range files {
		if err := addFileToZip(zipWriter, file, preservePath, baseDir, filenameMap); err != nil {
			zipWriter.Close()
			tmpFile.Close()
			return "", err
		}
	}

	if err := zipWriter.Close(); err != nil {
		tmpFile.Close()
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	success = true
	return tmpPath, nil
}

func addFileToZip(zw *zip.Writer, filePath string, preservePath bool, baseDir string, filenameMap map[string]string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	// Determine filename in zip
	// First check if there's a custom name (for downloaded files)
	if customName, ok := filenameMap[filePath]; ok {
		header.Name = customName
	} else if preservePath && baseDir != "" {
		// Preserve relative path
		relPath, err := filepath.Rel(baseDir, filePath)
		if err == nil {
			header.Name = relPath
		} else {
			header.Name = filepath.Base(filePath)
		}
	} else {
		header.Name = filepath.Base(filePath)
	}
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, f)
	return err
}

func expandFiles(patterns []string, recursive bool) ([]string, error) {
	var files []string
	for _, pattern := range patterns {
		// Check if it's a directory
		info, err := os.Stat(pattern)
		if err == nil && info.IsDir() {
			if recursive {
				// Walk directory recursively
				err := filepath.Walk(pattern, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() {
						files = append(files, path)
					}
					return nil
				})
				if err != nil {
					return nil, fmt.Errorf("failed to walk directory %s: %v", pattern, err)
				}
			} else {
				return nil, fmt.Errorf("%s is a directory, use --recursive to upload recursively", pattern)
			}
			continue
		}

		if strings.ContainsAny(pattern, "*?") {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, err
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("no files matched: %s", pattern)
			}
			// Filter out directories
			for _, m := range matches {
				info, err := os.Stat(m)
				if err == nil && !info.IsDir() {
					files = append(files, m)
				}
			}
		} else {
			if _, err := os.Stat(pattern); os.IsNotExist(err) {
				return nil, fmt.Errorf("file not found: %s", pattern)
			}
			files = append(files, pattern)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files to upload")
	}
	return files, nil
}

// Check if string is a URL
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// Get filename from URL
func getFilenameFromURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "downloaded_file"
	}
	path := u.Path
	if path == "" || path == "/" {
		return "downloaded_file"
	}
	filename := filepath.Base(path)
	// Remove query string if present in filename
	if idx := strings.Index(filename, "?"); idx != -1 {
		filename = filename[:idx]
	}
	if filename == "" || filename == "." {
		return "downloaded_file"
	}
	return filename
}

// Download URL to temporary file
func downloadURL(urlStr string) (string, string, error) {
	resp, err := http.Get(urlStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to download %s: %v", urlStr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to download %s: HTTP %d", urlStr, resp.StatusCode)
	}

	// Get filename from URL or Content-Disposition header
	filename := getFilenameFromURL(urlStr)

	// Try to get filename from Content-Disposition header
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if idx := strings.Index(cd, "filename="); idx != -1 {
			fn := cd[idx+9:]
			fn = strings.Trim(fn, `"'`)
			if fn != "" {
				filename = fn
			}
		}
	}

	// Create temp file with same extension
	ext := filepath.Ext(filename)
	tmpFile, err := os.CreateTemp("", "oss-download-*"+ext)
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()

	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("failed to save downloaded file: %v", err)
	}

	return tmpPath, filename, nil
}

// DownloadedFile holds info about a downloaded URL
type DownloadedFile struct {
	TmpPath  string
	Filename string
	OrigURL  string
}

// Global quiet mode flag
var quietMode bool

func outputResult(result Result) {
	if quietMode {
		// Quiet mode: only output URLs
		if result.Mode == "zip" {
			fmt.Println(result.URL)
		} else {
			for _, r := range result.Results {
				fmt.Println(r.URL)
			}
		}
	} else {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
	}
}

func outputError(err error) {
	if quietMode {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
	} else {
		result := Result{
			Success: false,
			Error:   err.Error(),
		}
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(os.Stderr, string(output))
	}
	os.Exit(1)
}

// Custom config file path
var customConfigPath string

func main() {
	zipMode := flag.Bool("zip", false, "Bundle files into zip before upload")
	zipName := flag.String("zip-name", "", "Zip filename (auto-generated by default)")
	prefix := flag.String("prefix", "", "OSS path prefix (overrides config)")
	check := flag.Bool("check", false, "Check configuration")
	version := flag.Bool("version", false, "Show version")
	quiet := flag.Bool("quiet", false, "Quiet mode, only output URLs")
	noTimestamp := flag.Bool("no-timestamp", false, "Don't add timestamp suffix")
	recursive := flag.Bool("recursive", false, "Recursively upload directory")
	dryRun := flag.Bool("dry-run", false, "Preview mode, only show file list")
	configFile := flag.String("config", "", "Specify config file path")
	preservePath := flag.Bool("preserve-path", false, "Preserve directory structure when zipping")
	flag.Parse()

	// Version
	if *version {
		fmt.Printf("file-share version %s\n", Version)
		os.Exit(0)
	}

	// Set global variables
	quietMode = *quiet
	customConfigPath = *configFile

	// Check mode
	if *check {
		checkEnv()
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: file-share [options] <files/URLs...>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  --check           Check configuration")
		fmt.Fprintln(os.Stderr, "  --zip             Bundle files into zip before upload")
		fmt.Fprintln(os.Stderr, "  --zip-name NAME   Specify zip filename")
		fmt.Fprintln(os.Stderr, "  --prefix PATH     OSS path prefix")
		fmt.Fprintln(os.Stderr, "  --no-timestamp    Don't add timestamp suffix")
		fmt.Fprintln(os.Stderr, "  --recursive       Recursively upload directory")
		fmt.Fprintln(os.Stderr, "  --preserve-path   Preserve directory structure when zipping")
		fmt.Fprintln(os.Stderr, "  --quiet           Quiet mode, only output URLs")
		fmt.Fprintln(os.Stderr, "  --dry-run         Preview mode, only show file list")
		fmt.Fprintln(os.Stderr, "  --config FILE     Specify config file path")
		fmt.Fprintln(os.Stderr, "  --version         Show version")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "URL Support:")
		fmt.Fprintln(os.Stderr, "  Pass URLs (http:// or https://) to download and re-upload to OSS")
		fmt.Fprintln(os.Stderr, "  Example: oss-upload https://example.com/image.png")
		os.Exit(1)
	}

	// Separate URLs from local files
	var localArgs []string
	var downloadedFiles []DownloadedFile

	for _, arg := range args {
		if isURL(arg) {
			if !quietMode {
				fmt.Fprintf(os.Stderr, "Downloading: %s\n", arg)
			}
			tmpPath, filename, err := downloadURL(arg)
			if err != nil {
				outputError(err)
			}
			downloadedFiles = append(downloadedFiles, DownloadedFile{
				TmpPath:  tmpPath,
				Filename: filename,
				OrigURL:  arg,
			})
		} else {
			localArgs = append(localArgs, arg)
		}
	}

	// Clean up downloaded files on exit
	defer func() {
		for _, df := range downloadedFiles {
			os.Remove(df.TmpPath)
		}
	}()

	// Expand local file list
	var files []string
	if len(localArgs) > 0 {
		var err error
		files, err = expandFiles(localArgs, *recursive)
		if err != nil {
			outputError(err)
		}
	}

	// Build a map from temp path to original filename for downloaded files
	tmpToFilename := make(map[string]string)
	for _, df := range downloadedFiles {
		files = append(files, df.TmpPath)
		tmpToFilename[df.TmpPath] = df.Filename
	}

	if len(files) == 0 {
		outputError(fmt.Errorf("no files to upload"))
	}

	// Preview mode
	if *dryRun {
		fmt.Printf("Will upload %d files:\n", len(files))
		for _, f := range files {
			// Show original filename for downloaded files
			if origName, ok := tmpToFilename[f]; ok {
				fmt.Printf("  %s (from URL)\n", origName)
			} else {
				fmt.Printf("  %s\n", f)
			}
		}
		if *zipMode {
			name := *zipName
			if name == "" {
				name = "archive_<timestamp>.zip"
			}
			fmt.Printf("\nZip mode: will bundle as %s\n", name)
		}
		os.Exit(0)
	}

	// Get config
	config, err := getConfig()
	if err != nil {
		outputError(err)
	}

	// Create OSS client
	client, err := oss.New(config.Endpoint, config.AccessKeyID, config.AccessKeySecret)
	if err != nil {
		outputError(fmt.Errorf("failed to create OSS client: %v", err))
	}

	bucket, err := client.Bucket(config.BucketName)
	if err != nil {
		outputError(fmt.Errorf("failed to get bucket: %v", err))
	}

	// Determine prefix
	ossPrefix := config.Prefix
	if *prefix != "" {
		ossPrefix = *prefix
	}

	// Calculate base directory (for preserving paths)
	baseDir := ""
	if *preservePath && len(args) > 0 {
		// Use first argument as base directory
		info, err := os.Stat(args[0])
		if err == nil && info.IsDir() {
			baseDir = args[0]
		} else {
			baseDir = filepath.Dir(args[0])
		}
	}

	if *zipMode {
		// Zip upload mode
		name := *zipName
		autoGenerated := name == ""
		if autoGenerated {
			timestamp := time.Now().Format("20060102_150405")
			name = fmt.Sprintf("archive_%s.zip", timestamp)
		}

		zipPath, err := createZip(files, name, *preservePath, baseDir, tmpToFilename)
		if err != nil {
			outputError(fmt.Errorf("failed to create zip: %v", err))
		}
		defer os.Remove(zipPath)

		// Auto-generated name already has timestamp, skip adding another
		ossKey := generateOSSKey(name, ossPrefix, *noTimestamp || autoGenerated)
		err = bucket.PutObjectFromFile(ossKey, zipPath)
		if err != nil {
			outputError(fmt.Errorf("upload failed: %v", err))
		}

		url := getFileURL(config, ossKey)
		filesIncluded := make([]string, len(files))
		for i, f := range files {
			// Use original filename for downloaded files
			if origName, ok := tmpToFilename[f]; ok {
				filesIncluded[i] = origName
			} else if *preservePath && baseDir != "" {
				relPath, err := filepath.Rel(baseDir, f)
				if err == nil {
					filesIncluded[i] = relPath
				} else {
					filesIncluded[i] = filepath.Base(f)
				}
			} else {
				filesIncluded[i] = filepath.Base(f)
			}
		}

		outputResult(Result{
			Success:       true,
			Mode:          "zip",
			ZipName:       name,
			FilesIncluded: filesIncluded,
			URL:           url,
		})
	} else {
		// Separate upload mode
		var results []UploadResult
		for _, file := range files {
			// Use original filename for downloaded files
			displayName := filepath.Base(file)
			keyName := file
			if origName, ok := tmpToFilename[file]; ok {
				displayName = origName
				keyName = origName
			}

			ossKey := generateOSSKey(keyName, ossPrefix, *noTimestamp)
			err = bucket.PutObjectFromFile(ossKey, file)
			if err != nil {
				outputError(fmt.Errorf("failed to upload %s: %v", displayName, err))
			}

			url := getFileURL(config, ossKey)
			results = append(results, UploadResult{
				File: displayName,
				URL:  url,
			})
		}

		outputResult(Result{
			Success: true,
			Mode:    "separate",
			Results: results,
		})
	}
}
