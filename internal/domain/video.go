package domain

// VideoFile 描述一次扫描得到的视频文件（只做 stat，不读内容）。
//
// 不变量（实现必须遵守）：
// - AbsPath 必须是 clean + absolute
// - 扫描阶段只做 stat，不读文件内容
type VideoFile struct {
	AbsPath string
	RelPath string
	Base    string // filename without ext
	Ext     string // ".mp4"
	Size    int64
	ModUnix int64
}
