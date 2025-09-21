# README.md

# Enhanced File Upload Service

A powerful, feature-rich file upload service written in Go with persistent storage, advanced file management, and a modern web interface.

## 🚀 Key Features

### Core Functionality
- **File Upload & Download**: Secure file uploads with unique IDs and download links
- **Persistent Storage**: File metadata survives server restarts via JSON metadata file
- **Automatic Cleanup**: Configurable cleanup of expired files and those exceeding download limits
- **File Integrity**: SHA256 checksums for all uploaded files

### Advanced Features
- **Time-to-Live (TTL)**: Configurable expiration times for files
- **Download Limits**: Set maximum download counts per file
- **Password Protection**: Optional password protection for individual files
- **File Tagging**: Organize files with custom tags
- **Search & Filter**: Full-text search in filenames and descriptions, tag filtering
- **File Descriptions**: Add descriptions to uploaded files
- **Content Type Restrictions**: Optionally limit allowed file types

### Management & Monitoring
- **Web Interface**: Beautiful, responsive web UI for file management
- **Statistics Dashboard**: Real-time stats on uploads, downloads, and storage usage
- **Bulk Operations**: Delete multiple files at once
- **API Endpoints**: RESTful API for programmatic access
- **Health Checks**: Built-in health monitoring endpoint

## 📁 File Structure
```
.
├── main.go              # Main application file
├── config.json          # Configuration file (optional)
├── metadata.json        # File metadata (auto-generated)
└── files/             # Upload directory (auto-created)
    └── [uploaded files]
```

## ⚙️ Configuration

Create a `config.json` file in the same directory as the executable:

```json
{
  "port": "8080",
  "upload_dir": "./files",
  "metadata_file": "./metadata.json",
  "default_ttl": "3600000000000",
  "max_file_size": 104857600,
  "allowed_origins": ["*"],
  "cleanup_interval": "300000000000",
  "max_downloads": 0,
  "require_password": false,
  "admin_password": "",
  "allowed_types": ["image/", "text/", "application/pdf"]
}
```

### Configuration Options
- `port`: Server port (default: "8080")
- `upload_dir`: Directory for uploaded files (default: "./files")
- `metadata_file`: Path to metadata storage file (default: "./metadata.json")
- `default_ttl`: Default file expiration time in nanoseconds (default: 1 hour)
- `max_file_size`: Maximum file size in bytes (default: 100MB)
- `allowed_origins`: CORS origins (default: ["*"])
- `cleanup_interval`: How often to run cleanup in nanoseconds (default: 5 minutes)
- `max_downloads`: Default max downloads per file (0 = unlimited)
- `require_password`: Require password for all uploads
- `admin_password`: Admin password for management interface
- `allowed_types`: Allowed content types (empty = all types allowed)

## Example config.json
```json
{
"port": "8080",
"upload_dir": "./files",
"metadata_file": "./metadata.json",
"default_ttl": "3600000000000",
"max_file_size": 104857600,
"allowed_origins": ["*"],
"cleanup_interval": "300000000000",
"max_downloads": 0,
"require_password": false,
"admin_password": "",
"allowed_types": ["image/", "text/", "application/pdf"]
}
```

## 🚦 Quick Start

1. **Build and run**:
```bash
go build -o fileserver main.go
./fileserver
```

2. **Access the web interface**:
   Open `http://localhost:8080` in your browser

3. **Upload a file**:
- Use the web interface, or
- Use curl: `curl -F "file=@example.txt" http://localhost:8080/upload`

## 🔌 API Endpoints

### Upload File
```bash
POST /upload
Content-Type: multipart/form-data

Parameters:
- file: File to upload (required)
- ttl: Time to live in seconds (optional)
- max_downloads: Maximum download count (optional)
- password: Password protection (optional)
- description: File description (optional)
- tags: Comma-separated tags (optional)
```

### Download File
```bash
GET /download/{fileID}?password={password}
```

### File Information
```bash
GET /info/{fileID}
```

### Search Files
```bash
GET /search?q={query}&tag={tag}&sort={field}
```

### Statistics
```bash
GET /stats
```

### API Endpoints
```bash
GET /api/files?limit={limit}&offset={offset}  # List files with pagination
GET /api/health                               # Health check
POST /api/upload                              # Upload via API
```

### Bulk Operations
```bash
POST /bulk-delete
Content-Type: application/json
{
  "file_ids": ["id1", "id2", "id3"]
}
```

## 📊 Web Interface Features

### Dashboard
- Real-time statistics (total files, active files, downloads, storage usage)
- File upload form with all options
- Search and filtering capabilities

### File Management
- Sortable file list with all metadata
- Visual indicators for expired files and those near download limits
- One-click download and delete operations
- File information including checksums, tags, and descriptions

### Enhanced UI
- Responsive design that works on all devices
- Color-coded file status (expired, near limit, etc.)
- Tag display and filtering
- Advanced search functionality

## 🔒 Security Features

- Unique file IDs prevent guessing download URLs
- Optional password protection per file
- File type restrictions
- Size limits
- Checksum verification
- Automatic cleanup of expired files

## 🎯 Use Cases

- **Temporary File Sharing**: Share files with automatic expiration
- **Download Tracking**: Monitor how many times files are downloaded
- **Secure Sharing**: Password-protected file sharing
- **File Organization**: Tag and categorize uploads
- **API Integration**: Integrate with other applications via REST API
- **Backup & Archive**: Temporary storage with automatic cleanup

## 📈 Monitoring

The service provides several monitoring endpoints:

- `/stats` - Upload statistics and storage metrics
- `/api/health` - Service health status
- `/manage` - Web-based management interface

## 🛠️ Development

To extend the service:

1. **Add new routes** in the `main()` function
2. **Extend FileInfo struct** for additional metadata
3. **Modify the HTML template** for UI changes
4. **Add new API endpoints** in the `apiHandler` function

## 📝 License

This is a sample implementation. Modify and use as needed for your projects.

## 🤝 Contributing

Feel free to submit issues and enhancement requests!