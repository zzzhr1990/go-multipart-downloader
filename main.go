package main

import (
	"log"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/zzzhr1990/go-multipart-downloader/downloader"
	"github.com/zzzhr1990/go-multipart-downloader/options"
	"github.com/zzzhr1990/go-multipart-downloader/utils"
)

func main() {
	str, _ := os.Getwd()
	fileDestination := str + "/test/1.test.bin"
	e, err := utils.ComputeWcsFileEtag(fileDestination)
	log.Printf("etag: %v, err: %v", e, err)
}

func main2() {

	// lpLeqIj_kty1N2MpPLXt1d2cTd5p
	oneMtestFile := "https://tx-us-ping.vultr.com/vultr.com.1000MB.bin" //"https://syd-au-ping.vultr.com/vultr.com.1000MB.bin"
	//"https://hnd-jp-ping.vultr.com/vultr.com.100MB.bin" // "https://hnd-jp-ping.vultr.com/vultr.com.1000MB.bin"
	str, _ := os.Getwd()
	downloadOpthon := &options.DownloadOption{
		FileURI:         oneMtestFile,
		TimeOut:         time.Second * 20,
		MaxThreads:      10,
		FileDestination: str + "/test/1.test.bin",
		MaxPieceLength:  52428800,
		MaxRetryCount:   10,
		Host:            "",
		ProgressUpdateFunc: func(percent int64, totalDownload int64, totalBytes int64, speedInBytes int64) {
			log.Printf("downloading percent: %v, downloaded: %v/%v, speed: %v/s", percent, humanize.Bytes(uint64(totalDownload)), humanize.Bytes(uint64(totalBytes)), humanize.Bytes(uint64(speedInBytes)))
		},
		// Host:            "",
	}

	cdl := downloader.CreateNew(downloadOpthon, nil)

	errorChan, err := cdl.StartAsync()
	if err != nil {
		log.Printf("pre download error: %v", err)
		return
	}

	err = <-errorChan

	if err != nil {
		log.Printf("download error: %v", err)
		return
	}

	log.Printf("download completed: %v, status: %v, file: %v", cdl.GetContentLength(), cdl.GetStatus(), downloadOpthon.FileDestination)
}