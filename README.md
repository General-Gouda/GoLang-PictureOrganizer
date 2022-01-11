<a name="top"></a>
# Picture Organizer

This program will help you sort through the media files within a given directory path.

You provide a source path and a destination path. It will gather information about all of the media files in the source location, weed out any duplicates (based on MD5 hash of the file), and sort the pictures into the destination folder based on Year/Month.

For instance, if a picture within the source directory was created on August 1st, 2009 it will be sorted into folder `{destinationpath}\2009\August`
- - - -

|THING|LINK/DESCRIPTION|
|---|---|
|Language|Go 1.17.6|
|Authors|[Matt Marchese](https://github.com/General-Gouda)|

[Installation](#installation)

[How to Use](#howtouse)

- - - -

<a name="installation"></a>
## Installation ##

Download the latest .exe file from the [Releases](https://github.com/General-Gouda/GoLang-PictureOrganizer/releases) page and put it somewhere on your computer.

[[Back to top]](#top)

- - - -

<a name="howtouse"></a>
## How to Use on Windows ##

Open up your favorite command line tool, change directory to where the PictureOrganizer.exe file exists and run the following.

```
.\PictureOrganizer.exe -p "C:\Temp\SourcePath" -d "C:\Temp\DestinationPath"
```

Optionally, you can specify the number of goroutine workers to deploy using the `-w` parameter. Default is set to 10 if nothing is specified.

```
.\PictureOrganizer.exe -p "C:\Temp\SourcePath" -d "C:\Temp\DestinationPath" -w 20
```

## How to Use on Linux ##
Download the source code, install [Go](https://go.dev/doc/install) and run:

```
go build ./PictureOrganizer.go
```

This should create an executable file within the same folder. You can then run the application using the same argument flags that are listed above `-p -d and -w`

[[Back to top]](#top)
