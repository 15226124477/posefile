package posfile

import (
	"github.com/15226124477/coord"
	"time"
)

const (
	GGAFILE = 1
	CSVFILE = 2
	POSFILE = 3
)

type PointData struct {
	coord.GpstTime   // 点时间
	coord.Coordinate // 点坐标
	coord.Sat        // 点卫星数
	coord.Diff       // 点差分
	coord.Sol        // 点解状态
}

type FileInfo struct {
	AllPointData  []PointData // 所有点数据
	FixPointData  []PointData // 固定解数据
	RealPointData []PointData // 真固定数据
}

type DiffInfo struct {
	Diff1     int64   // 差分龄期小于等于1个数
	Diff1Rate float64 // 差分龄期小于等于1占比
	Diff2     int64   // 差分龄期小于等于2个数
	Diff2Rate float64 // 差分龄期小于等于2占比
	Diff3     int64   // 差分龄期小于等于3个数
	Diff3Rate float64 // 差分龄期小于等于3占比
	Diff5     int64   // 差分龄期小于等于5个数
	Diff5Rate float64 // 差分龄期小于等于5占比
}

type RmsInfo struct {
	RmsH float64 // 高程RMS
	RmsV float64 // 平面RMS
}

type TimeSelect struct {
	TimeMode   string   `json:"TimeMode"`
	ExceptTime []string `json:"ExceptTime"`
}

type LimitInfo struct {
	IsAveLimit  bool    // 是否限制
	ErrPlane    float64 // 平面限差
	ErrAltitude float64 // 高程限差
}

type TimeInfo struct {
	Start      time.Time // 开始时间
	End        time.Time // 结束时间
	Duration   float64   // 时间跨度
	Sample     float64   // 采样间隔
	Intergrity float64   // 数据完整率
	Ttff       float64   // 首次固定时间

}

type FixInfo struct {
	Epoch         int64   // 历元总数
	BanPoint      int64   // 有但是跳过的点数
	Fix           int64   // 固定点数
	FixRate       float64 // 固定率
	GroupCount    int64   // 初始化组数
	FixSuccesRate float64 // 固定次率
	Bad           int64   // 错误点数
	BadRate       float64 // 固定错误率
}

type SigmaInfo struct {
	Grade  int
	SigmaN float64 // N方向Sigma
	SigmaE float64 // E方向Sigma
	SigmaH float64 // H方向Sigma
	SigmaV float64 // 平面Sigma
}

type StatisticInfo struct {
	TimeInfo           // 时间信息
	FixInfo            // 固定解信息
	RmsInfo            // RMS 精度
	DiffInfo           // 差分信息
	Sigma1   SigmaInfo // 1 sigma
	Sigma2   SigmaInfo // 2 sigma
	Sigma3   SigmaInfo // 3 sigma
}

type PosFile struct {
	InPath    string // 生成路径
	fileType  int    // 文件类型
	OutFolder string // 生成的所在文件夹

	TimeSelect []TimeSelect // 时间限制 Input
	LimitInfo               // 点位限制 Input

	IsReboot bool // is reboot mode

	PointRefs []PointData // 诸多参考值
	PointRef  PointData   // 参考值点
	PointAve  PointData   // 平均值
	FileInfo              // 点信息

	StatisticInfo // 统计信息
}
