### ✨ Features:

**📤 Upload Options:**
- **Password protection**: `-p/--password`
- **File descriptions**: `-d/--description`
- **Tagging system**: `--tags`
- **Download limits**: `-m/--max-downloads`
- **Interactive mode**: `-i/--interactive`

**🔍 Management Commands:**
- `./upload.sh list` - List all files with details
- `./upload.sh stats` - Show server statistics
- `./upload.sh search <query>` - Search files
- `./upload.sh info <file_id>` - Get detailed file info
- `./upload.sh download <file_id> [password]` - Download files
- `./upload.sh delete <file_id>` - Delete files
- `./upload.sh health` - Check server health

**🎨 Better User Experience:**
- **Colored output** with emojis
- **Formatted file sizes** (KB, MB, GB)
- **Duration formatting** (hours, minutes, seconds)
- **Pretty JSON parsing** with Python3
- **Clipboard integration** for download URLs
- **Server connectivity checks**

### 🚀 Usage Examples:

```bash
# Basic upload
./upload.sh myfile.txt

# Advanced upload with all options
./upload.sh document.pdf -t 7200 -p secret123 -d "Important contract" --tags "legal,urgent" -m 5

# Interactive mode (prompts for all options)
./upload.sh myfile.txt --interactive

# Management commands
./upload.sh list                    # List all files
./upload.sh stats                   # Server statistics
./upload.sh search "contract"       # Search files
./upload.sh info abc123def456       # File details
./upload.sh download abc123def456   # Download file
./upload.sh delete abc123def456     # Delete file
./upload.sh health                  # Health check

# Environment variables
export UPLOAD_SERVER="http://my-server:8080"
export UPLOAD_TTL="7200"
./upload.sh myfile.txt
```

### 🔧 Requirements:
- **Python3** (for JSON parsing)
- **curl** (for HTTP requests)
- **Clipboard tools** (optional): `pbcopy` (macOS), `xclip` (Linux), `clip` (Windows)

### 📋 Features Demonstration:

**Upload with all features:**
```bash
./upload.sh contract.pdf \
  --password "secret123" \
  --description "Legal contract for review" \
  --tags "legal,important,confidential" \
  --max-downloads 3 \
  --ttl 86400
```

**File management:**
```bash
# List files with filtering and search
./upload.sh list
./upload.sh search "legal"

# Get detailed info and download
./upload.sh info abc123def456
./upload.sh download abc123def456 secret123
```

The script now provides a complete command-line interface for your enhanced file service, with beautiful output formatting and full feature support! 🎉