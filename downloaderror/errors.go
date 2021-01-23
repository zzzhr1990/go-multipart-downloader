package downloaderror

var (
	// AlreadyStartedError sta
	AlreadyStartedError = &DownloadError{
		ErrorInfo:  "the download already started",
		StatusCode: -1,
		SubError:   nil,
	}

	// TooManyConnectionsError like 429
	TooManyConnectionsError = &DownloadError{
		ErrorInfo:  "the download has too many connections",
		StatusCode: -2,
		SubError:   nil,
	}

	// ServerDoesNotSupportMultipart not support multipart
	ServerDoesNotSupportMultipart = &DownloadError{
		ErrorInfo:  "the server does not support multipart",
		StatusCode: -3,
		SubError:   nil,
	}

	// DestinationIsDirectory not support multipart
	DestinationIsDirectory = &DownloadError{
		ErrorInfo:  "the destination is directory",
		StatusCode: -4,
		SubError:   nil,
	}

	// TaskCanceledError not support multipart
	TaskCanceledError = &DownloadError{
		ErrorInfo:  "the task canceled",
		StatusCode: -5,
		SubError:   nil,
	}

	// TaskRetryFailedError not support multipart
	TaskRetryFailedError = &DownloadError{
		ErrorInfo:  "the task retry failed",
		StatusCode: -6,
		SubError:   nil,
	}

	// TaskCreateFileError not support multipart
	TaskCreateFileError = &DownloadError{
		ErrorInfo:  "create destnation file",
		StatusCode: -7,
		SubError:   nil,
	}
)

// DownloadError errors whern download occored
type DownloadError struct {
	ErrorInfo  string
	StatusCode int
	SubError   error
}

func (e DownloadError) Error() string {
	return e.ErrorInfo
}

// NewHTTPStatusError new s
func NewHTTPStatusError(status int) *DownloadError {
	return &DownloadError{
		ErrorInfo:  "remote server response unexcept http status code",
		StatusCode: status,
		SubError:   nil,
	}
}

// NewAlreadyStartError new s
func NewAlreadyStartError() *DownloadError {
	return AlreadyStartedError
}

// PieceDownloadError errors whern download occored
type PieceDownloadError struct {
	ErrorInfo string
	// StatusCode int
	SubError error
	Piece    int
}

func (e PieceDownloadError) Error() string {
	return e.ErrorInfo
}

// NewPieceTerminatedError mc
func NewPieceTerminatedError(piece int, info string, subError error) *PieceDownloadError {
	return &PieceDownloadError{
		ErrorInfo: info,
		// StatusCode int
		SubError: subError,
		Piece:    piece,
	}
}
