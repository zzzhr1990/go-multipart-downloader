package status

// DownloadStatus idc status
type DownloadStatus int32

const (
	// Waiting waiting for download start
	Waiting int32 = iota
	// Downloading file is downloading
	Downloading
	// Completed file completed
	Completed
	// Canceled file download cancel / paused
	Canceled
	// Paused Download paused (the same as cancelled, you can restart it again)
	Paused
	// Error Download error
	Error
)
