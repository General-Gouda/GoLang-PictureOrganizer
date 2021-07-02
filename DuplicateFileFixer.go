package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	// "io/fs"
	"log"
	"os"
	"path/filepath"
	"time"
	// "sync"
)

// File information struct
type fileInformation struct {
	name string
	path string
	size int64
}

// Get a file's MD5 hash
func getMD5Hash(filePath string) string {
	fileBytes, _ := os.ReadFile(filePath)
	md5Sum := md5.Sum(fileBytes)
	md5String := hex.EncodeToString(md5Sum[:])

	// fmt.Printf("Adding another hash for %s!\n", filePath)

	return md5String
}

func getFileInfo(filePath string) fileInformation {
	fileStats, err := os.Stat(filePath)

	if err != nil {
		log.Fatal(err)
	}

	fileInfo := fileInformation{
		name: fileStats.Name(),
		path: filePath,
		size: fileStats.Size(),
	}

	return fileInfo
}

// Get a slice of files in the specified path
func getFilesInDirectory(path string, allFiles *map[string][]fileInformation) int {
	numberOfFiles := 0
	numberOfDirectories := 0

	filePaths := make([]string, 0)

	var ff = func(path string, item os.FileInfo, errX error) error {

		if errX != nil {
			fmt.Printf("error 「%v」 at a path 「%q」\n", errX, path)
			return errX
		}

		if item.IsDir() {
			numberOfDirectories++
		} else {
			numberOfFiles++
			filePaths = append(filePaths, path)
		}

		return nil
	}

	err := filepath.Walk(path, ff)

	if err != nil {
		log.Printf("Failure retrieving file information from %s", path)
	}

	fileInfoMaps := make(chan map[string][]fileInformation)

	go func() {
		for _, filePath := range filePaths {
			md5hash := getMD5Hash(filePath)
			fileInfoMap := make(map[string][]fileInformation)
			fileInfo := getFileInfo(filePath)
			fileInfoMap[md5hash] = append(fileInfoMap[md5hash], fileInfo)
			fileInfoMaps <- fileInfoMap
		}

		close(fileInfoMaps)
	}()

	for f := range fileInfoMaps {
		for key, val := range f {
			(*allFiles)[key] = append((*allFiles)[key], val...)
		}
	}

	return numberOfFiles
}

func main() {
	// path := "E:/Backup"  //247631 files
	// path := "E:/Backup/Amandas iPhone Pics" //2085 files
	// path := "C:/DRIVERS" // 458 files
	path := "C:/DRIVERS/Dupes" // 6 files in 3 folder
	// path := "C:/ffmpeg" // 44 files in 3 folders

	start := time.Now()

	allFilesMap := make(map[string][]fileInformation)

	numberOfFiles := getFilesInDirectory(path, &allFilesMap)

	numberOfMaps := len(allFilesMap)

	fmt.Println("Total Number of MD5 hashes found: ", numberOfMaps)
	fmt.Println("Total Number of Files found: ", numberOfFiles)

	for key := range allFilesMap {
		if len(allFilesMap[key]) > 1 {
			fmt.Println(key)
		}
	}

	elapsed := time.Since(start)

	fmt.Println("Time elapsed: ", elapsed)

	fmt.Println("Process Finished!")
}
