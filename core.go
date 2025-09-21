package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Port            string        `json:"port"`
	UploadDir       string        `json:"upload_dir"`
	MetadataFile    string        `json:"metadata_file"`
	DefaultTTL      time.Duration `json:"default_ttl"`
	MaxFileSize     int64         `json:"max_file_size"`
	AllowedOrigins  []string      `json:"allowed_origins"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	MaxDownloads    int           `json:"max_downloads"`
	RequirePassword bool          `json:"require_password"`
	AdminPassword   string        `json:"admin_password"`
	AllowedTypes    []string      `json:"allowed_types"`
}

type FileInfo struct {
	ID           string            `json:"id"`
	Filename     string            `json:"filename"`
	OriginalName string            `json:"original_name"`
	Size         int64             `json:"size"`
	ContentType  string            `json:"content_type"`
	Checksum     string            `json:"checksum"`
	UploadTime   time.Time         `json:"upload_time"`
	ExpiresAt    time.Time         `json:"expires_at"`
	Downloads    int               `json:"downloads"`
	MaxDownloads int               `json:"max_downloads"`
	Password     string            `json:"password,omitempty"`
	UploaderIP   string            `json:"uploader_ip"`
	Tags         []string          `json:"tags"`
	Description  string            `json:"description"`
	Path         string            `json:"path"`
	Metadata     map[string]string `json:"metadata"`
}

type FileManager struct {
	config Config
	files  map[string]*FileInfo
	mutex  sync.RWMutex
}

type UploadStats struct {
	TotalFiles     int   `json:"total_files"`
	TotalSize      int64 `json:"total_size"`
	TotalDownloads int   `json:"total_downloads"`
	ActiveFiles    int   `json:"active_files"`
}

func NewFileManager(config Config) *FileManager {
	fm := &FileManager{
		config: config,
		files:  make(map[string]*FileInfo),
	}

	// Load existing file metadata
	fm.loadMetadata()

	// Start cleanup routine
	go fm.cleanupRoutine()

	// Save metadata periodically
	go fm.saveMetadataPeriodically()

	return fm
}

func (fm *FileManager) loadMetadata() {
	data, err := os.ReadFile(fm.config.MetadataFile)
	if err != nil {
		log.Printf("No existing metadata file found, starting fresh")
		return
	}

	var files map[string]*FileInfo
	if err := json.Unmarshal(data, &files); err != nil {
		log.Printf("Error loading metadata: %v", err)
		return
	}

	// Verify files still exist on disk
	validFiles := make(map[string]*FileInfo)
	for id, fileInfo := range files {
		if _, err := os.Stat(fileInfo.Path); err == nil {
			validFiles[id] = fileInfo
		} else {
			log.Printf("File not found on disk, removing from metadata: %s", fileInfo.Filename)
		}
	}

	fm.files = validFiles
	log.Printf("Loaded %d files from metadata", len(fm.files))
}

func (fm *FileManager) saveMetadata() error {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()

	data, err := json.MarshalIndent(fm.files, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(fm.config.MetadataFile, data, 0644)
}

func (fm *FileManager) saveMetadataPeriodically() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := fm.saveMetadata(); err != nil {
			log.Printf("Error saving metadata: %v", err)
		}
	}
}

func (fm *FileManager) cleanupRoutine() {
	ticker := time.NewTicker(fm.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		fm.cleanup()
	}
}

func (fm *FileManager) cleanup() {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	now := time.Now()
	cleaned := 0

	for id, fileInfo := range fm.files {
		shouldDelete := false

		// Check expiration
		if now.After(fileInfo.ExpiresAt) {
			shouldDelete = true
		}

		// Check max downloads
		if fileInfo.MaxDownloads > 0 && fileInfo.Downloads >= fileInfo.MaxDownloads {
			shouldDelete = true
		}

		if shouldDelete {
			// Delete file from disk
			if err := os.Remove(fileInfo.Path); err != nil {
				log.Printf("Error deleting file %s: %v", fileInfo.Path, err)
			}
			// Remove from memory
			delete(fm.files, id)
			cleaned++
			log.Printf("Cleaned up file: %s (reason: %s)", fileInfo.Filename,
				func() string {
					if now.After(fileInfo.ExpiresAt) {
						return "expired"
					}
					return "max downloads reached"
				}())
		}
	}

	if cleaned > 0 {
		fm.saveMetadata()
	}
}

func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func calculateChecksum(file io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (fm *FileManager) uploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	err := r.ParseMultipartForm(fm.config.MaxFileSize)
	if err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Check file type if restricted
	if len(fm.config.AllowedTypes) > 0 {
		contentType := header.Header.Get("Content-Type")
		allowed := false
		for _, allowedType := range fm.config.AllowedTypes {
			if strings.Contains(contentType, allowedType) {
				allowed = true
				break
			}
		}
		if !allowed {
			http.Error(w, "File type not allowed", http.StatusBadRequest)
			return
		}
	}

	// Get parameters from form
	ttlStr := r.FormValue("ttl")
	maxDownloadsStr := r.FormValue("max_downloads")
	password := r.FormValue("password")
	description := r.FormValue("description")
	tagsStr := r.FormValue("tags")

	// Parse TTL
	var ttl time.Duration
	if ttlStr != "" {
		if ttlInt, err := strconv.Atoi(ttlStr); err == nil {
			ttl = time.Duration(ttlInt) * time.Second
		} else {
			ttl = fm.config.DefaultTTL
		}
	} else {
		ttl = fm.config.DefaultTTL
	}

	// Parse max downloads
	var maxDownloads int
	if maxDownloadsStr != "" {
		if md, err := strconv.Atoi(maxDownloadsStr); err == nil {
			maxDownloads = md
		}
	}

	// Parse tags
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(strings.ReplaceAll(tagsStr, " ", ""), ",")
	}

	// Generate unique ID and filename
	fileID := generateID()
	safeFilename := strings.ReplaceAll(header.Filename, " ", "_")
	storedFilename := fileID + "_" + safeFilename

	// Calculate checksum
	tempFile, err := os.CreateTemp("", "upload_*")
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Copy file for checksum calculation
	fileSize, err := io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Reset file pointer for checksum
	tempFile.Seek(0, 0)
	checksum, err := calculateChecksum(tempFile)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Create file info
	fileInfo := &FileInfo{
		ID:           fileID,
		Filename:     safeFilename,
		OriginalName: header.Filename,
		Size:         fileSize,
		ContentType:  header.Header.Get("Content-Type"),
		Checksum:     checksum,
		UploadTime:   time.Now(),
		ExpiresAt:    time.Now().Add(ttl),
		Downloads:    0,
		MaxDownloads: maxDownloads,
		Password:     password,
		UploaderIP:   r.RemoteAddr,
		Tags:         tags,
		Description:  description,
		Path:         filepath.Join(fm.config.UploadDir, storedFilename),
		Metadata:     make(map[string]string),
	}

	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(fm.config.UploadDir, 0755); err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Move temp file to final location
	tempFile.Seek(0, 0)
	dst, err := os.Create(fileInfo.Path)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, tempFile)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Store file info
	fm.mutex.Lock()
	fm.files[fileID] = fileInfo
	fm.mutex.Unlock()

	// Save metadata immediately for new uploads
	fm.saveMetadata()

	// Return response
	downloadURL := fmt.Sprintf("http://%s/download/%s", r.Host, fileID)

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"id":            fileID,
			"filename":      fileInfo.Filename,
			"original_name": fileInfo.OriginalName,
			"size":          fileInfo.Size,
			"checksum":      fileInfo.Checksum,
			"download_url":  downloadURL,
			"expires_at":    fileInfo.ExpiresAt.Format(time.RFC3339),
			"max_downloads": fileInfo.MaxDownloads,
		}
		json.NewEncoder(w).Encode(response)
	} else {
		fmt.Fprintf(w, "File uploaded successfully!\n\nDownload URL: %s\nExpires: %s\nChecksum: %s\n",
			downloadURL, fileInfo.ExpiresAt.Format("2006-01-02 15:04:05"), fileInfo.Checksum)
	}
}

func (fm *FileManager) downloadFile(w http.ResponseWriter, r *http.Request) {
	fileID := strings.TrimPrefix(r.URL.Path, "/download/")
	password := r.URL.Query().Get("password")

	fm.mutex.RLock()
	fileInfo, exists := fm.files[fileID]
	fm.mutex.RUnlock()

	if !exists {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Check password if required
	if fileInfo.Password != "" && fileInfo.Password != password {
		http.Error(w, "Password required", http.StatusUnauthorized)
		return
	}

	// Check expiration
	if time.Now().After(fileInfo.ExpiresAt) {
		fm.mutex.Lock()
		delete(fm.files, fileID)
		fm.mutex.Unlock()
		os.Remove(fileInfo.Path)
		fm.saveMetadata()
		http.Error(w, "File expired", http.StatusNotFound)
		return
	}

	// Check max downloads
	if fileInfo.MaxDownloads > 0 && fileInfo.Downloads >= fileInfo.MaxDownloads {
		http.Error(w, "Download limit reached", http.StatusForbidden)
		return
	}

	// Increment download counter
	fm.mutex.Lock()
	fileInfo.Downloads++
	fm.mutex.Unlock()

	// Serve file
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileInfo.OriginalName))
	w.Header().Set("Content-Type", fileInfo.ContentType)
	w.Header().Set("X-Checksum", fileInfo.Checksum)
	http.ServeFile(w, r, fileInfo.Path)

	// Save metadata after download
	go fm.saveMetadata()
}

func (fm *FileManager) searchFiles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	tag := r.URL.Query().Get("tag")
	sortBy := r.URL.Query().Get("sort")

	fm.mutex.RLock()
	var matchingFiles []*FileInfo
	for _, fileInfo := range fm.files {
		matches := true

		// Text search in filename and description
		if query != "" {
			matches = matches && (strings.Contains(strings.ToLower(fileInfo.Filename), strings.ToLower(query)) ||
				strings.Contains(strings.ToLower(fileInfo.Description), strings.ToLower(query)))
		}

		// Tag filter
		if tag != "" {
			tagMatches := false
			for _, t := range fileInfo.Tags {
				if strings.EqualFold(t, tag) {
					tagMatches = true
					break
				}
			}
			matches = matches && tagMatches
		}

		if matches {
			matchingFiles = append(matchingFiles, fileInfo)
		}
	}
	fm.mutex.RUnlock()

	// Sort results
	switch sortBy {
	case "size":
		sort.Slice(matchingFiles, func(i, j int) bool {
			return matchingFiles[i].Size > matchingFiles[j].Size
		})
	case "downloads":
		sort.Slice(matchingFiles, func(i, j int) bool {
			return matchingFiles[i].Downloads > matchingFiles[j].Downloads
		})
	default: // default to upload time
		sort.Slice(matchingFiles, func(i, j int) bool {
			return matchingFiles[i].UploadTime.After(matchingFiles[j].UploadTime)
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matchingFiles)
}

func (fm *FileManager) getStats(w http.ResponseWriter, r *http.Request) {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()

	stats := UploadStats{}
	now := time.Now()

	for _, fileInfo := range fm.files {
		stats.TotalFiles++
		stats.TotalSize += fileInfo.Size
		stats.TotalDownloads += fileInfo.Downloads

		if now.Before(fileInfo.ExpiresAt) {
			stats.ActiveFiles++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (fm *FileManager) manageFiles(w http.ResponseWriter, r *http.Request) {
	fm.mutex.RLock()
	files := make([]*FileInfo, 0, len(fm.files))
	for _, fileInfo := range fm.files {
		files = append(files, fileInfo)
	}
	fm.mutex.RUnlock()

	// Sort by upload time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].UploadTime.After(files[j].UploadTime)
	})

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
		return
	}

	// HTML response with enhanced UI
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>File Management</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { border-bottom: 2px solid #007bff; padding-bottom: 10px; margin-bottom: 20px; }
        h1 { color: #007bff; margin: 0; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin-bottom: 20px; }
        .stat-card { background: #007bff; color: white; padding: 15px; border-radius: 5px; text-align: center; }
        .stat-value { font-size: 2em; font-weight: bold; }
        .stat-label { font-size: 0.9em; opacity: 0.9; }
        table { border-collapse: collapse; width: 100%; margin-top: 20px; }
        th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        th { background-color: #f8f9fa; font-weight: bold; position: sticky; top: 0; }
        .expired { background-color: #ffeeee; }
        .near-limit { background-color: #fff3cd; }
        .actions { white-space: nowrap; }
        .upload-form { margin-bottom: 30px; padding: 20px; background: #f8f9fa; border-radius: 5px; border-left: 4px solid #007bff; }
        .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 15px; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
        .form-group input, .form-group textarea { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; }
        .btn { background: #007bff; color: white; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; }
        .btn:hover { background: #0056b3; }
        .btn-danger { background: #dc3545; }
        .btn-danger:hover { background: #c82333; }
        .tags { display: flex; flex-wrap: wrap; gap: 5px; }
        .tag { background: #e9ecef; padding: 2px 8px; border-radius: 12px; font-size: 0.8em; }
        .search-form { margin: 20px 0; padding: 15px; background: #e9ecef; border-radius: 5px; }
        .checksum { font-family: monospace; font-size: 0.8em; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Enhanced File Upload Service</h1>
        </div>
        
        <div class="stats">
            <div class="stat-card">
                <div class="stat-value">{{.Stats.TotalFiles}}</div>
                <div class="stat-label">Total Files</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{{.Stats.ActiveFiles}}</div>
                <div class="stat-label">Active Files</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{{.Stats.TotalDownloads}}</div>
                <div class="stat-label">Total Downloads</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{{formatBytes .Stats.TotalSize}}</div>
                <div class="stat-label">Total Size</div>
            </div>
        </div>
        
        <div class="upload-form">
            <h2>Upload File</h2>
            <form action="/upload" method="post" enctype="multipart/form-data">
                <div class="form-grid">
                    <div class="form-group">
                        <label>File:</label>
                        <input type="file" name="file" required>
                    </div>
                    <div class="form-group">
                        <label>TTL (seconds):</label>
                        <input type="number" name="ttl" placeholder="Default: 3600">
                    </div>
                    <div class="form-group">
                        <label>Max Downloads:</label>
                        <input type="number" name="max_downloads" placeholder="Unlimited">
                    </div>
                    <div class="form-group">
                        <label>Password:</label>
                        <input type="password" name="password" placeholder="Optional">
                    </div>
                </div>
                <div class="form-group">
                    <label>Description:</label>
                    <textarea name="description" rows="2" placeholder="Optional description"></textarea>
                </div>
                <div class="form-group">
                    <label>Tags (comma-separated):</label>
                    <input type="text" name="tags" placeholder="e.g., document, important, temp">
                </div>
                <input type="submit" value="Upload File" class="btn">
            </form>
        </div>
        
        <div class="search-form">
            <h3>Search & Filter</h3>
            <form method="get">
                <div class="form-grid">
                    <div class="form-group">
                        <input type="text" name="q" placeholder="Search filename or description..." value="{{.Query}}">
                    </div>
                    <div class="form-group">
                        <input type="text" name="tag" placeholder="Filter by tag..." value="{{.TagFilter}}">
                    </div>
                </div>
                <input type="submit" value="Search" class="btn">
            </form>
        </div>
        
        <h2>Uploaded Files ({{len .Files}})</h2>
        <div style="overflow-x: auto;">
            <table>
                <tr>
                    <th>Filename</th>
                    <th>Description</th>
                    <th>Size</th>
                    <th>Type</th>
                    <th>Uploaded</th>
                    <th>Expires</th>
                    <th>Downloads</th>
                    <th>Tags</th>
                    <th>Checksum</th>
                    <th>Actions</th>
                </tr>
                {{range .Files}}
                <tr{{if .IsExpired}} class="expired"{{else if .NearLimit}} class="near-limit"{{end}}>
                    <td><strong>{{.OriginalName}}</strong></td>
                    <td>{{.Description}}</td>
                    <td>{{formatBytes .Size}}</td>
                    <td>{{.ContentType}}</td>
                    <td>{{.UploadTime.Format "2006-01-02 15:04:05"}}</td>
                    <td>{{.ExpiresAt.Format "2006-01-02 15:04:05"}}</td>
                    <td>{{.Downloads}}{{if gt .MaxDownloads 0}}/{{.MaxDownloads}}{{end}}</td>
                    <td>
                        <div class="tags">
                            {{range .Tags}}<span class="tag">{{.}}</span>{{end}}
                        </div>
                    </td>
                    <td class="checksum">{{substr .Checksum 0 12}}...</td>
                    <td class="actions">
                        <a href="/download/{{.ID}}{{if .Password}}?password={{.Password}}{{end}}" target="_blank" class="btn">Download</a>
                        <a href="/delete/{{.ID}}" onclick="return confirm('Delete this file?')" class="btn btn-danger">Delete</a>
                    </td>
                </tr>
                {{end}}
            </table>
        </div>
    </div>
</body>
</html>`

	t := template.Must(template.New("manage").Funcs(template.FuncMap{
		"formatBytes": func(bytes int64) string {
			const unit = 1024
			if bytes < unit {
				return fmt.Sprintf("%d B", bytes)
			}
			div, exp := int64(unit), 0
			for n := bytes / unit; n >= unit; n /= unit {
				div *= unit
				exp++
			}
			return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
		},
		"substr": func(s string, start, length int) string {
			if start >= len(s) {
				return ""
			}
			end := start + length
			if end > len(s) {
				end = len(s)
			}
			return s[start:end]
		},
	}).Parse(tmpl))

	type TemplateFile struct {
		*FileInfo
		IsExpired bool
		NearLimit bool
	}

	// Get stats
	fm.mutex.RLock()
	stats := UploadStats{}
	now := time.Now()
	for _, fileInfo := range fm.files {
		stats.TotalFiles++
		stats.TotalSize += fileInfo.Size
		stats.TotalDownloads += fileInfo.Downloads
		if now.Before(fileInfo.ExpiresAt) {
			stats.ActiveFiles++
		}
	}
	fm.mutex.RUnlock()

	templateFiles := make([]TemplateFile, len(files))
	for i, f := range files {
		isExpired := time.Now().After(f.ExpiresAt)
		nearLimit := f.MaxDownloads > 0 && f.Downloads >= f.MaxDownloads-1
		templateFiles[i] = TemplateFile{
			FileInfo:  f,
			IsExpired: isExpired,
			NearLimit: nearLimit && !isExpired,
		}
	}

	data := struct {
		Files     []TemplateFile
		Stats     UploadStats
		Query     string
		TagFilter string
	}{
		Files:     templateFiles,
		Stats:     stats,
		Query:     r.URL.Query().Get("q"),
		TagFilter: r.URL.Query().Get("tag"),
	}

	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, data)
}

func (fm *FileManager) deleteFile(w http.ResponseWriter, r *http.Request) {
	fileID := strings.TrimPrefix(r.URL.Path, "/delete/")

	fm.mutex.Lock()
	fileInfo, exists := fm.files[fileID]
	if exists {
		delete(fm.files, fileID)
	}
	fm.mutex.Unlock()

	if exists {
		os.Remove(fileInfo.Path)
		fm.saveMetadata()

		if strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
		} else {
			http.Redirect(w, r, "/manage", http.StatusSeeOther)
		}
	} else {
		http.Error(w, "File not found", http.StatusNotFound)
	}
}

func (fm *FileManager) fileInfo(w http.ResponseWriter, r *http.Request) {
	fileID := strings.TrimPrefix(r.URL.Path, "/info/")

	fm.mutex.RLock()
	fileInfo, exists := fm.files[fileID]
	fm.mutex.RUnlock()

	if !exists {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileInfo)
}

func (fm *FileManager) bulkDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		FileIDs []string `json:"file_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	deleted := 0
	fm.mutex.Lock()
	for _, fileID := range request.FileIDs {
		if fileInfo, exists := fm.files[fileID]; exists {
			os.Remove(fileInfo.Path)
			delete(fm.files, fileID)
			deleted++
		}
	}
	fm.mutex.Unlock()

	if deleted > 0 {
		fm.saveMetadata()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted": deleted,
		"total":   len(request.FileIDs),
	})
}

func (fm *FileManager) apiHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 {
		http.Error(w, "Invalid API endpoint", http.StatusNotFound)
		return
	}

	switch parts[0] {
	case "files":
		if r.Method == "GET" {
			fm.listFilesAPI(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case "upload":
		if r.Method == "POST" {
			fm.uploadFile(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case "health":
		fm.healthCheck(w, r)
	default:
		http.Error(w, "Unknown API endpoint", http.StatusNotFound)
	}
}

func (fm *FileManager) listFilesAPI(w http.ResponseWriter, r *http.Request) {
	limit := 50 // default limit
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	fm.mutex.RLock()
	files := make([]*FileInfo, 0, len(fm.files))
	for _, fileInfo := range fm.files {
		files = append(files, fileInfo)
	}
	fm.mutex.RUnlock()

	// Sort by upload time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].UploadTime.After(files[j].UploadTime)
	})

	// Apply pagination
	total := len(files)
	end := offset + limit
	if end > total {
		end = total
	}
	if offset >= total {
		files = []*FileInfo{}
	} else {
		files = files[offset:end]
	}

	response := map[string]interface{}{
		"files":  files,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (fm *FileManager) healthCheck(w http.ResponseWriter, r *http.Request) {
	fm.mutex.RLock()
	fileCount := len(fm.files)
	fm.mutex.RUnlock()

	health := map[string]interface{}{
		"status":     "healthy",
		"timestamp":  time.Now().Format(time.RFC3339),
		"file_count": fileCount,
		"uptime":     time.Since(startTime).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

var startTime = time.Now()

func loadConfig() Config {
	config := Config{
		Port:            "8080",
		UploadDir:       "./files",
		MetadataFile:    "./metadata.json",
		DefaultTTL:      1 * time.Hour,
		MaxFileSize:     100 * 1024 * 1024, // 100MB
		AllowedOrigins:  []string{"*"},
		CleanupInterval: 5 * time.Minute,
		MaxDownloads:    0, // unlimited by default
		RequirePassword: false,
		AdminPassword:   "",
		AllowedTypes:    []string{}, // all types allowed by default
	}

	// Load from config file if exists
	if data, err := os.ReadFile("config.json"); err == nil {
		json.Unmarshal(data, &config)
	}

	return config
}
