package checkinapp

import (
	"sync"
	"time"
)

// shanghaiLocation 是签到统一业务时区（Asia/Shanghai）。
// 服务器运行在哪个时区都不能改变签到业务日。
// 兜底 FixedZone 仅在 LoadLocation 意外失败（系统 tzdata 缺失）时使用，
// 不作为长期业务时区实现。
var (
	shanghaiOnce sync.Once
	shanghaiLoc  *time.Location
)

func shanghaiLocation() *time.Location {
	shanghaiOnce.Do(func() {
		loc, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			shanghaiLoc = time.FixedZone("CST", 8*3600)
			return
		}
		shanghaiLoc = loc
	})
	return shanghaiLoc
}

// BusinessDate 把任意时刻换算为 Asia/Shanghai 业务日，并以 UTC 零点 time.Time 表示。
// 同一业务日（00:00:00 ~ 23:59:59 Asia/Shanghai）必然得到同一个返回值；
// 数据库列 type:date 序列化为 "YYYY-MM-DD"。
func BusinessDate(now time.Time) time.Time {
	year, month, day := now.In(shanghaiLocation()).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// FormatBusinessDate 输出业务日 "YYYY-MM-DD"。
func FormatBusinessDate(date time.Time) string {
	return date.Format("2006-01-02")
}

// BusinessDayRange 返回给定时刻所在业务日的 [from,to) UTC 边界。
// 全项目"今日"口径的唯一权威实现：积分运营统计等跨域聚合一律复用本函数，
// 不得在别处再写一套 Asia/Shanghai 日界计算。
func BusinessDayRange(now time.Time) (time.Time, time.Time) {
	from := BusinessDate(now)
	return from, from.AddDate(0, 0, 1)
}

// SystemClock 是生产时钟（返回真实当前时刻）。
type SystemClock struct{}

// Now 返回当前时刻。
func (SystemClock) Now() time.Time { return time.Now() }

// FixedClock 是测试时钟：固定返回注入时刻，保证时区/跨月/跨年测试确定性。
type FixedClock struct {
	At time.Time
}

// Now 返回固定时刻。
func (c FixedClock) Now() time.Time { return c.At }
