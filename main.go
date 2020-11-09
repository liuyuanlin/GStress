package main

import (
	"GStress/logger"
	"flag"

	//"runtime"
	//"math/rand"
	"strconv"
	"sync"
	"time"
)

var (
	RobotClientCfg = flag.String("cfg", "./cfg/robotClientSys0.xlsx", "The test cfg")
	LogLevel       = flag.String("log", "debug", "The log level: debug,info,warn,error")
	FileLogLevel   = flag.String("fileLog", "debug", "The File log level:debug,info,warn,error")
	WaitTime       = flag.Int("wt", 1000, "the robot start wait time")
)

func main() {
	//runtime.GOMAXPROCS(1)
	logger.AddFileFilter("GStree", "./log/gstree.log")
	defer logger.Log4.Close()
	flag.Parse()
	if *LogLevel == "error" {
		logger.AddConsoleFilterError()
	} else if *LogLevel == "warn" {
		logger.AddConsoleFilterWarn()
	} else if *LogLevel == "info" {
		logger.AddConsoleFilterInfo()
	}

	if *FileLogLevel == "error" {
		logger.AddFileFilterError("GStree", "./log/gstree.log")
	} else if *FileLogLevel == "warn" {
		logger.AddFileFilterWarn("GStree", "./log/gstree.log")
	} else if *FileLogLevel == "info" {
		logger.AddFileFilterInfo("GStree", "./log/gstree.log")
	}

	logger.Log4.Debug("<ENTER>")
	defer logger.Log4.Debug("<LEAVE>")

	var systemCfg ExcelCfg
	systemCfg.Parser(*RobotClientCfg, "robotClientSys0")

	robotTaskCfgFile := systemCfg.MExcelRows[0]["RobotTaskCfgFile"]
	var taskCfg ExcelCfg
	taskCfg.Parser(robotTaskCfgFile, "robotTask0")

	robotsCfgFile := systemCfg.MExcelRows[0]["RobotsCfgFile"]
	var RobotsCfg ExcelCfg
	RobotsCfg.Parser(robotsCfgFile, "robots0")

	//解析任务属性
	lTaskMap := make(map[int]TaskInfo)
	for _, row := range taskCfg.MExcelRows {
		var lTaskInfo TaskInfo

		//获取任务ID
		taskId, err := strconv.Atoi(row["taskId"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		lTaskInfo.MTaskId = taskId

		//获取任务类型
		taskType, err := strconv.Atoi(row["taskType"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		lTaskInfo.MTaskType = TaskType(taskType)

		//获取参数个数
		paramNum, err := strconv.Atoi(row["paramNum"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		for i := 0; i < paramNum; i++ {
			tmp0 := "param"
			tmp1 := strconv.Itoa(i + 1)
			tmp := tmp0 + tmp1

			lTaskInfo.MParm = append(lTaskInfo.MParm, row[tmp])
		}
		lTaskInfo.MCountent = row["content"]
		lTaskMap[lTaskInfo.MTaskId] = lTaskInfo

	}
	logger.Log4.Debug("lTaskInfo:%v", lTaskMap)

	//解析机器人属性
	var gRobots []*Robot
	for _, row := range RobotsCfg.MExcelRows {
		var robot Robot
		//获取机器人ID
		Uid, err := strconv.Atoi(row["Uid"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		robot.mRobotData.MUId = Uid

		robot.mRobotData.MUserName = row["UserName"]
		robot.mRobotData.MPassWord = row["Password"]

		//获取任务数量
		taskCount, err := strconv.Atoi(row["TaskCount"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		for i := 0; i < taskCount; i++ {
			tmp0 := "TaskId"
			tmp1 := strconv.Itoa(i + 1)
			tmp := tmp0 + tmp1
			//获取任务ID
			taskId, err := strconv.Atoi(row[tmp])
			if err != nil {
				logger.Log4.Error("err:%s", err)
				return
			}
			robot.mRobotData.MTaskId = append(robot.mRobotData.MTaskId, taskId)
		}
		logger.Log4.Error("RobotData:%v", robot.mRobotData)
		err = robot.Init(lTaskMap, systemCfg, "Init")
		if err != nil {
			logger.Log4.Error("UserName-%s: Init Fail", robot.mRobotData.MUserName)
			continue
		}
		gRobots = append(gRobots, &robot)

	}
	var wg sync.WaitGroup
	//启动所有机器人
	for _, robot := range gRobots {
		wg.Add(1)
		robotTmp := robot
		//lWaittime := time.Duration(rand.Int31n(int32(*WaitTime*1000)) + 200)
		lWaittime := time.Duration(int32(*WaitTime))
		time.Sleep(lWaittime * time.Millisecond)
		go func() {
			robotTmp.Work()

			wg.Done()
		}()
	}
	wg.Wait()

}
