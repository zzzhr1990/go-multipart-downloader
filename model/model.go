package model

// PieceInfo d
type PieceInfo struct {
	Index          int   `json:"index"`
	Downloading    bool  `json:"-"`
	StartPos       int64 `json:"startPos"`
	EndPos         int64 `json:"endPos"`
	CompletedBytes int64 `json:"completedBytes"`
	Completed      bool  `json:"-"`
	Trytime        int   `json:"-"`
	Failed         bool  `json:"-"`
}
