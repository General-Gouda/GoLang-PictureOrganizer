package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
)

// File information struct
type fileInfo struct {
	name    string
	path    string
	size    int64
	isDir   bool
	md5hash string
}

// Get a file's MD5 hash
func getMD5Hash(file fileInfo) string {
	fileBytes, _ := ioutil.ReadFile(file.path)
	md5Sum := md5.Sum(fileBytes)
	md5String := hex.EncodeToString(md5Sum[:])

	return md5String
}

// Get a slice of files in the specified path
func getFilesInDirectory(path string) []fileInfo {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatalf("Failure retreiving file information from %s", path)
	}

	var allFiles []fileInfo

	for _, file := range files {
		fileInfo := fileInfo{
			name:  file.Name(),
			path:  filepath.Join(path, file.Name()),
			size:  file.Size(),
			isDir: file.IsDir(),
		}

		allFiles = append(allFiles, fileInfo)
	}

	return allFiles
}

func main() {
	allFiles := getFilesInDirectory("/home/gouda/Pictures/")
	// allFiles := getFilesInDirectory("/opt/google/chrome")

	for index, file := range allFiles {
		if file.isDir == false {
			file.md5hash = getMD5Hash(file)
			fmt.Printf("Index: %d\nName: %s\nPath: %s\nSize: %d\nMD5Hash: %s\n\n",
				index,
				file.name,
				file.path,
				file.size,
				file.md5hash,
			)
		}
	}
}
