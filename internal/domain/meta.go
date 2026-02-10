package domain

// MovieMeta 是 provider 解析得到的结构化元数据（最小可用集）。
//
// 约束：
// - Website 必须写入最终成功 provider 的详情页 URL（也是来源标记）
// - 字段缺失允许为空，但结构必须稳定（不要为“全量字段”牺牲可维护性）
type MovieMeta struct {
	Code     Code
	Title    string
	Studio   string
	Series   string
	Release  string // ISO date, e.g. "2025-11-27"
	Year     int
	RuntimeM int

	Actors []string
	Genres []string
	Tags   []string

	Website   string
	CoverURL  string
	FanartURL string
}
