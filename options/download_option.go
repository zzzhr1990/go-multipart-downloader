package options

import (
	"time"

	"github.com/zzzhr1990/go-multipart-downloader/model"
)

// DownloadOption opt for download
type DownloadOption struct {
	// FileURI uri for download
	FileURI string
	// Host host for replace
	Host string
	// Header header to add
	Header map[string]string

	TimeOut time.Duration
	// NotFollowRedirect ids
	NotFollowRedirect bool

	MaxThreads int

	MaxPieceLength int64

	FileDestination string

	MaxRetryCount int

	// ForceStart will ignore previous download progress
	ForceStart bool

	RefreshURLAddressFunc func() (string, error)

	ProgressUpdateFunc func(percent int64, downloadBytes int64, totalBytes int64, downloadBytesPerSecond int64)

	Verbose bool

	Logger model.ILogger
}
