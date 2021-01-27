package downloader

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	context2 "github.com/zzzhr1990/go-multipart-downloader/contexts"
	"github.com/zzzhr1990/go-multipart-downloader/downloaderror"
)

func (md *MultipartDownloader) downPieceSync(index int, retry bool) error {
	if md.opt.Verbose {
		log.Printf("starting download piece %v", index)
		defer log.Printf("end download piece %v", index)
	}

	if !md.shouldContinue() {
		return downloaderror.TaskCanceledError
	}
	f := md.downloadPieces[index]
	f.Downloading = true
	defer func() {
		f.Downloading = false
	}()
	// 429 403 503 Seems you need reduce your thread...
	if index > 0 && !md.supportMultiPart && !retry {
		return downloaderror.ServerDoesNotSupportMultipart
	}

	startOffset := f.StartPos

	completedBytes := atomic.LoadInt64(&f.CompletedBytes)
	if md.supportMultiPart {
		if completedBytes > 0 {
			startOffset = startOffset + completedBytes
		}

		if startOffset > f.EndPos {
			f.Completed = true
			return nil
		}
	} else {
		startOffset = 0
	}

	if md.opt.Verbose {
		log.Printf("starting download [%v]: %v-%v", index, startOffset, f.EndPos)
	}

	ctt := &context2.TimeWrapper{
		Time: time.Now().Add(md.opt.TimeOut),
	}
	ctx, cancel := context2.WithDeadline(context.Background(), ctt)
	defer cancel()
	req, err := md.newRequestWithContext(ctx, http.MethodGet)
	if err != nil {
		return downloaderror.NewPieceTerminatedError(index, err.Error(), err)
	}

	if md.supportMultiPart {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(startOffset, 10)+"-"+strconv.FormatInt(f.EndPos, 10))
	}

	// f.CompletedBytes = 0

	resp, err := md.httpClient.Do(req)

	if err != nil {
		if md.opt.Verbose {
			log.Printf("error download piece: %v => %v", index, err)
		}
		return downloaderror.NewPieceTerminatedError(index, err.Error(), err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		if md.opt.Verbose {
			log.Printf("error download piece http code error: %v => %v", index, resp.StatusCode)
		}
		return downloaderror.NewHTTPStatusError(resp.StatusCode)
	}

	if resp.StatusCode == http.StatusOK && !retry {
		if md.opt.Verbose {
			log.Printf("error download piece: %v => %v, server does not support multipart", index, resp.StatusCode)
		}
		return downloaderror.ServerDoesNotSupportMultipart
	}

	file, err := os.OpenFile(md.getTempFilePath(false), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		if md.opt.Verbose {
			log.Printf("error download piece: %v => %v, cannot open file", index, err)
		}
		return downloaderror.NewPieceTerminatedError(index, err.Error(), err)
	}

	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		if md.opt.Verbose {
			log.Printf("error download piece: %v => %v, cannot stat file", index, err)
		}
		return downloaderror.NewPieceTerminatedError(index, err.Error(), err)
	}
	if fi.IsDir() {
		err = downloaderror.DestinationIsDirectory
		if md.opt.Verbose {
			log.Printf("error download piece: %v => %v, destination is a directory", index, err)
		}
		return downloaderror.NewPieceTerminatedError(index, err.Error(), err)
	}

	_, err = file.Seek(startOffset, 0)
	if err != nil {
		if md.opt.Verbose {
			log.Printf("cannot get download piece [%v]: %v", index, err)
		}
		return downloaderror.NewPieceTerminatedError(index, err.Error(), err)
	}

	// writer := bufio.NewWriter(file)
	//
	/*
		for true {
			if err != nil && err != io.EOF {
				// Here Retry...
				log.Printf("error download piece: %v => %v, progress ", index, err)
				return err
			}
			// block.retryCount = 0
			i64 := int64(len(buffer[:i]))
			needSize := block.EndOffset + 1 - block.BeginOffset
			if i64 > needSize {
				i64 = needSize
				err = io.EOF
			}
			_, e := writer.Write(buffer[:i64])
			if e != nil {
				block.Downloading = false
				client.callFailed(e)
				ch <- false
				return
			}
			block.BeginOffset += i64
			block.DownloadedSize += i64
			client.DownloadedSize += i64
			if err == io.EOF || block.BeginOffset > block.EndOffset {
				block.Completed = true
				break
			}
			i, err = resp.Body.Read(buffer)
		}
	*/

	// fileWritten, err := io.Copy(file, resp.Body)

	size := 32 * 1024
	buf := make([]byte, size)

	var written int64 = 0
	for {
		if !md.shouldContinue() {
			return downloaderror.TaskCanceledError
		}
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			// client.Timeout
			// ctt2 := ctt.Add(time.Second * 10)

			ctt.Time = time.Now().Add(md.opt.TimeOut)

			nw, ew := file.Write(buf[0:nr])
			if nw > 0 {
				wl := int64(nw)
				written += wl
				// atomic.StoreInt64(&f.CompletedBytes, 1234454)
				cur := atomic.AddInt64(&f.CompletedBytes, wl)
				if cur > f.EndPos-f.StartPos+1 && md.supportMultiPart {
					rangeBytes := "bytes=" + strconv.FormatInt(startOffset, 10) + "-" + strconv.FormatInt(f.EndPos, 10)

					log.Printf("exp Range!! %v -> %v, exp: %v, resp len: %v, cur: %v", index, rangeBytes, f.EndPos-f.StartPos+1, resp.ContentLength, cur)
					panic("oooxxx " + strconv.Itoa(index))
				}
				if f.Trytime > 0 {
					f.Trytime = 0
				}
			}
			if ew != nil {
				// log.Printf("this is write err: %v\n", ew)
				if md.opt.Verbose {
					log.Printf("doing copy download piece [%v]: %v", index, ew)
				}
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}

	if err != nil {
		if md.opt.Verbose {
			log.Printf("cannot copy download piece [%v]: %v", index, err)
		}
		return downloaderror.NewPieceTerminatedError(index, err.Error(), err)
	}

	if resp.ContentLength > 0 {
		if written != resp.ContentLength {

			log.Printf("cannot match content length [%v]: write: %v => need: %v", index, written, resp.ContentLength)
			// return err
		}
	}

	f.Completed = true
	if md.opt.Verbose {
		log.Printf("complete download piece [%v]: %v => block completed: %v", index, resp.ContentLength, f.CompletedBytes)
	}
	return nil
}
