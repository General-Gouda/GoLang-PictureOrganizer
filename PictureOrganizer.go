// +build windows
package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	// "sync"
)

// File information struct
type fileInformation struct {
	fullName     string
	fileName     string
	path         string
	directory    string
	extension    string
	size         int64
	creationTime time.Time
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

	winAttribData := fileStats.Sys().(*syscall.Win32FileAttributeData)

	creationTime := time.Unix(0, winAttribData.CreationTime.Nanoseconds())

	fileInfo := fileInformation{
		fullName:     fileStats.Name(),
		fileName:     strings.Split(filepath.Base(filePath), ".")[0],
		path:         filePath,
		directory:    filepath.Dir(filePath),
		extension:    filepath.Ext(filePath),
		size:         fileStats.Size(),
		creationTime: creationTime,
	}

	return fileInfo
}

// Get a slice of files in the specified path
func getFilesInDirectory(path string, allFiles *map[string][]fileInformation) (int, int) {
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

	return numberOfFiles, numberOfDirectories
}

func copy(src, dst string) int {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		fmt.Println(err)
		return 0
	}

	if !sourceFileStat.Mode().IsRegular() {
		fmt.Printf("%s is not a regular file\n", src)
		return 0
	}

	source, err := os.Open(src)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	defer destination.Close()

	bytes, err := io.Copy(destination, source)

	if err != nil {
		fmt.Println(err)
		return 0
	} else {
		fmt.Sprintln(bytes)
		return 1
	}
}

func main() {
	// path := "E:/Backup"  //247631 files
	// path := "E:/Backup/Amandas iPhone Pics" //2085 files
	// path := "F:/Backups/Amandas iPhone Pics" //2085 files
	// path := "E:/Backup/Amandas iPhone Pics 10-2014 to 05-2018" //18,335 files
	// path := "C:/DRIVERS" // 458 files
	// path := "C:/DRIVERS/Dupes" // 6 files in 3 folder
	// path := "C:/ffmpeg" // 44 files in 3 folders
	path := "D:/Amandas iPhone Pics 10-2014 to 05-2018" // 5742 files

	sortPath := "D:/SortedPhotos"

	fmt.Println("Photos will be sorted into folder ", sortPath)

	start := time.Now()

	allFilesMap := make(map[string][]fileInformation)

	numberOfFiles, numberOfDirectories := getFilesInDirectory(path, &allFilesMap)

	numberOfMaps := len(allFilesMap)

	fmt.Println("Total Number of MD5 hashes found: ", numberOfMaps)
	fmt.Println("Total Number of Files found: ", numberOfFiles)
	fmt.Println("Total Number of Directories found: ", numberOfDirectories)

	copiedFiles := 0

	for key := range allFilesMap {
		copiedFilesChan := make(chan int)
		go func() {
			for _, val := range allFilesMap[key] {
				destDir := fmt.Sprintf("%s/%d/%s", sortPath, val.creationTime.Year(), val.creationTime.Month())
				splitDestDir := strings.Split(destDir, "/")

				incrementDir := splitDestDir[0] + "/"

				for _, split := range splitDestDir[1:] {
					err := os.Mkdir((incrementDir + "/" + split), fs.FileMode(0777))
					incrementDir = incrementDir + split + "/"
					if err != nil {
						continue
					}
				}

				destPath := destDir + "/" + val.fullName

				fileFound := true
				incrementFileNum := 2

				for fileFound {
					_, err := os.Stat(destPath)
					if err == nil {
						fmt.Println("File already exists. Incrementing name by one: ", destPath)
						destPath = destDir + "/" + val.fileName + " (" + fmt.Sprint(incrementFileNum) + ")" + val.extension
						incrementFileNum++
					} else {
						copiedFilesChan <- copy(val.path, destPath)
						// copiedFiles = copiedFiles + copy(val.path, destPath)
						fileFound = false
					}
				}
			}

			close(copiedFilesChan)
		}()

		for cp := range copiedFilesChan {
			copiedFiles = copiedFiles + cp
		}
	}

	elapsed := time.Since(start)

	fmt.Println("Files copied: ", copiedFiles)

	fmt.Println("Time elapsed: ", elapsed)

	fmt.Println("Process Finished!")
}
