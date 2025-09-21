package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	config := loadConfig()
	fm := NewFileManager(config)

	// Ensure upload directory exists
	os.MkdirAll(config.UploadDir, 0755)

	// Routes
	http.HandleFunc("/upload", fm.uploadFile)
	http.HandleFunc("/download/", fm.downloadFile)
	http.HandleFunc("/delete/", fm.deleteFile)
	http.HandleFunc("/manage", fm.manageFiles)
	http.HandleFunc("/search", fm.searchFiles)
	http.HandleFunc("/stats", fm.getStats)
	http.HandleFunc("/info/", fm.fileInfo)
	http.HandleFunc("/bulk-delete", fm.bulkDelete)
	http.HandleFunc("/api/", fm.apiHandler)
	http.HandleFunc("/", fm.manageFiles)

	// Graceful shutdown
	log.Printf("Starting file upload service on port %s", config.Port)
	log.Printf("Upload directory: %s", config.UploadDir)
	log.Printf("Management interface: http://localhost:%s/manage", config.Port)

	if err := http.ListenAndServe(":"+config.Port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
