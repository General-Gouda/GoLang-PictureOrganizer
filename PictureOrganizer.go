//go:build windows
// +build windows

package main

import (
	"bufio"
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

type copyStruct struct {
	incrementNum int
	sortPath     string
	key          string
	fileInfo     fileInformation
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
func getFilesInDirectory(path string, allFiles *map[string][]fileInformation, workers int, mediaFileExtensions []string) (int, int) {
	numberOfFiles := 0
	numberOfDirectories := 0

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

	jobsChannel := make(chan string, numOfFilePaths)
	fileInfoMapsChannel := make(chan map[string][]fileInformation, numOfFilePaths)

	for w := 1; w <= workers; w++ {
		go fileWorker(jobsChannel, fileInfoMapsChannel)
	}

	for _, filePath := range filePaths {
		jobsChannel <- filePath
	}

	close(jobsChannel)

	counter := 0

	for range filePaths {
		fileInfoMaps := <-fileInfoMapsChannel
		counter++
		fmt.Print(displayProgressBar(counter, numOfFilePaths))
		for key, files := range fileInfoMaps {
			for _, file := range files {
				for _, mediaFileExtension := range mediaFileExtensions {
					if strings.ToUpper(file.extension) == mediaFileExtension {
						(*allFiles)[key] = append((*allFiles)[key], file)
					}
				}
			}
		}
	}

	return numberOfFiles, numberOfDirectories
}

func copyWorker(copyJobs <-chan copyStruct, copiedFilesChan chan<- int, moveFiles, noRenameFiles bool) {
	for copyJob := range copyJobs {
		val := copyJob.fileInfo
		sortPath := copyJob.sortPath
		md5hash := copyJob.key
		destDir := fmt.Sprintf("%s/%d/%s", sortPath, val.creationTime.Year(), val.creationTime.Month())
		formattedCreationTime := fmt.Sprintf("%d-%02d-%02d %02d%02d%02d",
			val.creationTime.Year(),
			int(val.creationTime.Month()),
			val.creationTime.Day(),
			val.creationTime.Hour(),
			val.creationTime.Minute(),
			val.creationTime.Second(),
		)

		// Ensure that the folders exist all the way down the tree
		splitDestDir := strings.Split(destDir, "/")

		incrementDir := splitDestDir[0] + "/"

		for _, split := range splitDestDir[1:] {
			err := os.Mkdir((incrementDir + "/" + split), fs.FileMode(0777))
			incrementDir = incrementDir + split + "/"
			if err != nil {
				continue
			}
		}

		var incrementName string
		var destPath string

		if noRenameFiles == true {
			// Use the original file name and then check if it exists. If it does, add a number to
			// the end of the file name and try again.
			fileNameContinue := false

			incrementName = fmt.Sprintf("%s", val.fullName)
			incrementNumber := 1
			destPath = fmt.Sprintf("%s/%s", destDir, incrementName)

			for fileNameContinue == false {
				_, err := os.Stat(destPath)
				if err == nil {
					incrementNumber += 1
					incrementName = fmt.Sprintf("%s_%d%s", val.fileName, incrementNumber, val.extension)
					destPath = fmt.Sprintf("%s/%s", destDir, incrementName)
				} else {
					fileNameContinue = true
				}
			}
		} else {
			incrementName = fmt.Sprintf("%s-%s%s", formattedCreationTime, md5hash, val.extension)
			destPath = destDir + "/" + incrementName
		}

		// Perform another file exists check to make sure the renamed files aren't duplicated
		// if the noRenameFiles flag is false
		fileFound := true

		for fileFound {
			_, err := os.Stat(destPath)
			if err == nil {
				fmt.Printf("File '%s' already exists. Skipping\n", incrementName)
				copiedFilesChan <- 0
			} else {
				copiedFilesChan <- copy(val.path, destPath, moveFiles)
				fileFound = false
			}
		}
	}
}

// Copy file function
func copy(sourceFile, destinationFile string, moveFile bool) int {
	sourceFileStat, err := os.Stat(sourceFile)
	if err != nil {
		fmt.Println(err)
		return 0
	}

	if !sourceFileStat.Mode().IsRegular() {
		fmt.Printf("%s is not a regular file\n", sourceFile)
		return 0
	}

	// Move the file with a rename if moveFile is True
	// else Copy the file
	if moveFile {
		err := os.Rename(sourceFile, destinationFile)
		if err != nil {
			return 0
		} else {
			return 1
		}
	} else {
		source, err := os.Open(sourceFile)
		if err != nil {
			fmt.Println(err)
			return 0
		}
		defer source.Close()

		destination, err := os.Create(destinationFile)
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
}

func displayProgressBar(count, total int) string {
	doneBar := "█"
	incompleteBar := " "

	percentComplete := int(float32(count) / float32(total) * 100)
	doneBarsDrawn := int(float32(count) / float32(total) * 50)
	incompleteBarsDrawn := int(50 - doneBarsDrawn)

	formattedString := "\r["

	for range doneBarsDrawn {
		formattedString += doneBar
	}

	for range incompleteBarsDrawn {
		formattedString += incompleteBar
	}

	finishTheString := fmt.Sprintf("] %d%% - %d/%d", percentComplete, count, total)

	formattedString += finishTheString

	return formattedString
}

func main() {
	numCPUs := runtime.NumCPU()

	runtime.GOMAXPROCS(numCPUs)

	var path string
	var sortPath string
	var workers int
	var moveFiles bool
	var noRenameFiles bool

	// flags declaration
	flag.StringVar(&path, "p", "F:/PhotoBackups", "Path to original files")
	flag.StringVar(&sortPath, "d", "F:/SortedPhotosNoDupes", "Copy destination path")
	flag.IntVar(&workers, "w", 10, "Number of worker goroutines to launch during concurrent operations")
	flag.BoolVar(&moveFiles, "m", false, "If True, move the files and remove them from the original path.")
	flag.BoolVar(&noRenameFiles, "r", false, "If True, do not rename the files with creation datetime and MD5 hash.")

	flag.Usage = func() {
		fmt.Printf("\nParameters Available:\n\n")
		fmt.Printf("-p\tPath to original files\n")
		fmt.Printf("-d\tCopy destination path\n\n")
		fmt.Printf("-m\tMove the files to the Destination Folder\n\n")
		fmt.Printf("-r\tDo not rename the files\n\n")
		fmt.Printf("Example: ./PictureOrganizer.exe -p \"C:\\foo\\bar\" -d \"C:\\bar\\foo\" -m\n")
	}

	flag.Parse()

	_, err := os.Stat(sortPath)
	if os.IsNotExist(err) {
		err := os.Mkdir(sortPath, fs.FileMode(0777))
		if err != nil {
			log.Fatalf("Destination path '%s' could not be created!\nError: %s\n", sortPath, err)
		}
	}

	fmt.Println("Photos will be sorted within folder", sortPath)

	start := time.Now()

	allFilesMap := make(map[string][]fileInformation)
	allDestinationFilesMap := make(map[string][]fileInformation)

	var mediaFileExtensions []string
	mediaFileExtensionsFile, medialFileErr := os.Open("mediaFileExtensions.txt")

	if medialFileErr != nil {
		fmt.Println("A mediaFileExtensions.txt file could not be found. Using default list of media file extensions instead.")
		mediaFileExtensions = []string{
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

		f, err := os.Create("mediaFileExtensions.txt")

		if err != nil {
			log.Print(err)
		}

		for _, value := range mediaFileExtensions {
			fmt.Fprintln(f, value) // print values to f, one per line
		}

		f.Close()
	} else {
		scanner := bufio.NewScanner(mediaFileExtensionsFile)

		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			mediaFileExtensions = append(mediaFileExtensions, scanner.Text())
		}

		mediaFileExtensionsFile.Close()
	}

	fmt.Println("Gathering source file information...")

	numberOfFiles, numberOfDirectories := getFilesInDirectory(path, &allFilesMap, workers, mediaFileExtensions)

	fmt.Println("\nGathering destination file information...")
	getFilesInDirectory(sortPath, &allDestinationFilesMap, workers, mediaFileExtensions)

	alreadyExists := 0

	for k := range allDestinationFilesMap {
		delete(allFilesMap, k)
		alreadyExists++
	}

	numberOfMaps := len(allFilesMap)

	gatherFileInfoElapsed := time.Since(start)

	fmt.Println("\nFinished gathering File information in ", gatherFileInfoElapsed)
	fmt.Println("\nTotal Number of Files found: ", numberOfFiles)
	fmt.Println("Total Number of Directories found: ", numberOfDirectories)
	fmt.Println("Total Number of Unique Files found: ", numberOfMaps)
	fmt.Println("Total Number of Files found that already exist in destination path: ", alreadyExists)
	fmt.Println("")

	copiedFiles := 0
	incrementNum := 0

	copyJobsChannel := make(chan copyStruct, numberOfMaps)
	copiedFilesChannel := make(chan int, numberOfMaps)

	for w := 1; w <= workers; w++ {
		go copyWorker(copyJobsChannel, copiedFilesChannel, moveFiles, noRenameFiles)
	}

	for key := range allFilesMap {
		incrementNum++
		copyJobsChannel <- copyStruct{incrementNum, sortPath, key, allFilesMap[key][0]}
	}

	close(copyJobsChannel)

	fmt.Println("Copying files...")

	for range allFilesMap {
		copiedFilesResults := <-copiedFilesChannel
		copiedFiles = copiedFiles + copiedFilesResults
		fmt.Print(displayProgressBar(copiedFiles, numberOfMaps))
	}

	elapsed := time.Since(start)

	fmt.Println("\n\nFiles copied: ", copiedFiles)

	fmt.Println("\nTime elapsed: ", elapsed)

	fmt.Println("\nProcess Finished!")
}
