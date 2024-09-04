package posfile

import (
	"fmt"
	"github.com/15226124477/coord"
	"github.com/15226124477/method"
	log "github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"html/template"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"
)

// getStatisticInfo 对数据进行统计分析
func (pf *PosFile) getStatisticInfo() {
	if len(pf.AllPointData) > 0 {
		// 1、固定率相关统计
		pf.getSolStatistic()
		// 3、固定解
		pf.getFixStatistic()
		// 4、真固定解
		pf.getRealStatistic()
		// 5、输出
		pf.ToExcelFile(pf.InPath + ".xlsx")
		// pf.ShowStatistic()
	}
}

// getSolStatistic 解状态相关统计
func (pf *PosFile) getSolStatistic() {
	// 全部解 - >  固定解
	toDrawData := make([]string, 0)
	toDrawData = append(toDrawData, fmt.Sprintf("fileType,%d,chartTitle,%s,errV,%.3f,errH,%.3f", pf.fileType, filepath.Base(pf.InPath)+"所有点", 0.0, 0.0))

	isFirstFix := false
	firstFixTime := pf.AllPointData[0].GPST
	timeDiff := make([]float64, 0)
	// 定义模板
	const tmpl = `
					<!DOCTYPE html>
					<html>
					<head>
		
					</head>
					<style>

					h1, table {
					margin-left:auto;
					margin-right:auto;
					width: 40%;
					border-collapse: collapse;
					}
					th, td {
					border: 1px solid #ddd;
					padding: 8px;
					text-align: left;
					}
					th {
					background-color: #f2f2f2;
					}
					tr:nth-child(even) {
					background-color: #f2f2f2;
					}
					</style>
					<body>
						<table border="1">
							<h1>文件信息<h1>
							<tr>
								<th>文件名</th>
								<th> {{ .Name }} </th>
							</tr>	
							<tr>
								<th>文件起始时间</th>
								<th> {{ .Start }} </th>
							</tr>	
							<tr>
								<th>文件结束时间</th>
								<th> {{ .End }} </th>
							</tr>	
							<tr>
								<th> 采集总时长 </th>
								<th> {{ .Duration }}h </th>
							</tr>	
							<tr>
								<th> 应有数据总数 </th>
								<th> {{ .AllEpoch }} </th>
							</tr>	
							<tr>
								<th> 理论丢失总数 </th>
								<th> {{ .LostEpoch }} </th>
							</tr>	
	
							<tr>
								<th> 实际历元总数 </th>
								<th> {{ .Epoch }} </th>
							</tr>	
				
							<tr>
								<th> 数据完整率 </th>
								<th> {{ .Intergrity }}%</th>
							</tr>	
							<tr>
								<th> 报告输出时间 </th>
								<th> {{ .OutTime }}</th>
							</tr>	
							
						</table>


						<table border="1">
							<h1>数据丢失详情</h1>
							<tr>
								<th>序号</th>
								<th>起始时间</th>
								<th>结束时间</th>
								<th>丢失数</th>
							</tr>
							 {{ range $i, $item := .Items }}
							<tr>
								<td> {{ $item.Key }} </td>
								<td>{{ $item.StartTime }}</td>
								<td>{{ $item.EndTime }}</td>
								<td>{{ $item.LostCount }}</td>
							</tr>
							{{end}}
						</table>
					</body>
					</html>
					`

	lostInfo := make([]coord.LostInterval, 0)
	pointButBan := 0

	for i := 0; i < len(pf.AllPointData)-1; i++ {
		// 添加时间差值
		timeDiff = append(timeDiff, pf.AllPointData[i+1].GPST.Sub(pf.AllPointData[i].GPST).Seconds())
		lostInfo = append(lostInfo, coord.LostInterval{
			StartTime: pf.AllPointData[i].GPST.Format("2006-01-02 15:04:05.000"),
			EndTime:   pf.AllPointData[i+1].GPST.Format("2006-01-02 15:04:05.000"),
			LostCount: pf.AllPointData[i+1].GPST.Sub(pf.AllPointData[i].GPST).Seconds(),
		})

		pickTime := false
		if len(pf.TimeSelect) == 0 {
			pickTime = true
		}
		for j := 0; j < len(pf.TimeSelect); j++ {
			if pf.TimeSelect[j].TimeMode == "ban" {
				pickTime = true
			} else {
				pickTime = false
			}
			const layout = "2006-01-02 15:04:05"
			// 使用 time.Parse 函数进行转换
			t1, err := time.Parse(layout, pf.TimeSelect[j].ExceptTime[0])
			if err != nil {
				log.Info("转换失败:", err)
				return
			}
			t2, err := time.Parse(layout, pf.TimeSelect[j].ExceptTime[1])
			if err != nil {
				log.Info("转换失败:", err)
				return
			}
			check := time.Date(1, 1, 1, pf.AllPointData[i].GPST.Hour(), pf.AllPointData[i].GPST.Minute(), pf.AllPointData[i].GPST.Second(), 0, time.UTC)
			before := time.Date(1, 1, 1, t1.Hour(), t1.Minute(), t1.Second(), 0, time.UTC)
			after := time.Date(1, 1, 1, t2.Hour(), t2.Minute(), t2.Second(), 0, time.UTC)
			if pf.TimeSelect[j].TimeMode == "ban" && before.Before(check) && check.Before(after) {
				pickTime = false
				// break
			} else if pf.TimeSelect[j].TimeMode == "ban" && !(before.Before(check) && check.Before(after)) {
				pickTime = true
				break
			} else if pf.TimeSelect[j].TimeMode == "pick" && (before.Before(check) && check.Before(after)) {
				pickTime = true
				break
			} else if pf.TimeSelect[j].TimeMode == "pick" && !(before.Before(check) && check.Before(after)) {
				pickTime = false
				// break
			}

		}
		if pickTime == false {
			pointButBan = pointButBan + 1
			continue
		}

		// 固定解
		fixMode := false
		if pf.AllPointData[i].Sol.SolMode == coord.POS && pf.AllPointData[i].Sol.SolValue == 1 {
			fixMode = true
			// POS 类数据
		} else if pf.AllPointData[i].Sol.SolMode == coord.GGA && pf.AllPointData[i].Sol.SolValue == 4 {
			// GGA 类数据
			fixMode = true
		}
		if fixMode == true {
			// 统计首次固定时间
			if !isFirstFix {
				firstFixTime = pf.AllPointData[i].GPST
				isFirstFix = true
			}
			// 提取固定解数据
			pf.FixPointData = append(pf.FixPointData, pf.AllPointData[i])
		}
		line := fmt.Sprintf("%s,%.4f,%.4f,%.4f,%d,%d,%.4f", pf.AllPointData[i].GPST.Format("2006-01-02 15:04:05.000"), pf.AllPointData[i].CoordinateNEZ.N, pf.AllPointData[i].CoordinateNEZ.E, pf.AllPointData[i].CoordinateNEZ.Z, pf.AllPointData[i].Sol.SolValue, pf.AllPointData[i].SatNum, pf.AllPointData[i].DiffValue)

		// mLog.LostInterval(line)
		toDrawData = append(toDrawData, line)

	}

	// 时间差值排序
	sort.Float64s(timeDiff)

	pf.BanPoint = int64(pointButBan)
	pf.Epoch = int64(len(pf.AllPointData)) - pf.BanPoint // 历元总数
	pf.Start = pf.AllPointData[0].GPST                   // 开始记录时间
	pf.End = pf.AllPointData[pf.Epoch-1].GPST            // 结束记录时间
	pf.Sample = timeDiff[pf.Epoch/2]                     // 采样间隔
	pf.Intergrity = method.Decimal(100.0*(1-((pf.End.Sub(pf.Start).Seconds())/pf.Sample+1-float64(pf.Epoch))/((1+pf.End.Sub(pf.Start).Seconds())/pf.Sample)), 2)

	/*
		start.Debug(pf.Start)
		start.Debug(pf.End)
		start.Debug(pf.Epoch)
		start.Debug(pf.End.Sub(pf.Start))
	*/
	pf.Duration = method.Decimal(pf.End.Sub(pf.Start).Hours(), 2)                           // 文件记录周期单位h
	pf.TimeInfo.Ttff = method.Decimal(firstFixTime.Sub(pf.Start).Seconds(), 2)              // 首次固定时间
	pf.FixInfo.Fix = int64(len(pf.FixPointData))                                            // 固定解总数
	pf.FixInfo.FixRate = method.Decimal(float64(pf.FixInfo.Fix*100.0)/float64(pf.Epoch), 2) // 固定率

	realInfo := make([]coord.LostInterval, 0)
	key := 1
	// 剔除丢点信息
	for i := 0; i < len(lostInfo); i++ {
		if lostInfo[i].LostCount == pf.Sample {
			continue
		} else {
			lostInfo[i].LostCount = (lostInfo[i].LostCount / pf.Sample) - 1
			lostInfo[i].Key = key
			key = key + 1
			realInfo = append(realInfo, lostInfo[i])
		}
	}
	html := coord.LostHtmlFormat{
		Start:      pf.Start.Format("2006-01-02 15:01:05.000"),
		End:        pf.End.Format("2006-01-02 15:01:05.000"),
		OutTime:    time.Now().Format("2006-01-02 15:04:05.000"),
		Epoch:      pf.Epoch,
		Intergrity: pf.Intergrity,
		Duration:   pf.Duration,
		AllEpoch:   pf.End.Sub(pf.Start).Seconds()/pf.Sample + 1,
		LostEpoch:  pf.End.Sub(pf.Start).Seconds()/pf.Sample + 1 - float64(pf.Epoch),
		Name:       filepath.Base(pf.InPath),
		Items:      realInfo}
	// 创建模板实例
	t := template.Must(template.New("LostHtmlFormat").Parse(tmpl))
	// 创建文件
	file, err := os.Create(pf.InPath + ".ggaLost.html")
	if err != nil {
		panic(err)
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			log.Error(err)
		}
	}(file)

	// 执行模板并将结果写入文件
	err = t.Execute(file, html)
	if err != nil {
		panic(err)
	}
	// 执行模板，并将结果写入到标准输出

}

// getDiffStatistic 统计差分相关数据
func (pf *PosFile) getDiffStatistic(solDiff []float64) {
	diffDic := method.ListCount(solDiff)
	// start.Debug(diffDic)
	for arr, hash := range diffDic {
		if hash >= 1 {
			if arr <= 1 {
				pf.DiffInfo.Diff1 = pf.DiffInfo.Diff1 + int64(hash)
				pf.DiffInfo.Diff2 = pf.DiffInfo.Diff2 + int64(hash)
				pf.DiffInfo.Diff3 = pf.DiffInfo.Diff3 + int64(hash)
				pf.DiffInfo.Diff5 = pf.DiffInfo.Diff5 + int64(hash)
			} else if arr <= 2 {
				pf.DiffInfo.Diff2 = pf.DiffInfo.Diff2 + int64(hash)
				pf.DiffInfo.Diff3 = pf.DiffInfo.Diff3 + int64(hash)
				pf.DiffInfo.Diff5 = pf.DiffInfo.Diff5 + int64(hash)
			} else if arr <= 3 {
				pf.DiffInfo.Diff3 = pf.DiffInfo.Diff3 + int64(hash)
				pf.DiffInfo.Diff5 = pf.DiffInfo.Diff5 + int64(hash)
			} else if arr <= 5 {
				pf.DiffInfo.Diff5 = pf.DiffInfo.Diff5 + int64(hash)
			}

		}
	}
	pf.DiffInfo.Diff1Rate = method.Decimal(100.0*float64(pf.DiffInfo.Diff1)/float64(pf.Fix), 2)
	pf.DiffInfo.Diff2Rate = method.Decimal(100.0*float64(pf.DiffInfo.Diff2)/float64(pf.Fix), 2)
	pf.DiffInfo.Diff3Rate = method.Decimal(100.0*float64(pf.DiffInfo.Diff3)/float64(pf.Fix), 2)
	pf.DiffInfo.Diff5Rate = method.Decimal(100.0*float64(pf.DiffInfo.Diff5)/float64(pf.Fix), 2)
}

// OutDrawData 生成绘图数据
func (pf *PosFile) OutDrawData(fPath string, lines []string) {
	// start.Warning("生成绘图数据", fPath)
	method.WriteFile(fPath, lines)
}

// getFixStatistic 统计固定相关统计
func (pf *PosFile) getFixStatistic() {
	// 固定解 - > 真固定解
	solDiff := make([]float64, 0)
	nFixValue := make([]float64, 0)
	eFixValue := make([]float64, 0)
	zFixValue := make([]float64, 0)
	toDrawFixData := make([]string, 0)
	toDrawFixData = append(toDrawFixData, fmt.Sprintf("fileType,%d,chartTitle,%s,errV,%.3f,errH,%.3f", pf.fileType, filepath.Base(pf.InPath)+"筛选固定解", 0.0, 0.0))
	toDrawRealData := make([]string, 0)
	toDrawRealData = append(toDrawRealData, fmt.Sprintf("fileType,%d,chartTitle,%s,errV,%.3f,errH,%.3f", pf.fileType, filepath.Base(pf.InPath)+"筛选固定解并剔除固定错误", pf.ErrPlane, pf.ErrAltitude))

	// 定义模板
	const tmpl = `
					<!DOCTYPE html>
					<html>
					<head>
		
					</head>
					<style>

					h1, table {
					margin-left:auto;
					margin-right:auto;
					width: 80%;
					border-collapse: collapse;
					}
					th, td {
					border: 1px solid #ddd;
					padding: 8px;
					text-align: left;
					}
					th {
					background-color: #f2f2f2;
					}
					tr:nth-child(even) {
					background-color: #f2f2f2;
					}
					</style>
					<body>
						<table border="1">
							<h1>文件信息<h1>
							<tr>
								<th>文件名</th>
								<th> {{ .Name }} </th>
							</tr>
	
							<tr>
								<th> 固定历元总数 </th>
								<th> {{ .Fix }} </th>
							</tr>
							<tr>
								<th> 固定错误总数 </th>
								<th> {{ .FixErr }} </th>
							</tr>
							<tr>
								<th> 固定错误率 </th>
								<th> {{ .FixErrRate }} </th>
							</tr>
							<tr>
								<th> 平面限差 </th>
								<th> {{ .ErrV }}m</th>
							</tr>
							<tr>
								<th> 高程限差 </th>
								<th> {{ .ErrH }}m</th>
							</tr>
							<tr>
								<th> 统计方式 </th>
								<th> {{ .Mode }} </th>
							</tr>
							<tr>
								<th> N参考值 </th>
								<th> {{ .RefN }} </th>
							</tr>
							<tr>
								<th> E参考值 </th>
								<th> {{ .RefE }} </th>
							</tr>
							<tr>
								<th> Z参考值 </th>
								<th> {{ .RefZ }} </th>
							</tr>
							<tr>
								<th> 报告输出时间 </th>
								<th> {{ .OutTime }}</th>
							</tr>	

						</table>


						<table border="1">
							<h1>精度统计分析</h1>
							<tr>
								<th>序号</th>
								<th>采集时间</th>
								<th>N</th>
								<th>E</th>
								<th>Z</th>
								<th>δN</th>
								<th>δE</th>
								<th>δNE</th>
								<th>δZ</th>
								<th>固定错误</th>
							</tr>
							 {{ range $i, $item := .Items }}
							<tr>
								<td> {{ $item.Key }} </td>
								<td>{{ $item.GPST }}</td>
								<td>{{ $item.N }}</td>
								<td>{{ $item.E }}</td>
								<th>{{ $item.Z }}</th>
								<td>{{ $item.DeltaN }}</td>
								<td>{{ $item.DeltaE }}</td>
								<td>{{ $item.DeltaNE }}</td>
								<th>{{ $item.DeltaZ }}</th>
								<th>{{ $item.IsErr }}</th>
							</tr>
							{{end}}
						</table>
					</body>
					</html>
					`

	for i := 0; i < len(pf.FixPointData); i++ {
		solDiff = append(solDiff, pf.FixPointData[i].Diff.DiffValue)
		nFixValue = append(nFixValue, pf.FixPointData[i].CoordinateNEZ.N)
		eFixValue = append(eFixValue, pf.FixPointData[i].CoordinateNEZ.E)
		zFixValue = append(zFixValue, pf.FixPointData[i].CoordinateNEZ.Z)
		line := fmt.Sprintf("%s,%.4f,%.4f,%.4f,%d,%d,%.4f", pf.FixPointData[i].GPST.Format("2006-01-02 15:04:05.000"), pf.FixPointData[i].CoordinateNEZ.N, pf.FixPointData[i].CoordinateNEZ.E, pf.FixPointData[i].CoordinateNEZ.Z, pf.FixPointData[i].Sol.SolValue, pf.FixPointData[i].SatNum, pf.FixPointData[i].DiffValue)
		// start.Debug(line)
		// mLog.LostInterval(line)
		toDrawFixData = append(toDrawFixData, line)
	}
	// 统计差分相关数据
	pf.getDiffStatistic(solDiff)

	// n e z 三方向均值
	pf.PointAve.CoordinateNEZ.N = method.Average(nFixValue)
	pf.PointAve.CoordinateNEZ.E = method.Average(eFixValue)
	pf.PointAve.CoordinateNEZ.Z = method.Average(zFixValue)

	html := coord.FixHtmlFormat{
		Name:       filepath.Base(pf.InPath),
		Fix:        int64(len(toDrawFixData)),
		FixErr:     pf.Bad,
		OutTime:    time.Now().Format("2006-01-02 15:04:05.000"),
		FixErrRate: fmt.Sprintf("%.3f%%", pf.BadRate),
		ErrV:       pf.ErrPlane,
		ErrH:       pf.ErrAltitude,
		Mode:       "",
		RefN:       "",
		RefE:       "",
		RefZ:       "",
	}
	if pf.IsAveLimit {
		html.Mode = "内符合"
		html.RefN = fmt.Sprintf("%.3f", pf.PointAve.N)
		html.RefE = fmt.Sprintf("%.3f", pf.PointAve.E)
		html.RefZ = fmt.Sprintf("%.3f", pf.PointAve.CoordinateNEZ.Z)
	} else {
		html.Mode = "外符合"
		html.RefN = fmt.Sprintf("%.3f", pf.PointRef.N)
		html.RefE = fmt.Sprintf("%.3f", pf.PointRef.E)
		html.RefZ = fmt.Sprintf("%.3f", pf.PointRef.CoordinateNEZ.Z)
	}
	fixInfo := make([]coord.FixInterval, 0)

	for i := 0; i < len(pf.FixPointData); i++ {
		line := fmt.Sprintf("%s,%.4f,%.4f,%.4f,%d,%d,%.4f", pf.FixPointData[i].GPST.Format("2006-01-02 15:04:05.000"), pf.FixPointData[i].CoordinateNEZ.N, pf.FixPointData[i].CoordinateNEZ.E, pf.FixPointData[i].CoordinateNEZ.Z, pf.FixPointData[i].Sol.SolValue, pf.FixPointData[i].SatNum, pf.FixPointData[i].DiffValue)
		// 满足平面限差 和 高程限差
		fi := coord.FixInterval{
			Key:  i + 1,
			GPST: pf.FixPointData[i].GPST.Format("2006-01-02 15:04:05.000"),
			N:    fmt.Sprintf("%.3f", pf.FixPointData[i].N),
			E:    fmt.Sprintf("%.3f", pf.FixPointData[i].E),
			Z:    fmt.Sprintf("%.3f", pf.FixPointData[i].CoordinateNEZ.Z),
		}

		if pf.IsAveLimit == true && math.Sqrt(math.Pow(pf.FixPointData[i].N-pf.PointAve.CoordinateNEZ.N, 2)+math.Pow(pf.FixPointData[i].E-pf.PointAve.CoordinateNEZ.E, 2)) < pf.LimitInfo.ErrPlane && math.Abs(pf.FixPointData[i].CoordinateNEZ.Z-pf.PointAve.CoordinateNEZ.Z) <= pf.LimitInfo.ErrAltitude {

			fi.DeltaN = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].N-pf.PointAve.CoordinateNEZ.N, 2)))
			fi.DeltaE = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].E-pf.PointAve.CoordinateNEZ.E, 2)))
			fi.DeltaZ = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].CoordinateNEZ.Z-pf.PointAve.CoordinateNEZ.Z, 2)))
			fi.DeltaNE = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].N-pf.PointAve.CoordinateNEZ.N, 2)+math.Pow(pf.FixPointData[i].E-pf.PointAve.CoordinateNEZ.E, 2)))
			fi.IsErr = false
			fixInfo = append(fixInfo)
			pf.RealPointData = append(pf.RealPointData, pf.FixPointData[i])
			toDrawRealData = append(toDrawRealData, line)
		} else if pf.IsAveLimit == false {
			for j := 0; j < len(pf.PointRefs); j++ {
				if math.Sqrt(math.Pow(pf.FixPointData[i].N-pf.PointRefs[j].CoordinateNEZ.N, 2)+math.Pow(pf.FixPointData[i].E-pf.PointRefs[j].CoordinateNEZ.E, 2)) < pf.LimitInfo.ErrPlane && math.Abs(pf.FixPointData[i].CoordinateNEZ.Z-pf.PointRefs[j].CoordinateNEZ.Z) <= pf.LimitInfo.ErrAltitude {
					fi.DeltaN = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].N-pf.PointRef.CoordinateNEZ.N, 2)))
					fi.DeltaE = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].E-pf.PointRef.CoordinateNEZ.E, 2)))
					fi.DeltaZ = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].CoordinateNEZ.Z-pf.PointRef.CoordinateNEZ.Z, 2)))
					fi.DeltaNE = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].N-pf.PointRef.CoordinateNEZ.N, 2)+math.Pow(pf.FixPointData[i].E-pf.PointRef.CoordinateNEZ.E, 2)))
					fi.IsErr = false
					pf.RealPointData = append(pf.RealPointData, pf.FixPointData[i])
					toDrawRealData = append(toDrawRealData, line)
					break
				}
			}

		}

		if fi.IsErr == true {
			// 点位超限
			if pf.IsAveLimit {
				fi.DeltaN = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].N-pf.PointAve.CoordinateNEZ.N, 2)))
				fi.DeltaE = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].E-pf.PointAve.CoordinateNEZ.E, 2)))
				fi.DeltaZ = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].CoordinateNEZ.Z-pf.PointAve.CoordinateNEZ.Z, 2)))
				fi.DeltaNE = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].N-pf.PointAve.CoordinateNEZ.N, 2)+math.Pow(pf.FixPointData[i].E-pf.PointAve.CoordinateNEZ.E, 2)))
				fi.IsErr = true
			} else {
				fi.DeltaN = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].N-pf.PointRef.CoordinateNEZ.N, 2)))
				fi.DeltaE = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].E-pf.PointRef.CoordinateNEZ.E, 2)))
				fi.DeltaZ = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].CoordinateNEZ.Z-pf.PointRef.CoordinateNEZ.Z, 2)))
				fi.DeltaNE = fmt.Sprintf("%.3f", math.Sqrt(math.Pow(pf.FixPointData[i].N-pf.PointRef.CoordinateNEZ.N, 2)+math.Pow(pf.FixPointData[i].E-pf.PointRef.CoordinateNEZ.E, 2)))
				fi.IsErr = true
			}
		}
		// 添加
		fixInfo = append(fixInfo, fi)
	}

	drawRealDataPath := path.Join(pf.OutFolder, filepath.Base(pf.InPath)+".Real")
	pf.OutDrawData(drawRealDataPath, toDrawRealData)
	pf.CreatChart(drawRealDataPath, drawRealDataPath)

	html.Items = fixInfo
	t := template.Must(template.New("FixHtmlFormat").Parse(tmpl))
	// 创建文件
	file, err := os.Create(pf.InPath + ".fixInfo.html")
	if err != nil {
		panic(err)
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			log.Error(err)
		}
	}(file)

	// 执行模板并将结果写入文件
	err = t.Execute(file, html)
	if err != nil {
		panic(err)
	}
	// 执行模板，并将结果写入到标准输出
}

// 统计真固定相关统计
func (pf *PosFile) getRealStatistic() {
	// 错误点数
	pf.FixInfo.Bad = pf.Fix - int64(len(pf.RealPointData))
	pf.FixInfo.BadRate = method.Decimal(100*(1-float64(len(pf.RealPointData))/float64(pf.Fix)), 2)
	// RMS 和 Sigma

	realPointN := make([]float64, 0)
	realPointE := make([]float64, 0)
	realPointZ := make([]float64, 0)

	realPointDeltaN2E2 := make([]float64, 0)
	realPointDeltaZ2 := make([]float64, 0)

	realPointDeltaN := make([]float64, 0)
	realPointDeltaE := make([]float64, 0)

	realPointDeltaNE := make([]float64, 0)
	realPointDeltaZ := make([]float64, 0)
	//
	for i := 0; i < len(pf.RealPointData); i++ {
		realPointN = append(realPointN, pf.RealPointData[i].Coordinate.CoordinateNEZ.N)
		realPointE = append(realPointE, pf.RealPointData[i].Coordinate.CoordinateNEZ.E)
		realPointZ = append(realPointZ, pf.RealPointData[i].Coordinate.CoordinateNEZ.Z)
	}
	// 参考值
	realPointNAve := method.Average(realPointN)
	realPointEAve := method.Average(realPointE)
	realPointZAve := method.Average(realPointZ)
	// 判断是内符合还是外符合
	if pf.LimitInfo.IsAveLimit == false {
		for i := 0; i < len(pf.PointRefs); i++ {
			if math.Abs(pf.PointRefs[i].Coordinate.CoordinateNEZ.N-pf.PointAve.CoordinateNEZ.N) < pf.ErrPlane && math.Abs(pf.PointRefs[i].Coordinate.CoordinateNEZ.E-pf.PointAve.CoordinateNEZ.E) < pf.ErrPlane && math.Abs(pf.PointRefs[i].Coordinate.CoordinateNEZ.Z-pf.PointAve.CoordinateNEZ.Z) < pf.ErrAltitude {
				pf.PointAve.CoordinateNEZ.N = pf.PointRefs[i].Coordinate.CoordinateNEZ.N
				pf.PointAve.CoordinateNEZ.E = pf.PointRefs[i].Coordinate.CoordinateNEZ.E
				pf.PointAve.CoordinateNEZ.Z = pf.PointRefs[i].Coordinate.CoordinateNEZ.Z

				realPointNAve = pf.PointAve.CoordinateNEZ.N
				realPointEAve = pf.PointAve.CoordinateNEZ.E
				realPointZAve = pf.PointAve.CoordinateNEZ.Z
				// break
			}
		}
	} else {
		pf.PointAve.CoordinateNEZ.N = realPointNAve
		pf.PointAve.CoordinateNEZ.E = realPointEAve
		pf.PointAve.CoordinateNEZ.Z = realPointZAve
	}
	for i := 0; i < len(realPointN); i++ {
		realPointDeltaN = append(realPointDeltaN, math.Abs(realPointN[i]-realPointNAve))
		realPointDeltaE = append(realPointDeltaE, math.Abs(realPointE[i]-realPointEAve))

		realPointDeltaNE = append(realPointDeltaNE, math.Sqrt(math.Pow(realPointN[i]-realPointNAve, 2)+math.Pow(realPointE[i]-realPointEAve, 2)))
		realPointDeltaZ = append(realPointDeltaZ, math.Abs(realPointZ[i]-realPointZAve))

		realPointDeltaN2E2 = append(realPointDeltaN2E2, math.Pow(pf.RealPointData[i].CoordinateNEZ.N-realPointNAve, 2)+math.Pow(pf.RealPointData[i].CoordinateNEZ.E-realPointEAve, 2))
		realPointDeltaZ2 = append(realPointDeltaZ2, math.Pow(pf.RealPointData[i].CoordinateNEZ.Z-realPointZAve, 2))
	}
	if len(realPointDeltaNE) > 0 {

		// 1sigma
		numSigma1 := math.Ceil(float64(len(realPointDeltaNE))*0.682) - 1
		numSigma2 := math.Ceil(float64(len(realPointDeltaNE))*0.955) - 1
		numSigma3 := math.Ceil(float64(len(realPointDeltaNE))*0.997) - 1
		sort.Float64s(realPointDeltaNE)
		sort.Float64s(realPointDeltaZ)
		sort.Float64s(realPointDeltaN)
		sort.Float64s(realPointDeltaE)
		sort.Float64s(realPointDeltaN2E2)
		sort.Float64s(realPointDeltaZ2)

		pf.Sigma1.SigmaV = method.Decimal(realPointDeltaNE[int(numSigma1)], 3)
		pf.Sigma1.SigmaH = method.Decimal(realPointDeltaZ[int(numSigma1)], 3)
		pf.Sigma1.SigmaN = method.Decimal(realPointDeltaN[int(numSigma1)], 3)
		pf.Sigma1.SigmaE = method.Decimal(realPointDeltaE[int(numSigma1)], 3)

		// 2sigma
		pf.Sigma2.SigmaV = method.Decimal(realPointDeltaNE[int(numSigma2)], 3)
		pf.Sigma2.SigmaH = method.Decimal(realPointDeltaZ[int(numSigma2)], 3)
		pf.Sigma2.SigmaN = method.Decimal(realPointDeltaN[int(numSigma2)], 3)
		pf.Sigma2.SigmaE = method.Decimal(realPointDeltaE[int(numSigma2)], 3)

		// 3sigma
		pf.Sigma3.SigmaV = method.Decimal(realPointDeltaNE[int(numSigma3)], 3)
		pf.Sigma3.SigmaH = method.Decimal(realPointDeltaZ[int(numSigma3)], 3)
		pf.Sigma3.SigmaN = method.Decimal(realPointDeltaN[int(numSigma3)], 3)
		pf.Sigma3.SigmaE = method.Decimal(realPointDeltaE[int(numSigma3)], 3)

		pf.RmsV = method.Decimal(math.Sqrt(method.Average(realPointDeltaN2E2)), 3)
		pf.RmsH = method.Decimal(math.Sqrt(method.Average(realPointDeltaZ2)), 3)
	} else {
		pf.Sigma1 = SigmaInfo{
			SigmaN: math.NaN(),
			SigmaE: math.NaN(),
			SigmaH: math.NaN(),
			SigmaV: math.NaN(),
		}
		pf.Sigma2 = pf.Sigma1
		pf.Sigma3 = pf.Sigma1

		pf.RmsV = math.NaN()
		pf.RmsH = math.NaN()
	}
	log.Debug(pf.FixInfo)
}

// ShowStatistic 打印统计信息
func (pf *PosFile) ShowStatistic() {
	log.Warning("******************************")
	log.Debug("开始记录时间:", pf.TimeInfo.Start.Format("2006-01-02 15:04:05.000"))
	log.Debug("结束记录时间:", pf.TimeInfo.End.Format("2006-01-02 15:04:05.000"))
	log.Debug("记录总时间:", pf.TimeInfo.Duration, "h")
	log.Debug("首次固定耗时:", pf.TimeInfo.Ttff, "s")
	log.Debug("采样间隔:", pf.TimeInfo.Sample, "s")
	log.Debug("完整率:", pf.TimeInfo.Intergrity, "%")
	log.Warning("******************************")

	log.Debug("固定率:", pf.FixInfo.FixRate, "%")
	log.Debug("采集总历元:", pf.FixInfo.Epoch)
	log.Debug("固定解点数:", pf.FixInfo.Fix)
	log.Debug("固定错点数:", pf.Bad)
	log.Debug("固定错误率:", pf.BadRate, "%")

	log.Warning("******************************")

	log.Debug(fmt.Sprintf("RmsNE,%.3f,RmsH,%.3f", pf.RmsV, pf.RmsH))
	if pf.IsAveLimit == true {
		log.Debug(fmt.Sprintf("均值N,%.4f,均值E,%.4f,均值Z,%.4f", pf.PointAve.CoordinateNEZ.N, pf.PointAve.CoordinateNEZ.E, pf.PointAve.CoordinateNEZ.Z))

	} else {
		log.Debug(fmt.Sprintf("真值N,%.4f,真值E,%.4f,真值Z,%.4f", pf.PointRef.CoordinateNEZ.N, pf.PointRef.CoordinateNEZ.E, pf.PointRef.CoordinateNEZ.Z))
	}
	log.Debug(fmt.Sprintf("Sigma1N,%.3f,Sigma1E,%.3f,Sigma1H,%.3f,Sigma1NE,%.3f", pf.Sigma1.SigmaN, pf.Sigma1.SigmaE, pf.Sigma1.SigmaH, pf.Sigma1.SigmaV))
	log.Debug(fmt.Sprintf("Sigma2N,%.3f,Sigma2E,%.3f,Sigma2H,%.3f,Sigma2NE,%.3f", pf.Sigma2.SigmaN, pf.Sigma2.SigmaE, pf.Sigma2.SigmaH, pf.Sigma2.SigmaV))
	log.Debug(fmt.Sprintf("Sigma3N,%.3f,Sigma3E,%.3f,Sigma3H,%.3f,Sigma3NE,%.3f", pf.Sigma3.SigmaN, pf.Sigma3.SigmaE, pf.Sigma3.SigmaH, pf.Sigma3.SigmaV))
	log.Warning("******************************")
	log.Debug("差分龄期≤1s:", pf.Diff1, "占比:", pf.Diff1Rate, "%")
	log.Debug("差分龄期≤2s:", pf.Diff2, "占比:", pf.Diff2Rate, "%")
	log.Debug("差分龄期≤3s:", pf.Diff3, "占比:", pf.Diff3Rate, "%")
	log.Debug("差分龄期≤5s:", pf.Diff5, "占比:", pf.Diff5Rate, "%")
	// 写入Excel

}
func (pf *PosFile) ToExcelFile(xlsxPath string) {
	sheet1Title := map[string]string{
		"文件名":    "A",
		"开始时间":   "B",
		"结束时间":   "C",
		"记录总时间":  "D",
		"采样间隔":   "E",
		"采集总历元":  "F",
		"首次固定耗时": "G",

		"完整率":   "H",
		"固定解数":  "I",
		"固定率":   "J",
		"固定错误数": "K",
		"固定错误率": "L",

		"平面限差":    "M",
		"高程限差":    "N",
		"RmsV":    "O",
		"RmsH":    "P",
		"1sigmaV": "Q",
		"1sigmaH": "R",
		"2sigmaV": "S",
		"2sigmaH": "T",
		"3sigmaV": "U",
		"3sigmaH": "V",

		"差分龄期≤1占比": "W",
		"差分龄期≤2占比": "X",
		"差分龄期≤3占比": "Y",
		"差分龄期≤5占比": "Z",
	}
	sheet2Title := map[string]string{
		"文件名":      "A",
		"采集总历元":    "B",
		"完整率":      "C",
		"固定率":      "D",
		"固定错误率":    "E",
		"差分龄期≤1占比": "F",
		"差分龄期≤3占比": "G",
		"差分龄期≤5占比": "H",
	}
	sheet3Title := map[string]string{
		"文件名":      "A",
		"2sigmaV":  "B",
		"2sigmaH":  "C",
		"固定率":      "D",
		"固定错误率":    "E",
		"差分龄期≤5占比": "F",
		"完整率":      "G",
	}
	sheet5Title := map[string]string{
		"文件名":      "A",
		"记录总时间":    "B",
		"固定率":      "C",
		"1sigmaH":  "D",
		"1sigmaV":  "E",
		"2sigmaH":  "F",
		"2sigmaV":  "G",
		"差分龄期≤1占比": "H",
		"差分龄期≤2占比": "I",
		"差分龄期≤3占比": "J",
	}

	PosFileResult := map[string]string{
		"文件名":    filepath.Base(pf.InPath),
		"开始时间":   pf.Start.Format("2006-01-02 15:01:05.000"),
		"结束时间":   pf.End.Format("2006-01-02 15:01:05.000"),
		"记录总时间":  fmt.Sprintf("%.2fh", pf.TimeInfo.Duration),
		"采样间隔":   fmt.Sprintf("%.4fs", pf.TimeInfo.Sample),
		"采集总历元":  fmt.Sprintf("%d", pf.Epoch),
		"首次固定耗时": fmt.Sprintf("%.2fs", pf.Ttff),

		"完整率":  fmt.Sprintf("%.2f%%", pf.Intergrity),
		"固定解数": fmt.Sprintf("%d", pf.Fix),

		"固定率":   fmt.Sprintf("%.2f%%", pf.FixRate),
		"固定错误数": fmt.Sprintf("%d", pf.FixInfo.Bad),
		"固定错误率": fmt.Sprintf("%.2f%%", pf.FixInfo.BadRate),

		"平面限差":    fmt.Sprintf("%.3f", pf.ErrPlane),
		"高程限差":    fmt.Sprintf("%.3f", pf.ErrAltitude),
		"RmsV":    fmt.Sprintf("%.3f", pf.RmsV),
		"RmsH":    fmt.Sprintf("%.3f", pf.RmsH),
		"1sigmaV": fmt.Sprintf("%.3f", pf.Sigma1.SigmaV),
		"1sigmaH": fmt.Sprintf("%.3f", pf.Sigma1.SigmaH),
		"1sigmaN": fmt.Sprintf("%.3f", pf.Sigma1.SigmaN),
		"1sigmaE": fmt.Sprintf("%.3f", pf.Sigma1.SigmaE),

		"2sigmaV": fmt.Sprintf("%.3f", pf.Sigma2.SigmaV),
		"2sigmaH": fmt.Sprintf("%.3f", pf.Sigma2.SigmaH),
		"2sigmaN": fmt.Sprintf("%.3f", pf.Sigma2.SigmaN),
		"2sigmaE": fmt.Sprintf("%.3f", pf.Sigma2.SigmaE),

		"3sigmaV": fmt.Sprintf("%.3f", pf.Sigma3.SigmaV),
		"3sigmaH": fmt.Sprintf("%.3f", pf.Sigma3.SigmaH),
		"3sigmaN": fmt.Sprintf("%.3f", pf.Sigma3.SigmaN),
		"3sigmaE": fmt.Sprintf("%.3f", pf.Sigma3.SigmaE),

		"差分龄期≤1占比": fmt.Sprintf("%.2f%%", pf.Diff1Rate),
		"差分龄期≤2占比": fmt.Sprintf("%.2f%%", pf.Diff2Rate),
		"差分龄期≤3占比": fmt.Sprintf("%.2f%%", pf.Diff3Rate),
		"差分龄期≤5占比": fmt.Sprintf("%.2f%%", pf.Diff5Rate),
		// 老的精度分析
		"N真值": fmt.Sprintf("%.3f", pf.PointRef.CoordinateNEZ.N),
		"E真值": fmt.Sprintf("%.3f", pf.PointRef.CoordinateNEZ.E),
		"Z真值": fmt.Sprintf("%.3f", pf.PointRef.CoordinateNEZ.Z),

		"N平均": fmt.Sprintf("%.3f", pf.PointAve.CoordinateNEZ.N),
		"E平均": fmt.Sprintf("%.3f", pf.PointAve.CoordinateNEZ.E),
		"Z平均": fmt.Sprintf("%.3f", pf.PointAve.CoordinateNEZ.Z),
	}
	if pf.IsReboot {
		PosFileResult["固定率"] = fmt.Sprintf("%.2f%%", pf.FixSuccesRate)
	}

	log.Info(PosFileResult)
	xlsx := excelize.NewFile()
	pf.toExcelSheet(xlsx, "Sheet1", PosFileResult, sheet1Title)
	pf.toExcelSheet(xlsx, "Sheet2", PosFileResult, sheet2Title)
	pf.toExcelSheet(xlsx, "Sheet3", PosFileResult, sheet3Title)
	pf.toExcelSheet2(xlsx, "Sheet4", PosFileResult)
	pf.toExcelSheet(xlsx, "Sheet5", PosFileResult, sheet5Title)
	errSave := xlsx.SaveAs(xlsxPath)
	if errSave != nil {
		log.Warning(errSave)
	}

}

// toExcelSheet1 导出Excel表单1
func (pf *PosFile) toExcelSheet(xlsx *excelize.File, sheetName string, PosFileResult map[string]string, sheetTitle map[string]string) {
	_, err := xlsx.NewSheet(sheetName)
	if err != nil {
		log.Error(err)
		return
	}
	for key, value := range sheetTitle {
		err = xlsx.SetCellValue(sheetName, value+"1", key)
		if err != nil {
			log.Error(err)
			return
		}
		err = xlsx.SetCellValue(sheetName, value+"2", PosFileResult[key])
		if err != nil {
			log.Error(err)
			return
		}
	}
}

// toExcelSheet2 导出Excel表单2
func (pf *PosFile) toExcelSheet2(xlsx *excelize.File, sheetName string, PosFileResult map[string]string) {
	// Sheet2
	_, err := xlsx.NewSheet(sheetName)
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "A1", "文件名")
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "A2", PosFileResult["文件名"])
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "A3", "***")
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "A4", "***")
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "A5", "***")
	if err != nil {
		log.Error(err)
		return
	}

	err = xlsx.SetCellValue(sheetName, "C1", "X(m)")
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "D1", "Y(m)")
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "E1", "H(m)")
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "F1", "△d(m)")
	if err != nil {
		log.Error(err)
		return
	}
	if pf.IsAveLimit {
		err = xlsx.SetCellValue(sheetName, "B2", "平均值")
		if err != nil {
			log.Error(err)
			return
		}
		err = xlsx.SetCellValue(sheetName, "C2", PosFileResult["N平均"])
		if err != nil {
			log.Error(err)
			return
		}
		err = xlsx.SetCellValue(sheetName, "D2", PosFileResult["E平均"])
		if err != nil {
			log.Error(err)
			return
		}
		err = xlsx.SetCellValue(sheetName, "E2", PosFileResult["Z平均"])
		if err != nil {
			log.Error(err)
			return
		}
		err = xlsx.SetCellValue(sheetName, "F2", "\\")
		if err != nil {
			log.Error(err)
			return
		}
	} else {
		err = xlsx.SetCellValue(sheetName, "B2", "真值")
		if err != nil {
			log.Error(err)
			return
		}
		err = xlsx.SetCellValue(sheetName, "C2", PosFileResult["N真值"])
		if err != nil {
			log.Error(err)
			return
		}
		err = xlsx.SetCellValue(sheetName, "D2", PosFileResult["E真值"])
		if err != nil {
			log.Error(err)
			return
		}
		err = xlsx.SetCellValue(sheetName, "E2", PosFileResult["Z真值"])
		if err != nil {
			log.Error(err)
			return
		}
		err = xlsx.SetCellValue(sheetName, "F2", "\\")
		if err != nil {
			log.Error(err)
			return
		}
	}
	err = xlsx.SetCellValue(sheetName, "B3", "-σ~σ(68.2%)")
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "B4", "-2σ~2σ(95.5%)")
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "B5", "-3σ~3σ(99.7%)")
	if err != nil {
		log.Error(err)
		return
	}

	err = xlsx.SetCellValue(sheetName, "C3", PosFileResult["1sigmaN"])
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "D3", PosFileResult["1sigmaE"])
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "E3", PosFileResult["1sigmaH"])
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "F3", PosFileResult["1sigmaV"])
	if err != nil {
		log.Error(err)
		return
	}

	err = xlsx.SetCellValue(sheetName, "C4", PosFileResult["2sigmaN"])
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "D4", PosFileResult["2sigmaE"])
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "E4", PosFileResult["2sigmaH"])
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "F4", PosFileResult["2sigmaV"])
	if err != nil {
		log.Error(err)
		return
	}

	err = xlsx.SetCellValue(sheetName, "C5", PosFileResult["3sigmaN"])
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "D5", PosFileResult["3sigmaE"])
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "E5", PosFileResult["3sigmaH"])
	if err != nil {
		log.Error(err)
		return
	}
	err = xlsx.SetCellValue(sheetName, "F5", PosFileResult["3sigmaV"])
	if err != nil {
		log.Error(err)
		return
	}
}
