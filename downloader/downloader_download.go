package downloader

import (
	"encoding/json"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/zzzhr1990/go-multipart-downloader/downloaderror"
	"github.com/zzzhr1990/go-multipart-downloader/model"
	"github.com/zzzhr1990/go-multipart-downloader/status"
	// "github.com/zzzhr1990/go-multipart-downloader/utils"
)

func (md *MultipartDownloader) shouldContinue() bool {
	return atomic.LoadInt32(&md.status) == status.Downloading
}

// GetStatus f
func (md *MultipartDownloader) GetStatus() int32 {
	return atomic.LoadInt32(&md.status)
}

func (md *MultipartDownloader) doDownload(isRetry bool) error {

	// md.stopped = false

	stopped := false
	go func() {
		lastTick := time.Now()
		var lastDownloadBytes int64 = 0
		tm := time.NewTimer(time.Second)
		for md.shouldContinue() { // Thread safe?
			select {
			case <-tm.C:
				if !md.shouldContinue() {
					return
				}
				if stopped {
					return
				}
				var totalDownload int64 = 0
				downloadSnap := []*model.PieceInfo{}
				for _, piece := range md.downloadPieces {
					dl := atomic.LoadInt64(&piece.CompletedBytes)
					totalDownload += dl
					downloadSnap = append(downloadSnap, &model.PieceInfo{
						StartPos:       piece.StartPos,
						EndPos:         piece.EndPos,
						CompletedBytes: dl,
						Index:          piece.Index,
					})
				}
				var percent int64 = 0

				if md.GetContentLength() > 0 {
					percent = totalDownload * 100 / md.GetContentLength()
					// sync
					md.syncDownloadProgress(downloadSnap)
				}

				sec := time.Since(lastTick).Seconds()
				delta := totalDownload - lastDownloadBytes

				var speedInBytes int64 = 0

				if sec > 0 {
					speedInBytes = int64(float64(delta) / sec)
					lastTick = time.Now()
					lastDownloadBytes = totalDownload
				}

				if md.opt.ProgressUpdateFunc != nil {
					go func() {
						md.opt.ProgressUpdateFunc(percent, totalDownload, md.GetContentLength(), speedInBytes)
					}()
				}

				tm.Reset(time.Second)
			}

		}
	}()

	pieceCompleteChan := make(chan error)
	totalPieces := len(md.downloadPieces)
	completedPieces := 0
	currentDownloadingIndex := -1
	currentDownloadingThread := 0
	reduceThreads := false

	if !md.supportMultiPart {
		md.opt.MaxThreads = 1
		md.downloadPieces = []*model.PieceInfo{md.downloadPieces[0]}
		md.downloadPieces[0].StartPos = 0
		md.downloadPieces[0].EndPos = -1
		md.downloadPieces[0].CompletedBytes = 0
		totalPieces = len(md.downloadPieces)
	}

	for currentDownloadingThread < md.opt.MaxThreads {

		if currentDownloadingIndex+1 > totalPieces-1 {
			break //Or Break?
		}
		currentDownloadingIndex++
		currentDownloadingThread++
		md.downPiece(currentDownloadingIndex, pieceCompleteChan, isRetry)
	}

	for completedPieces < totalPieces {

		err := <-pieceCompleteChan

		if err != nil {
			if val, ok := err.(*downloaderror.PieceDownloadError); ok {
				index := val.Piece
				if md.downloadPieces[index].Trytime < md.opt.MaxRetryCount {
					md.downloadPieces[index].Trytime = md.downloadPieces[index].Trytime + 1
					log.Printf("retrying downloading piece: %v, (%v / %v)", index, md.downloadPieces[index].Trytime, md.opt.MaxRetryCount)
					if md.opt.RefreshURLAddressFunc != nil {
						ret, err := md.opt.RefreshURLAddressFunc()
						if err != nil {
							log.Println("cannot refresh download URL", err)
						} else {
							md.opt.FileURI = ret
						}
					}
					md.downPiece(index, pieceCompleteChan, isRetry)
					continue
				} else {
					md.downloadPieces[index].Failed = true
					log.Printf("give up download piece: %v, after %v times retry", index, md.downloadPieces[index].Trytime)
				}
			}
		}

		currentDownloadingThread = currentDownloadingThread - 1
		completedPieces++
		if err != nil {
			// no, just move next
			if err == downloaderror.ServerDoesNotSupportMultipart {
				md.supportMultiPart = false
				md.opt.MaxThreads = 1
				reduceThreads = true
			}
			if err == downloaderror.TooManyConnectionsError {
				reduceThreads = true
				if md.opt.MaxThreads > 1 {
					md.opt.MaxThreads = md.opt.MaxThreads - 1
				}
			}

		}

		if currentDownloadingThread < md.opt.MaxThreads {
			if currentDownloadingIndex+1 > totalPieces-1 {
				log.Printf("no blocks left to download, concurrent: %v", currentDownloadingThread)
				continue
				//break // no file
			}
			currentDownloadingIndex++
			currentDownloadingThread++
			log.Printf("switching next downloading index: %v, concurrent: %v", currentDownloadingIndex, currentDownloadingThread)
			md.downPiece(currentDownloadingIndex, pieceCompleteChan, isRetry)
		}
	}

	log.Printf("complete download, %v, max threads: %v", reduceThreads, md.opt.MaxThreads)

	if reduceThreads && !isRetry {
		log.Printf("retrying download... %v", md.supportMultiPart)
		return md.doDownload(true)
	}

	allCompleted := true

	for index, v := range md.downloadPieces {
		if !v.Completed {
			allCompleted = false
			log.Printf("download not completed: %v", index)
		}
	}

	stopped = true

	downloadSnap := []*model.PieceInfo{}
	var totalDownload int64 = 0
	for _, piece := range md.downloadPieces {

		dl := atomic.LoadInt64(&piece.CompletedBytes)
		totalDownload += dl
		downloadSnap = append(downloadSnap, &model.PieceInfo{
			StartPos:       piece.StartPos,
			EndPos:         piece.EndPos,
			CompletedBytes: dl,
			Index:          piece.Index,
		})

	}
	var percent int64 = 0
	if md.GetContentLength() > 0 {
		percent = totalDownload * 100 / md.GetContentLength()
		md.syncDownloadProgress(downloadSnap)
	}
	if md.opt.ProgressUpdateFunc != nil {
		go func() {
			md.opt.ProgressUpdateFunc(percent, totalDownload, md.GetContentLength(), 0)
		}()
	}
	if md.shouldContinue() {

		if !allCompleted {
			atomic.StoreInt32(&md.status, status.Error)
			return downloaderror.TaskRetryFailedError
		}
		atomic.StoreInt32(&md.status, status.Completed)
		os.RemoveAll(md.getTempFilePath(true))
		os.RemoveAll(md.opt.FileDestination)
		err := os.Rename(md.getTempFilePath(false), md.opt.FileDestination)
		if err != nil {
			atomic.StoreInt32(&md.status, status.Error)
			return downloaderror.TaskCreateFileError
		}
	}
	return nil
}

func (md *MultipartDownloader) syncDownloadProgress(d []*model.PieceInfo) error {
	jData, err := json.Marshal(d)
	if err != nil {
		log.Printf("cannot marshal err: %v", err)
		return err
	}

	file, err := os.OpenFile(md.getTempFilePath(true), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("error sync progress: %v => %v, cannot open file", "tmp files", err)
		// return downloaderror.NewPieceTerminatedError(index, err.Error(), err)
		return err
	}

	defer file.Close()

	_, err = file.Write(jData)
	if err != nil {
		log.Printf("error sync progress: %v => %v, cannot write file", "tmp files", err)
	}

	return err
}
