#!/bin/bash
# Enhanced file upload convenience script
# Usage: ./upload.sh <file> [options]
set -e

# Default configuration
DEFAULT_TTL=3600  # 1 hour
DEFAULT_SERVER="http://localhost:8080"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Global variables
FILE=""
TTL=""
SERVER=""
PASSWORD=""
DESCRIPTION=""
TAGS=""
MAX_DOWNLOADS=""
INTERACTIVE=false

usage() {
    echo "Enhanced File Upload Script"
    echo "Usage: $0 <file> [options]"
    echo ""
    echo "Arguments:"
    echo "  file              Path to file to upload (required)"
    echo ""
    echo "Options:"
    echo "  -t, --ttl SECONDS          Time to live in seconds (default: $DEFAULT_TTL)"
    echo "  -s, --server URL           Upload server URL (default: $DEFAULT_SERVER)"
    echo "  -p, --password PASSWORD    Protect file with password"
    echo "  -d, --description TEXT     File description"
    echo "  --tags TAGS               Comma-separated tags (e.g., 'important,work,temp')"
    echo "  -m, --max-downloads NUM    Maximum number of downloads (default: unlimited)"
    echo "  -i, --interactive         Interactive mode with prompts"
    echo "  -h, --help                Show this help"
    echo ""
    echo "Commands (without file argument):"
    echo "  list                      List all files on server"
    echo "  stats                     Show server statistics"
    echo "  search <query>            Search files by name/description"
    echo "  info <file_id>            Get file information"
    echo "  download <file_id> [pwd]  Download file by ID"
    echo "  delete <file_id>          Delete file by ID"
    echo "  health                    Check server health"
    echo ""
    echo "Environment variables:"
    echo "  UPLOAD_SERVER             Default server URL"
    echo "  UPLOAD_TTL                Default TTL in seconds"
    echo ""
    echo "Examples:"
    echo "  $0 myfile.txt"
    echo "  $0 myfile.txt -t 7200 -p secret123"
    echo "  $0 myfile.txt --interactive"
    echo "  $0 myfile.txt -d 'Important document' --tags 'work,urgent'"
    echo "  $0 list"
    echo "  $0 search 'document'"
    echo "  $0 stats"
    echo "  UPLOAD_SERVER=http://my-server:8080 $0 myfile.txt"
    exit 1
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                usage
                ;;
            -t|--ttl)
                TTL="$2"
                shift 2
                ;;
            -s|--server)
                SERVER="$2"
                shift 2
                ;;
            -p|--password)
                PASSWORD="$2"
                shift 2
                ;;
            -d|--description)
                DESCRIPTION="$2"
                shift 2
                ;;
            --tags)
                TAGS="$2"
                shift 2
                ;;
            -m|--max-downloads)
                MAX_DOWNLOADS="$2"
                shift 2
                ;;
            -i|--interactive)
                INTERACTIVE=true
                shift
                ;;
            -*)
                echo -e "${RED}Unknown option: $1${NC}" >&2
                usage
                ;;
            *)
                if [ -z "$FILE" ] && [ "$1" != "list" ] && [ "$1" != "stats" ] && [ "$1" != "search" ] && [ "$1" != "info" ] && [ "$1" != "download" ] && [ "$1" != "delete" ] && [ "$1" != "health" ]; then
                    FILE="$1"
                else
                    COMMAND="$1"
                    COMMAND_ARG="$2"
                    COMMAND_ARG2="$3"
                fi
                shift
                ;;
        esac
    done
}

# Interactive mode
interactive_mode() {
    echo -e "${CYAN}=== Interactive Upload Mode ===${NC}"

    if [ -z "$TTL" ]; then
        echo -n "TTL in seconds (default $DEFAULT_TTL): "
        read input_ttl
        TTL="${input_ttl:-$DEFAULT_TTL}"
    fi

    if [ -z "$PASSWORD" ]; then
        echo -n "Password protection (leave empty for none): "
        read -s input_password
        PASSWORD="$input_password"
        echo
    fi

    if [ -z "$DESCRIPTION" ]; then
        echo -n "File description (optional): "
        read input_description
        DESCRIPTION="$input_description"
    fi

    if [ -z "$TAGS" ]; then
        echo -n "Tags (comma-separated, optional): "
        read input_tags
        TAGS="$input_tags"
    fi

    if [ -z "$MAX_DOWNLOADS" ]; then
        echo -n "Max downloads (leave empty for unlimited): "
        read input_max_downloads
        MAX_DOWNLOADS="$input_max_downloads"
    fi

    echo
}

# Format file size
format_size() {
    local size=$1
    if [ "$size" -lt 1024 ]; then
        echo "${size}B"
    elif [ "$size" -lt 1048576 ]; then
        echo "$(( size / 1024 ))KB"
    elif [ "$size" -lt 1073741824 ]; then
        echo "$(( size / 1048576 ))MB"
    else
        echo "$(( size / 1073741824 ))GB"
    fi
}

# Format duration
format_duration() {
    local seconds=$1
    local hours=$((seconds / 3600))
    local minutes=$(((seconds % 3600) / 60))
    local secs=$((seconds % 60))

    if [ "$hours" -gt 0 ]; then
        echo "${hours}h ${minutes}m ${secs}s"
    elif [ "$minutes" -gt 0 ]; then
        echo "${minutes}m ${secs}s"
    else
        echo "${secs}s"
    fi
}

# Check server availability
check_server() {
    local server="$1"
    if ! curl -s -f "$server/api/health" >/dev/null 2>&1; then
        echo -e "${RED}Error: Cannot connect to server at $server${NC}" >&2
        echo "Make sure the server is running and accessible." >&2
        exit 1
    fi
}

# Command functions
list_files() {
    echo -e "${BLUE}Fetching file list from server...${NC}"
    local response=$(curl -s -f -H "Accept: application/json" "$SERVER/api/files" 2>/dev/null)

    if [ $? -eq 0 ]; then
        echo "$response" | python3 -c "
import json, sys
data = json.load(sys.stdin)
files = data.get('files', [])
total = data.get('total', 0)

if not files:
    print('No files found.')
else:
    print(f'Found {total} files:')
    print()
    for i, f in enumerate(files[:10], 1):  # Show first 10
        size = f.get('size', 0)
        downloads = f.get('downloads', 0)
        max_dl = f.get('max_downloads', 0)
        tags = ', '.join(f.get('tags', []))

        print(f'{i:2d}. {f[\"original_name\"]} ({size} bytes)')
        print(f'    ID: {f[\"id\"]}')
        print(f'    Downloads: {downloads}' + (f'/{max_dl}' if max_dl > 0 else ''))
        print(f'    Expires: {f[\"expires_at\"][:19]}')
        if tags:
            print(f'    Tags: {tags}')
        if f.get('description'):
            print(f'    Description: {f[\"description\"]}')
        print()

    if total > 10:
        print(f'... and {total - 10} more files.')
"
    else
        echo -e "${RED}Failed to fetch file list${NC}" >&2
        exit 1
    fi
}

show_stats() {
    echo -e "${BLUE}Fetching server statistics...${NC}"
    local response=$(curl -s -f -H "Accept: application/json" "$SERVER/stats" 2>/dev/null)

    if [ $? -eq 0 ]; then
        echo "$response" | python3 -c "
import json, sys
data = json.load(sys.stdin)
print(f'📊 Server Statistics:')
print(f'   Total Files: {data.get(\"total_files\", 0)}')
print(f'   Active Files: {data.get(\"active_files\", 0)}')
print(f'   Total Downloads: {data.get(\"total_downloads\", 0)}')
print(f'   Total Size: {data.get(\"total_size\", 0):,} bytes')
"
    else
        echo -e "${RED}Failed to fetch statistics${NC}" >&2
        exit 1
    fi
}

search_files() {
    local query="$1"
    echo -e "${BLUE}Searching for: '$query'${NC}"
    local response=$(curl -s -f -H "Accept: application/json" "$SERVER/search?q=$query" 2>/dev/null)

    if [ $? -eq 0 ]; then
        echo "$response" | python3 -c "
import json, sys
files = json.load(sys.stdin)

if not files:
    print('No files found matching the search criteria.')
else:
    print(f'Found {len(files)} matching files:')
    print()
    for i, f in enumerate(files, 1):
        print(f'{i:2d}. {f[\"original_name\"]} ({f[\"size\"]} bytes)')
        print(f'    ID: {f[\"id\"]}')
        print(f'    Downloads: {f[\"downloads\"]}')
        if f.get('description'):
            print(f'    Description: {f[\"description\"]}')
        print()
"
    else
        echo -e "${RED}Search failed${NC}" >&2
        exit 1
    fi
}

get_file_info() {
    local file_id="$1"
    echo -e "${BLUE}Getting file information for: $file_id${NC}"
    local response=$(curl -s -f -H "Accept: application/json" "$SERVER/info/$file_id" 2>/dev/null)

    if [ $? -eq 0 ]; then
        echo "$response" | python3 -c "
import json, sys, datetime
f = json.load(sys.stdin)

print(f'📄 File Information:')
print(f'   ID: {f[\"id\"]}')
print(f'   Original Name: {f[\"original_name\"]}')
print(f'   Size: {f[\"size\"]:,} bytes')
print(f'   Content Type: {f.get(\"content_type\", \"unknown\")}')
print(f'   Uploaded: {f[\"upload_time\"][:19]}')
print(f'   Expires: {f[\"expires_at\"][:19]}')
print(f'   Downloads: {f[\"downloads\"]}' + (f'/{f[\"max_downloads\"]}' if f.get('max_downloads', 0) > 0 else ''))
print(f'   Checksum: {f.get(\"checksum\", \"unknown\")}')
print(f'   Uploader IP: {f.get(\"uploader_ip\", \"unknown\")}')

if f.get('tags'):
    print(f'   Tags: {\", \".join(f[\"tags\"])}')
if f.get('description'):
    print(f'   Description: {f[\"description\"]}')
if f.get('password'):
    print(f'   Password Protected: Yes')

print(f'   Download URL: http://localhost:8080/download/{f[\"id\"]}')
"
    else
        echo -e "${RED}Failed to get file information${NC}" >&2
        exit 1
    fi
}

download_file() {
    local file_id="$1"
    local password="$2"
    local url="$SERVER/download/$file_id"

    if [ ! -z "$password" ]; then
        url="${url}?password=$password"
    fi

    echo -e "${BLUE}Downloading file: $file_id${NC}"

    # Get filename from server first
    local filename=$(curl -s -I "$url" | grep -i 'content-disposition' | sed 's/.*filename="\(.*\)".*/\1/')

    if [ -z "$filename" ]; then
        filename="download_$file_id"
    fi

    if curl -f -o "$filename" "$url" 2>/dev/null; then
        echo -e "${GREEN}✓ File downloaded successfully: $filename${NC}"
    else
        echo -e "${RED}Download failed${NC}" >&2
        exit 1
    fi
}

delete_file() {
    local file_id="$1"
    echo -e "${YELLOW}Are you sure you want to delete file $file_id? (y/N)${NC}"
    read -r confirm

    if [ "$confirm" = "y" ] || [ "$confirm" = "Y" ]; then
        if curl -s -f "$SERVER/delete/$file_id" >/dev/null 2>&1; then
            echo -e "${GREEN}✓ File deleted successfully${NC}"
        else
            echo -e "${RED}Delete failed${NC}" >&2
            exit 1
        fi
    else
        echo "Delete cancelled."
    fi
}

check_health() {
    echo -e "${BLUE}Checking server health...${NC}"
    local response=$(curl -s -f -H "Accept: application/json" "$SERVER/api/health" 2>/dev/null)

    if [ $? -eq 0 ]; then
        echo "$response" | python3 -c "
import json, sys
data = json.load(sys.stdin)
print(f'🏥 Server Health:')
print(f'   Status: {data.get(\"status\", \"unknown\")}')
print(f'   File Count: {data.get(\"file_count\", 0)}')
print(f'   Uptime: {data.get(\"uptime\", \"unknown\")}')
print(f'   Timestamp: {data.get(\"timestamp\", \"unknown\")}')
"
    else
        echo -e "${RED}Health check failed${NC}" >&2
        exit 1
    fi
}

upload_file() {
    # Validate file exists
    if [ ! -f "$FILE" ]; then
        echo -e "${RED}Error: File not found: $FILE${NC}" >&2
        exit 1
    fi

    # Set defaults
    TTL="${TTL:-${UPLOAD_TTL:-$DEFAULT_TTL}}"
    SERVER="${SERVER:-${UPLOAD_SERVER:-$DEFAULT_SERVER}}"

    # Interactive mode
    if [ "$INTERACTIVE" = true ]; then
        interactive_mode
    fi

    # Get file info
    FILE_SIZE=$(stat -f%z "$FILE" 2>/dev/null || stat -c%s "$FILE" 2>/dev/null || echo "unknown")
    FILE_NAME=$(basename "$FILE")

    echo -e "${BLUE}📤 Uploading file...${NC}"
    echo -e "File: ${YELLOW}$FILE_NAME${NC}"
    echo -e "Size: ${YELLOW}$(format_size $FILE_SIZE)${NC}"
    echo -e "TTL:  ${YELLOW}$(format_duration $TTL)${NC}"
    echo -e "Server: ${YELLOW}$SERVER${NC}"

    if [ ! -z "$PASSWORD" ]; then
        echo -e "Password: ${MAGENTA}***${NC}"
    fi
    if [ ! -z "$DESCRIPTION" ]; then
        echo -e "Description: ${CYAN}$DESCRIPTION${NC}"
    fi
    if [ ! -z "$TAGS" ]; then
        echo -e "Tags: ${CYAN}$TAGS${NC}"
    fi
    if [ ! -z "$MAX_DOWNLOADS" ]; then
        echo -e "Max downloads: ${CYAN}$MAX_DOWNLOADS${NC}"
    fi

    echo ""

    # Build curl command
    local curl_cmd="curl -s -f -H \"Accept: application/json\" -F \"file=@$FILE\" -F \"ttl=$TTL\""

    if [ ! -z "$PASSWORD" ]; then
        curl_cmd="$curl_cmd -F \"password=$PASSWORD\""
    fi
    if [ ! -z "$DESCRIPTION" ]; then
        curl_cmd="$curl_cmd -F \"description=$DESCRIPTION\""
    fi
    if [ ! -z "$TAGS" ]; then
        curl_cmd="$curl_cmd -F \"tags=$TAGS\""
    fi
    if [ ! -z "$MAX_DOWNLOADS" ]; then
        curl_cmd="$curl_cmd -F \"max_downloads=$MAX_DOWNLOADS\""
    fi

    curl_cmd="$curl_cmd \"$SERVER/upload\""

    # Perform upload
    local response=$(eval $curl_cmd 2>/dev/null)

    if [ $? -eq 0 ]; then
        # Parse JSON response and display results
        echo "$response" | python3 -c "
import json, sys
data = json.load(sys.stdin)
print('✅ Upload successful!')
print()
print(f'📁 File Details:')
print(f'   ID: {data[\"id\"]}')
print(f'   Name: {data[\"original_name\"]}')
print(f'   Size: {data[\"size\"]:,} bytes')
print(f'   Checksum: {data[\"checksum\"]}')
print(f'   Expires: {data[\"expires_at\"]}')
if data.get('max_downloads', 0) > 0:
    print(f'   Max Downloads: {data[\"max_downloads\"]}')
print()
print(f'🔗 Download URL:')
print(data['download_url'])
"

        # Extract download URL for clipboard
        local download_url=$(echo "$response" | python3 -c "import json, sys; print(json.load(sys.stdin)['download_url'])")

        # Copy to clipboard if available
        if command -v pbcopy >/dev/null 2>&1; then
            echo "$download_url" | pbcopy
            echo -e "${GREEN}📋 Download URL copied to clipboard (macOS)${NC}"
        elif command -v xclip >/dev/null 2>&1; then
            echo "$download_url" | xclip -selection clipboard
            echo -e "${GREEN}📋 Download URL copied to clipboard (Linux)${NC}"
        elif command -v clip >/dev/null 2>&1; then
            echo "$download_url" | clip
            echo -e "${GREEN}📋 Download URL copied to clipboard (Windows)${NC}"
        fi

    else
        echo -e "${RED}❌ Upload failed${NC}" >&2
        echo "Check if the server is running at: $SERVER" >&2
        exit 1
    fi
}

# Main script logic
main() {
    if [ $# -eq 0 ]; then
        usage
    fi

    parse_args "$@"

    # Set server default
    SERVER="${SERVER:-${UPLOAD_SERVER:-$DEFAULT_SERVER}}"

    # Check server availability
    check_server "$SERVER"

    # Handle commands
    if [ ! -z "$COMMAND" ]; then
        case "$COMMAND" in
            list)
                list_files
                ;;
            stats)
                show_stats
                ;;
            search)
                if [ -z "$COMMAND_ARG" ]; then
                    echo -e "${RED}Search query required${NC}" >&2
                    exit 1
                fi
                search_files "$COMMAND_ARG"
                ;;
            info)
                if [ -z "$COMMAND_ARG" ]; then
                    echo -e "${RED}File ID required${NC}" >&2
                    exit 1
                fi
                get_file_info "$COMMAND_ARG"
                ;;
            download)
                if [ -z "$COMMAND_ARG" ]; then
                    echo -e "${RED}File ID required${NC}" >&2
                    exit 1
                fi
                download_file "$COMMAND_ARG" "$COMMAND_ARG2"
                ;;
            delete)
                if [ -z "$COMMAND_ARG" ]; then
                    echo -e "${RED}File ID required${NC}" >&2
                    exit 1
                fi
                delete_file "$COMMAND_ARG"
                ;;
            health)
                check_health
                ;;
            *)
                echo -e "${RED}Unknown command: $COMMAND${NC}" >&2
                usage
                ;;
        esac
    elif [ ! -z "$FILE" ]; then
        upload_file
    else
        usage
    fi
}

# Check for Python3 (required for JSON parsing)
if ! command -v python3 >/dev/null 2>&1; then
    echo -e "${RED}Error: Python3 is required for JSON parsing${NC}" >&2
    echo "Please install Python3 to use this script." >&2
    exit 1
fi

main "$@"