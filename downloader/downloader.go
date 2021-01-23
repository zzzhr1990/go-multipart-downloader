package downloader

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	context2 "github.com/zzzhr1990/go-multipart-downloader/contexts"
	"github.com/zzzhr1990/go-multipart-downloader/downloaderror"
	"github.com/zzzhr1990/go-multipart-downloader/model"
	"github.com/zzzhr1990/go-multipart-downloader/options"
	"github.com/zzzhr1990/go-multipart-downloader/status"
	"github.com/zzzhr1990/go-multipart-downloader/utils"
)

// MultipartDownloader downloader
type MultipartDownloader struct {
	opt              *options.DownloadOption
	contentLength    int64
	errorChan        chan error
	downloadPieces   []*model.PieceInfo
	supportMultiPart bool
	//fileDestination          string
	httpClient *http.Client

	// stopped bool
	statusMutex sync.Mutex
	status      int32
}

// CreateNew new instance
func CreateNew(opt *options.DownloadOption, errorChan chan error) *MultipartDownloader {

	if opt == nil {
		panic("cannot init with nil options")
	}

	if opt.MaxRetryCount < 1 {
		opt.MaxRetryCount = 1 // Should be 0? no retry
	}

	return &MultipartDownloader{
		opt:              opt,
		downloadPieces:   []*model.PieceInfo{},
		supportMultiPart: true,
		// errorChan: errorChan,
		httpClient:  http.DefaultClient,
		status:      status.Waiting,
		errorChan:   errorChan,
		statusMutex: sync.Mutex{},
	}
}

// startSync Download sync
func (md *MultipartDownloader) startSync() {

	// PREV CHECK
	parent := filepath.Dir(md.opt.FileDestination)

	fi, err := os.Stat(parent)
	if err != nil {
		if os.IsNotExist(err) {
			// return false
			// not existe
			err := os.MkdirAll(parent, 0755)
			if err != nil {
				log.Printf("cannot check dest dir: %v, %v", parent, err)
				md.errorChan <- downloaderror.TaskCreateFileError
				return
			}
			//
		} else {
			log.Printf("cannot check dest dir: %v, %v", parent, err)
			md.errorChan <- downloaderror.TaskCreateFileError
			return
		}
		// no per,

	} else {
		if !fi.IsDir() {
			md.errorChan <- downloaderror.TaskCreateFileError
			return
		}
	}

	if md.opt.TimeOut < time.Second {
		md.opt.TimeOut = time.Second * 30
	}
	tryTime := 0
	err = md.getFileInfo()

	for err != nil && tryTime < md.opt.MaxRetryCount {
		tryTime++
		if md.opt.RefreshURLAddressFunc != nil {
			ret, err := md.opt.RefreshURLAddressFunc()
			if err != nil {
				log.Println("cannot refresh download URL", err)
			} else {
				md.opt.FileURI = ret
			}
		}
		err = md.getFileInfo()
	}

	if err != nil {
		md.errorChan <- err
		return
	}

	// calc pieces
	if md.opt.MaxThreads < 1 {
		md.opt.MaxThreads = 1
	}

	// threadCount := 0

	var startPos int64 = 0

	if md.opt.MaxPieceLength < 1 {
		md.opt.MaxPieceLength = 8388608 // 8388608=8MB
	}

	if md.opt.MaxPieceLength < 0 {
		md.opt.MaxPieceLength = 8388608 // 8388608=8MB
	}

	currentIndex := 0
	if md.contentLength > 0 {
		for {
			endPos := startPos + md.opt.MaxPieceLength - 1 // Note: CLOSE
			if endPos >= (md.contentLength - 1) {
				endPos = md.contentLength - 1
				// current: [startPos-endPos]
				// log.Printf("[%v] - [%v]", startPos, endPos)
				md.downloadPieces = append(md.downloadPieces, &model.PieceInfo{
					StartPos: startPos,
					EndPos:   endPos,
					Index:    currentIndex,
				})
				currentIndex++
				break
			}
			// current: [startPos-endPos]
			md.downloadPieces = append(md.downloadPieces, &model.PieceInfo{
				StartPos: startPos,
				EndPos:   endPos,
				Index:    currentIndex,
			})
			currentIndex++
			startPos = endPos + 1
		}
	} else {
		md.supportMultiPart = false
		md.downloadPieces = append(md.downloadPieces, &model.PieceInfo{
			StartPos: 0,
			EndPos:   -1,
			Index:    currentIndex,
		})
	}

	if len(md.downloadPieces) < 1 {
		md.errorChan <- nil
		return
	}

	var pc int64 = 0
	for _, v := range md.downloadPieces {
		pc += (v.EndPos - v.StartPos + 1)
	}
	// trying to read tmp file...

	if len(md.downloadPieces) > 0 {

		if !utils.FileExists(md.getTempFilePath(false)) || !utils.FileExists(md.getTempFilePath(true)) {
			md.clearTempPath()
		}
		if !md.supportMultiPart || md.opt.ForceStart {
			md.clearTempPath()
		}

		file, err := os.OpenFile(md.getTempFilePath(true), os.O_RDONLY, 0644)
		if err == nil {
			b, err := ioutil.ReadAll(file)
			if err == nil {
				d := []*model.PieceInfo{}
				err := json.Unmarshal(b, &d)
				if err == nil {
					if len(d) == len(md.downloadPieces) {
						failedCheck := false
						for idx, det := range md.downloadPieces {
							chk := d[idx]
							if chk.StartPos != det.StartPos || chk.EndPos != det.EndPos {
								md.clearTempPath()
								failedCheck = true
								break
							}
							det.CompletedBytes = chk.CompletedBytes
							if chk.CompletedBytes > chk.EndPos-chk.StartPos+1 {
								panic("ooo " + strconv.Itoa(idx))
							}
							log.Printf("turn complete index: [%v] to: %v", idx, det.CompletedBytes)
						}
						if failedCheck {
							for _, det := range md.downloadPieces {
								det.CompletedBytes = 0
							}
						}
					} else {
						md.clearTempPath()
					}
				} else {
					md.clearTempPath()
				}
			} else {
				md.clearTempPath()
			}
		} else {
			md.clearTempPath()
		}
	}

	md.errorChan <- md.doDownload(!md.supportMultiPart)
}

func (md *MultipartDownloader) downPiece(index int, ch chan error, retry bool) {
	go func() {
		ch <- md.downPieceSync(index, retry)
	}()
}

// StartAsync asy
func (md *MultipartDownloader) StartAsync() (chan error, error) {
	// If it started, throw error..
	if md.errorChan == nil {
		md.errorChan = make(chan error)
		//
	}
	md.statusMutex.Lock()
	defer md.statusMutex.Unlock()
	if md.GetStatus() == status.Downloading {
		return nil, downloaderror.AlreadyStartedError
	}
	atomic.StoreInt32(&md.status, status.Downloading)
	go func() {
		md.startSync()
	}()
	return md.errorChan, nil
}

func (md *MultipartDownloader) getFileInfo() error {

	// 1. We first, check if server support HEAD
	//transport := http.Transport{
	//	Dial: md.opt.TimeOut,
	// }

	ctt := &context2.TimeWrapper{
		Time: time.Now().Add(md.opt.TimeOut),
	}
	ctx, cancel := context2.WithDeadline(context.Background(), ctt)
	defer cancel()
	req, err := md.newRequestWithContext(ctx, http.MethodHead)
	if err != nil {
		log.Printf("cannot create request: %v, => %v", md.opt.FileURI, err)
		return err
	}
	// client := md.newClient()
	// defer client.CloseIdleConnections()
	response, err := md.httpClient.Do(req)
	if err != nil {
		log.Printf("cannot do request: %v, => %v", md.opt.FileURI, err)
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		// return downloaderror.NewHTTPStatusError(response.StatusCode)
		md.contentLength = response.ContentLength
		return nil
	}
	ctt2 := &context2.TimeWrapper{
		Time: time.Now().Add(md.opt.TimeOut),
	}
	ctx2, cancel2 := context2.WithDeadline(context.Background(), ctt2)
	defer cancel2()
	req, err = md.newRequestWithContext(ctx2, http.MethodGet)
	if err != nil {
		log.Printf("cannot create 2nd request: %v, => %v", md.opt.FileURI, err)
		return err
	}
	response, err = md.httpClient.Do(req)
	if err != nil {
		log.Printf("cannot do 2dn request: %v, => %v", md.opt.FileURI, err)
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		// return downloaderror.NewHTTPStatusError(response.StatusCode)
		md.contentLength = response.ContentLength
		return nil
	}
	log.Printf("unexcept error code: %v", response.StatusCode)
	return downloaderror.NewHTTPStatusError(response.StatusCode)
}

func (md *MultipartDownloader) newRequestUnsafe(method string) (*http.Request, error) {
	uri := md.opt.FileURI
	req, err := http.NewRequest(method, uri, nil)
	if err != nil {
		return nil, err
	}
	if len(md.opt.Host) > 0 {
		req.Host = md.opt.Host
	}
	if md.opt.Header != nil {
		for k, v := range md.opt.Header {
			req.Header.Add(k, v)
		}
	}

	return req, nil
}

func (md *MultipartDownloader) newRequestWithContext(context context.Context, method string) (*http.Request, error) {
	uri := md.opt.FileURI
	req, err := http.NewRequestWithContext(context, method, uri, nil)
	if err != nil {
		return nil, err
	}
	if len(md.opt.Host) > 0 {
		req.Host = md.opt.Host
	}
	if md.opt.Header != nil {
		for k, v := range md.opt.Header {
			req.Header.Add(k, v)
		}
	}

	return req, nil
}

/*
func (md *MultipartDownloader) newClient() *http.Client {
	//CheckRedirect: func(req *http.Request, via []*http.Request) error {
	//    return http.ErrUseLastResponse
	//},
	tw := md.opt.TimeOut
	if tw < time.Second {
		tw = time.Minute
	}
	client := http.Client{
		Transport: http.DefaultTransport,
		Timeout:   tw,
	}
	return &client
}
*/

// GetContentLength get content
func (md *MultipartDownloader) GetContentLength() int64 {
	return md.contentLength
}

// Cancel cancel download
func (md *MultipartDownloader) Cancel() {
	atomic.StoreInt32(&md.status, status.Canceled)
}

// Pause the same with cancel
func (md *MultipartDownloader) Pause() {
	atomic.StoreInt32(&md.status, status.Paused)
}
