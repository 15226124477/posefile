package posfile

import (
	"bufio"
	"fmt"
	"github.com/15226124477/method"
	log "github.com/sirupsen/logrus"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type PosFileArgs struct {
	IFilePath string // 输入原始数据文件路径

	OGGAFilePath  string // 导出GGA路径
	OPOSFilePath  string // 导出POS路径
	OLostFilePath string // Rinex输出报告

	MayInterval   float64  // 评估输出频率
	LostInfo      []string // 评估丢点位置
	integrityRate float64  // 完整率

	BasicYear  int // GGA 丢失的年
	BasicMonth int // GGA 丢失的月
	BasicDay   int // GGA 丢失的日

	// rawPOS    []string      // 原始POS数据
	BasicPosInfo []PointData // 基础数据类型

	IsRef bool
	RefN  float64
	RefE  float64
	RefZ  float64
}

func (pFile *PosFileArgs) LoadRawRinex() {

	log.Debug("加载文件:", pFile.IFilePath)
	fi, err := os.Open(pFile.IFilePath)
	if err != nil {
		panic(err)
	}
	// 创建 Reader
	r := bufio.NewReader(fi)
	for {
		line, err := r.ReadString('\n')
		line = strings.TrimSpace(line)
		if err != nil && err != io.EOF {
			panic(err)
		}
		if err == io.EOF {
			break
		}
		if line[0] == '>' {
			reg := regexp.MustCompile(`\s+`)
			result := reg.Split(line, -1)
			pt := PointData{}

			year, _ := strconv.Atoi(result[1])
			month, _ := strconv.Atoi(result[2])
			day, _ := strconv.Atoi(result[3])
			hour, _ := strconv.Atoi(result[4])
			minute, _ := strconv.Atoi(result[5])

			seconds, _ := strconv.ParseFloat(result[6], 64)
			mircosecond := method.Decimal((seconds-math.Floor(seconds))*1000, 0)
			sat, _ := strconv.Atoi(result[8])

			rinexTime := time.Date(year, time.Month(month), day, hour, minute, int(seconds), int(1000*1000*mircosecond), time.UTC)
			pt.GPST = rinexTime
			pt.Sat.SatNum = sat
			// log.Info(pt.GPST.Format("2006-01-02 15:04:05.000"), "\t", pt.Sat)

			pFile.BasicPosInfo = append(pFile.BasicPosInfo, pt)
		}

	}
	intervals := make([]float64, 0)
	for i := 1; i < len(pFile.BasicPosInfo); i++ {
		interval := pFile.BasicPosInfo[i].GPST.Sub(pFile.BasicPosInfo[i-1].GPST).Seconds()
		// log.Info(pFile.BasicPosInfo[i-1].GPST.Format("2006-01-02 15:04:05.000"), " ", pFile.BasicPosInfo[i].GPST.Format("2006-01-02 15:04:05.000"), " ", interval)

		intervals = append(intervals, interval)
	}
	mayIntervals := method.ListCount(intervals)
	mayInterval := 0.0
	maxCount := 0.0

	for k, v := range mayIntervals.(map[float64]int) {
		if float64(v) > maxCount {
			mayInterval = k
			maxCount = float64(v)
		}

		log.Info("频率:", k, "次数:", v)

	}

	xlist := make([]time.Time, 0)
	y1list := make([]int, 0)
	lostInfo := make([]string, 0)
	lostSum := 0.0

	xlist = append(xlist, pFile.BasicPosInfo[0].GPST)
	y1list = append(y1list, pFile.BasicPosInfo[0].SatNum)
	for i := 1; i < len(pFile.BasicPosInfo); i++ {
		xlist = append(xlist, pFile.BasicPosInfo[i].GPST)
		y1list = append(y1list, pFile.BasicPosInfo[i].SatNum)
		interval := pFile.BasicPosInfo[i].GPST.Sub(pFile.BasicPosInfo[i-1].GPST).Seconds()

		if interval != mayInterval {
			lostSum = lostSum + (pFile.BasicPosInfo[i].GPST.Sub(pFile.BasicPosInfo[i-1].GPST).Seconds()-mayInterval)/mayInterval
			lostInfo = append(lostInfo, fmt.Sprintf("%s ~ %s #Epoch_LOST %.f\r\n",
				pFile.BasicPosInfo[i-1].GPST.Format("2006-01-02 15:04:05.000"),
				pFile.BasicPosInfo[i].GPST.Format("2006-01-02 15:04:05.000"),
				(pFile.BasicPosInfo[i].GPST.Sub(pFile.BasicPosInfo[i-1].GPST).Seconds()-mayInterval)/mayInterval,
			))

		}
		if (pFile.BasicPosInfo[i-1].SatNum - pFile.BasicPosInfo[i].SatNum) >= 3 {
			lostInfo = append(lostInfo, fmt.Sprintf("%s ~ %s #SAT_LOST %d\r\n",
				pFile.BasicPosInfo[i-1].GPST.Format("2006-01-02 15:04:05.000"),
				pFile.BasicPosInfo[i].GPST.Format("2006-01-02 15:04:05.000"),
				pFile.BasicPosInfo[i-1].SatNum-pFile.BasicPosInfo[i].SatNum,
			))
		}
	}

	log.Debug("文件频率:", mayInterval, "s")

	pFile.integrityRate = 100.0 * (float64(len(pFile.BasicPosInfo))) / (float64(len(pFile.BasicPosInfo)) + lostSum)
	log.Debug("完整率:", pFile.integrityRate, "%")
	pFile.LostInfo = append(pFile.LostInfo, fmt.Sprintf("************************************************************************************\r\n文件路径:%s\r\n起始时间:%s\r\n结束时间:%s\r\n历元总数:%d\r\n采样间隔:%.4fs\r\n完整率:%.4f%%\r\n******************************************\r\n",
		pFile.IFilePath,
		pFile.BasicPosInfo[0].GPST.Format("2006-01-02 15:04:05.000"),
		pFile.BasicPosInfo[len(pFile.BasicPosInfo)-1].GPST.Format("2006-01-02 15:04:05.000"),
		len(pFile.BasicPosInfo),
		mayInterval,
		pFile.integrityRate,
	))
	pFile.LostInfo = append(pFile.LostInfo, lostInfo...)
	pFile.ToLostReport()
	// CreateRinexChart(pFile.IFilePath+".SatNum.html", filepath.Base(pFile.IFilePath)+"卫星序列图", xlist, "卫星数", y1list, 60)
}

func (pFile *PosFileArgs) ToLostReport() {
	if len(pFile.LostInfo) == 0 {
		return
	}

	_, err := os.Stat(pFile.OLostFilePath)
	if !os.IsNotExist(err) {
		err := os.Remove(pFile.OLostFilePath)
		if err != nil {
			return
		}
	}
	file, err := os.OpenFile(pFile.OLostFilePath, os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("文件打开失败")
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	// 写入文件头
	for i := 0; i < len(pFile.LostInfo); i++ {
		var writeLine = pFile.LostInfo[i]
		// log.Info(writeLine)

		_, err2 := writer.WriteString(writeLine)
		if err2 != nil {
			return
		}
	}
	errClose := writer.Flush()
	if errClose != nil {
		return
	}
}
