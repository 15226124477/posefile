package posfile

import (
	"bufio"
	"bytes"
	"github.com/15226124477/coord"
	"github.com/15226124477/define"
	"github.com/15226124477/method"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func contains(arr []int, target int) bool {
	for _, value := range arr {
		if value == target {
			return true
		}
	}
	return false
}

type DrawTask struct {
	GGAFile string
	GGAId   int
	GGaPng  string
}

func (t DrawTask) do() {
	cmd := exec.Command(define.Setting.Python, define.WebGGADrawPy, t.GGAFile, t.GGaPng, strconv.Itoa(t.GGAId)) // 这里的"python"应该替换为你的Python解释器路径
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.HideWindow = true
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Warning(err)
	}
	log.Debug(string(output))
}

// InitTask 初始化运行项目
func InitTask(taskchan chan<- DrawTask, tasks []DrawTask) {
	for i := 0; i < len(tasks); i++ {
		tsk := DrawTask{
			GGAFile: tasks[i].GGAFile,
			GGAId:   tasks[i].GGAId,
			GGaPng:  tasks[i].GGaPng,
		}
		taskchan <- tsk
	}
	close(taskchan)
}

func (pf *PosFile) CreatChart(dat string, png string) {
	log.Debug("绘图png", png)
	TaskFiles := make([]DrawTask, 0)
	for i := 1; i < 7; i++ {
		TaskFiles = append(TaskFiles, DrawTask{
			GGAFile: dat,
			GGAId:   i,
			GGaPng:  png,
		})
	}

	num := runtime.NumCPU()
	numWorkers := runtime.GOMAXPROCS(num) / 2
	log.Info("同时运行程序数:", num)
	taskChan := make(chan DrawTask, numWorkers)
	done := make(chan struct{}, numWorkers)
	go InitTask(taskChan, TaskFiles)
	DistributeTask(taskChan, numWorkers, done)
	CloseResult(done, numWorkers)

}

// CloseResult 结束运行操作
func CloseResult(done chan struct{}, workers int) {
	for i := 0; i < workers; i++ {
		<-done
	}
	// close(done)
}

// DistributeTask 处理分配任务操作
func DistributeTask(taskchan <-chan DrawTask, worders int, done chan struct{}) {
	for i := 0; i < worders; i++ {
		go ProcessTask(taskchan, done)
	}

}

// ProcessTask 处理任务进程的中断操作
func ProcessTask(taskchan <-chan DrawTask, done chan struct{}) {
	for t := range taskchan {
		t.do()
	}
	done <- struct{}{}
}

func (pf *PosFile) LoadFile(filePath string, fileType int) {
	pf.fileType = fileType
	// 数据初始化
	fInfo := FileInfo{
		AllPointData:  make([]PointData, 0),
		FixPointData:  make([]PointData, 0),
		RealPointData: make([]PointData, 0),
	}
	pFile := PosFile{
		FileInfo:      fInfo,
		StatisticInfo: StatisticInfo{},
	}
	if fileType == CSVFILE {
		pf.readCsvPath(filePath, pFile)
		return
	} else {
		// 打开PosPath路径
		fi, err := os.Open(filePath)
		if err != nil {
			log.Warning(err)
		}
		// 创建 Reader
		r := bufio.NewReader(fi)
		if fileType == GGAFILE {
			pf.readGgaPath(r, pFile)
		} else if fileType == POSFILE {
			pf.readPosPath(r, pFile)
		}
		err = fi.Close()
		if err != nil {
			return
		}
	}

}

func (pf *PosFile) readCsvPath(fPath string, pFile PosFile) {
	content, err := os.ReadFile(fPath)
	if err != nil {
		log.Warning(err)
	}
	// 转换字符编码
	content, err = io.ReadAll(transform.NewReader(bytes.NewReader(content), simplifiedchinese.GBK.NewDecoder()))
	if err != nil {
		log.Warning(err)
	}
	lines := strings.Split(string(content), "\n")

	for i := 1; i < len(lines); i++ {
		line := lines[i]
		rawPosPoint := strings.Split(line, ",")
		if len(rawPosPoint) >= 30 {
			// 时间
			layout := "2006-01-02 15:04:05.0"
			posTime, _ := time.Parse(layout, rawPosPoint[14])
			// 坐标
			nValue, nError := strconv.ParseFloat(rawPosPoint[2], 64)
			eValue, eError := strconv.ParseFloat(rawPosPoint[3], 64)
			zValue, zError := strconv.ParseFloat(rawPosPoint[4], 64)
			// 解状态 卫星数 差分龄期
			sol := 0
			if rawPosPoint[13] == "RTK固定解" {
				sol = 4
			} else if rawPosPoint[13] == "RTK浮动解" {
				sol = 5
			} else if rawPosPoint[13] == "单点定位" {
				sol = 1
			} else if rawPosPoint[13] == "伪距" {
				sol = 2
			} else {
				continue
			}

			sat, satErr := strconv.ParseInt(rawPosPoint[21], 10, 64)
			//
			diffValue, diffErr := strconv.ParseFloat(rawPosPoint[19], 64)
			if diffErr != nil || nError != nil || eError != nil || zError != nil || satErr != nil {
				log.Info("POS数据异常")
				break
				// 异常
			}

			point := PointData{
				GpstTime:   coord.GpstTime{GPST: posTime},
				Coordinate: coord.Coordinate{ConvertBefore: coord.NEZ, ConvertAfter: coord.NEZ},
				Sat:        coord.Sat{SatNum: int(sat)},
				Diff:       coord.Diff{DiffValue: diffValue},
				Sol:        coord.Sol{SolValue: sol, SolMode: coord.GGA},
			}

			point.Coordinate.CoordinateNEZ.N = nValue
			point.Coordinate.CoordinateNEZ.E = eValue
			point.Coordinate.CoordinateNEZ.Z = zValue
			// point.Convert()
			pFile.FileInfo.AllPointData = append(pFile.FileInfo.AllPointData, point)
			//
		}
	}
	//
	pf.AllPointData = pFile.AllPointData
	// 如果识别到的点数大于0
	pf.getStatisticInfo()
}

// readPosPath 加载PosPath 的文件
func (pf *PosFile) readPosPath(r *bufio.Reader, pFile PosFile) {
	for {
		line, err := r.ReadString('\n')
		line = strings.TrimSpace(line)
		if err != nil && err != io.EOF {
			log.Warning(err)
		}
		if err == io.EOF {
			break
		}
		rawPosPoint := strings.Fields(line)
		if len(line) > 0 && line[0] != '%' && len(rawPosPoint) > 13 {
			second := len(strings.Split(rawPosPoint[1], ".")[1])
			// 时间
			layout := "2006/01/02 15:04:05." + strings.Repeat("0", second)
			posTime, tErr := time.Parse(layout, rawPosPoint[0]+" "+rawPosPoint[1])
			if tErr != nil {
				log.Warning(tErr)
				continue
			}
			// 坐标
			ecefXValue, ecefXError := strconv.ParseFloat(rawPosPoint[2], 64)
			ecefYValue, ecefYError := strconv.ParseFloat(rawPosPoint[3], 64)
			ecefZValue, ecefZError := strconv.ParseFloat(rawPosPoint[4], 64)
			// 解状态 卫星数 差分龄期
			sol, solErr := strconv.ParseInt(rawPosPoint[5], 10, 64)
			sat, satErr := strconv.ParseInt(rawPosPoint[6], 10, 64)
			//
			diffValue, diffErr := strconv.ParseFloat(rawPosPoint[13], 64)
			if diffErr != nil || ecefXError != nil || ecefYError != nil || ecefZError != nil || solErr != nil || satErr != nil {
				log.Info("POS数据异常")
				break
				// 异常
			}
			point := PointData{
				GpstTime:   coord.GpstTime{GPST: posTime},
				Coordinate: coord.Coordinate{ConvertBefore: coord.XYZ, ConvertAfter: coord.NEZ},
				Sat:        coord.Sat{SatNum: int(sat)},
				Diff:       coord.Diff{DiffValue: diffValue},
				Sol:        coord.Sol{SolValue: int(sol), SolMode: coord.POS},
			}
			point.Coordinate.CoordinateXYZ.X = ecefXValue
			point.Coordinate.CoordinateXYZ.Y = ecefYValue
			point.Coordinate.CoordinateXYZ.Z = ecefZValue
			point.Convert()
			pFile.FileInfo.AllPointData = append(pFile.FileInfo.AllPointData, point)
			//
		}
		// continue
	}
	//
	pf.AllPointData = pFile.AllPointData
	pf.getStatisticInfo()
}

// readGgaPath 读取GGA文件
func (pf *PosFile) readGgaPath(r *bufio.Reader, pFile PosFile) {
	for {
		line, err := r.ReadString('\n')
		line = strings.TrimSpace(line)
		// start.Debug(line)
		if err != nil && err != io.EOF {
			break
			// start.Warning(err)
		}
		if err == io.EOF {
			break
		}
		dayAdd := 0
		//根据规则提取关键信息
		re, _ := regexp.Compile("[$][A-Z]{2}GGA\\S*[*][A-F|0-9]{2}")
		match := re.FindStringSubmatch(line)
		if len(match) == 1 {
			rawPosPoint := strings.Split(match[0], ",")

			if len(rawPosPoint) == 15 {

				if rawPosPoint[6] == "0" || rawPosPoint[6] == "" {
					continue
				}

				second := len(strings.Split(rawPosPoint[1], ".")[1])
				// 时间
				layout := "150405." + strings.Repeat("0", second)
				posTime, tErr := time.Parse(layout, rawPosPoint[1])

				if len(pFile.AllPointData) > 1 {
					if pFile.AllPointData[len(pFile.AllPointData)-1].GPST.Sub(posTime).Seconds() > 0 {
						dayAdd = dayAdd + 1
						posTime = posTime.AddDate(0, 0, dayAdd)
					}
				}
				if tErr != nil {
					log.Warning(tErr)
					continue
				}
				// 坐标
				latitudeValue, latitudeErr := strconv.ParseFloat(rawPosPoint[2], 64)    // 纬度
				longitudeValue, longitudeErr := strconv.ParseFloat(rawPosPoint[4], 64)  // 经度
				altitudeValue1, altitudeErr1 := strconv.ParseFloat(rawPosPoint[9], 64)  // 高程
				altitudeValue2, altitudeErr2 := strconv.ParseFloat(rawPosPoint[11], 64) // 高程改正
				satValue, satErr := strconv.Atoi(rawPosPoint[7])                        // 卫星数
				solValue, solErr := strconv.Atoi(rawPosPoint[6])                        // 解状态

				longitudeDu := math.Floor(longitudeValue / 100)
				longitudeDuDelta := (longitudeValue - longitudeDu*100) / 60
				longitudeDu = method.Decimal(longitudeDu+longitudeDuDelta, 10)

				// 纬度
				latitudeDu := math.Floor(latitudeValue / 100)
				latitudeDuDelta := (latitudeValue - latitudeDu*100) / 60
				latitudeDu = method.Decimal(latitudeDu+latitudeDuDelta, 10)

				diffValue, diffErr := strconv.ParseFloat(rawPosPoint[13], 64)

				diffIntValue, _ := strconv.Atoi(rawPosPoint[13])

				if diffErr != nil {
					diffValue = float64(diffIntValue)
				}

				if latitudeErr != nil || longitudeErr != nil || altitudeErr1 != nil || altitudeErr2 != nil || solErr != nil || satErr != nil {
					log.Info("POS数据异常")
					break
					// 异常
				}
				point := PointData{
					GpstTime:   coord.GpstTime{GPST: posTime},
					Coordinate: coord.Coordinate{ConvertBefore: coord.BLH, ConvertAfter: coord.NEZ},
					Sat:        coord.Sat{SatNum: satValue},
					Diff:       coord.Diff{DiffValue: diffValue},
					Sol:        coord.Sol{SolValue: solValue, SolMode: coord.GGA},
				}
				point.Coordinate.CoordinateBLH.B = latitudeDu
				point.Coordinate.CoordinateBLH.L = longitudeDu
				point.Coordinate.CoordinateBLH.H = altitudeValue1 + altitudeValue2
				point.Convert()

				/*
					start.Info(fmt.Sprintf("%.3f,", point.Coordinate.CoordinateNEZ.N),
						fmt.Sprintf("%.3f,", point.Coordinate.CoordinateNEZ.E),
						fmt.Sprintf("%.3f,", point.Coordinate.CoordinateNEZ.Z),
						point.Coordinate.CoordinateBLH.B,
						point.Coordinate.CoordinateBLH.L)
				*/
				pFile.FileInfo.AllPointData = append(pFile.FileInfo.AllPointData, point)
				//
			}
		}
	}
	if len(pFile.AllPointData) == 0 {
		// start.Warning("退出", pFile.InPath)
		return
	}
	pf.AllPointData = pFile.AllPointData

	rebootCount := 0
	successFix := 0
	// 判断固定次率
	existFixPointList := make([]int, 0)
	if pf.IsReboot {
		for i := 0; i < len(pf.AllPointData)-1; i++ {
			if pf.AllPointData[i+1].GPST.Sub(pf.AllPointData[i].GPST).Seconds() > 20 {
				rebootCount = rebootCount + 1
				// start.Info(rebootCount, pf.AllPointData[i+1].GPST, pf.AllPointData[i].GPST)
				if contains(existFixPointList, 4) {
					successFix = successFix + 1
					// start.Info(successFix, "固定次数+1")
				}
				existFixPointList = make([]int, 0)
			} else {
				existFixPointList = append(existFixPointList, pf.AllPointData[i].SolValue)
			}

		}
		// start.Debug(rebootCount, successFix)
		pf.FixInfo.GroupCount = int64(rebootCount)
		pf.FixInfo.FixSuccesRate = method.Decimal(float64(successFix*100)/float64(rebootCount), 2)
	}
	// 如果识别到的点数大于0
	pf.getStatisticInfo()
}

/*
	func DecimalToDMS(decimalDegrees float64) string {
		degrees := int(decimalDegrees)
		minutes := int((decimalDegrees - float64(degrees)) * 60)
		seconds := (decimalDegrees - float64(degrees) - float64(minutes)/60) * 3600

		return fmt.Sprintf("%d°%02d'%f\"", degrees, minutes, seconds)
	}
*/
