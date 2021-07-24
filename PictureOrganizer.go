// +build windows
package main

import (
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
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
	fileBytes = nil

	return md5String
}

// Collect file information and return fileInformation object
func getFileInfo(filePath string) fileInformation {
	fileStats, err := os.Stat(filePath)

	if err != nil {
		log.Fatal(err)
	}

	winAttribData := fileStats.Sys().(*syscall.Win32FileAttributeData)

	creationTime := winAttribData.LastWriteTime.Nanoseconds()
	lastAccessTime := winAttribData.LastAccessTime.Nanoseconds()
	lastWriteTime := winAttribData.LastWriteTime.Nanoseconds()

	sortedTime := []int64{
		creationTime,
		lastAccessTime,
		lastWriteTime,
	}

	sort.Slice(sortedTime, func(i, j int) bool { return sortedTime[i] < sortedTime[j] })

	smallestTime := time.Unix(0, sortedTime[0])

	fileInfo := fileInformation{
		fullName:     fileStats.Name(),
		fileName:     strings.Split(filepath.Base(filePath), ".")[0],
		path:         filePath,
		directory:    filepath.Dir(filePath),
		extension:    filepath.Ext(filePath),
		size:         fileStats.Size(),
		creationTime: smallestTime,
	}

	return fileInfo
}

func fileWorker(jobs <-chan string, fileInfoMaps chan<- map[string][]fileInformation) {
	for filePath := range jobs {
		md5hash := getMD5Hash(filePath)
		fileInfoMap := make(map[string][]fileInformation)
		fileInfo := getFileInfo(filePath)

		fileInfoMap[md5hash] = append(fileInfoMap[md5hash], fileInfo)
		fileInfoMaps <- fileInfoMap
	}
}

// Get a slice of files in the specified path
func getFilesInDirectory(path string, allFiles *map[string][]fileInformation, workers int) (int, int) {
	numberOfFiles := 0
	numberOfDirectories := 0

	mediaFileExtensions := []string{
		".JPG",
		".JPEG",
		".HEIC",
		".MP4",
		".MOV",
		".HEVC",
		".PNG",
		".JPEG",
		".GIF",
		".TIF",
		".BMP",
		".AVI",
	}

	filePaths := []string{}

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

	numOfFilePaths := len(filePaths)

	jobs := make(chan string, numOfFilePaths)
	fileInfoMaps := make(chan map[string][]fileInformation, numOfFilePaths)

	for w := 1; w <= workers; w++ {
		go fileWorker(jobs, fileInfoMaps)
	}

	for _, filePath := range filePaths {
		jobs <- filePath
	}

	close(jobs)

	for range filePaths {
		f := <-fileInfoMaps
		for key, val := range f {
			for _, v := range val {
				for _, ext := range mediaFileExtensions {
					if strings.ToUpper(v.extension) == ext {
						(*allFiles)[key] = append((*allFiles)[key], v)
					}
				}
			}

			// (*allFiles)[key] = append((*allFiles)[key], val...)
		}
	}

	return numberOfFiles, numberOfDirectories
}

func copyWorker(copyJobs <-chan struct {
	int
	string
	fileInformation
}, copiedFilesChan chan<- int) {
	for cj := range copyJobs {
		val := cj.fileInformation
		sortPath := cj.string
		incrementNum := cj.int
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

		incrementName := fmt.Sprintf("MediaFile_%d%s", incrementNum, val.extension)

		destPath := destDir + "/" + incrementName

		fileFound := true

		for fileFound {
			_, err := os.Stat(destPath)
			if err == nil {
				fmt.Printf("File '%s' already exists. Skipping\n", incrementName)
			} else {
				copiedFilesChan <- copy(val.path, destPath)
				fileFound = false
			}
		}
	}
}

// Copy file function
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
	numCPUs := runtime.NumCPU()

	runtime.GOMAXPROCS(numCPUs)

	var path string
	var sortPath string
	var workers int

	// flags declaration
	flag.StringVar(&path, "p", "F:/PhotoBackups", "Path to original files")
	flag.StringVar(&sortPath, "d", "F:/SortedPhotosNoDupes", "Copy destination path")
	flag.IntVar(&workers, "w", 10, "Number of worker goroutines to launch during concurrent operations")

	flag.Usage = func() {
		fmt.Printf("\nParameters Available:\n\n")
		fmt.Printf("-p\tPath to original files\n")
		fmt.Printf("-d\tCopy destination path\n\n")
		fmt.Printf("Example: ./PictureOrganizer.exe -p \"C:\\foo\\bar\" -d \"C:\\bar\\foo\"\n")
	}

	flag.Parse()

	_, err := os.Stat(sortPath)
	if os.IsNotExist(err) {
		err := os.Mkdir(sortPath, fs.FileMode(0777))
		if err != nil {
			log.Fatalf("Destination path '%s' could not be created!\nError: %s\n", sortPath, err)
		}
	}

	fmt.Println("Photos will be sorted into folder", sortPath)

	start := time.Now()

	allFilesMap := make(map[string][]fileInformation)

	fmt.Println("Gathering file information...")

	numberOfFiles, numberOfDirectories := getFilesInDirectory(path, &allFilesMap, workers)

	numberOfMaps := len(allFilesMap)

	gatherFileInfoElapsed := time.Since(start)

	fmt.Println("Finished gathering file information in ", gatherFileInfoElapsed)
	fmt.Println("\nTotal Number of Files found: ", numberOfFiles)
	fmt.Println("Total Number of Directories found: ", numberOfDirectories)
	fmt.Println("Total Number of Unique Photos found: ", numberOfMaps)
	fmt.Println("")

	copiedFiles := 0
	incrementNum := 0

	copyJobs := make(chan struct {
		int
		string
		fileInformation
	}, numberOfMaps)
	copiedFilesChan := make(chan int, numberOfMaps)

	for w := 1; w <= workers; w++ {
		go copyWorker(copyJobs, copiedFilesChan)
	}

	for key := range allFilesMap {
		incrementNum++
		copyJobs <- struct {
			int
			string
			fileInformation
		}{incrementNum, sortPath, allFilesMap[key][0]}
	}

	close(copyJobs)

	for range allFilesMap {
		cp := <-copiedFilesChan
		copiedFiles = copiedFiles + cp
	}

	elapsed := time.Since(start)

	fmt.Println("\nFiles copied: ", copiedFiles)

	fmt.Println("Time elapsed: ", elapsed)

	fmt.Println("\nProcess Finished!")
}
