package downloader

import "os"

func (md *MultipartDownloader) getTempFilePath(isLogFile bool) string {
	if isLogFile {
		return md.opt.FileDestination + ".qz.tmp!"
	}
	return md.opt.FileDestination + ".qz!"
}

func (md *MultipartDownloader) clearTempPath() {
	os.RemoveAll(md.getTempFilePath(false))
	os.RemoveAll(md.getTempFilePath(true))
}
