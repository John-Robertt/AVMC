package domain

// MovieMeta 是 provider 解析得到的结构化元数据。
//
// 约束：
// - Website 必须写入最终成功 provider 的详情页 URL（也是来源标记）
// - 字段缺失允许为空，但结构必须稳定（不要为“全量字段”牺牲可维护性）
type MovieMeta struct {
	Code     Code
	Title    string
	Director string
	Studio   string // 製作商 / Maker
	Label    string // 發行商 / Label
	Series   string
	Release  string // ISO date, e.g. "2025-11-27"
	Year     int
	RuntimeM int
	Rating   float64 // 用户评分（满分 5；仅 JavDB 提供）
	Votes    int     // 评价人数

	Actors []string
	Genres []string
	Tags   []string

	Website   string
	CoverURL  string // 写入 NFO <thumb aspect="poster"> 和 cache JSON；执行层不直接消费
	FanartURL string // 执行层用于下载 fanart.jpg；当前两个 provider 均设为与 CoverURL 相同
}
