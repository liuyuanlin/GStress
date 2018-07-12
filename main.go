package main

import (
	"GStress/logger"
	"flag"
	//"runtime"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

var (
	RobotClientCfg = flag.String("cfg", "./cfg/robotClientSys0.xlsx", "The test cfg")
	LogLevel       = flag.String("log", "debug", "The log level: debug,info,warn,error")
	FileLogLevel   = flag.String("fileLog", "debug", "The File log level:debug,info,warn,error")
	WaitTime       = flag.Int("wt", 2, "the robot start wait time")
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
		var robotAttr RobotAttr
		//获取机器人ID
		Uid, err := strconv.Atoi(row["Uid"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		robotAttr.MUId = Uid
		//获取机器人类型
		userType, err := strconv.Atoi(row["UserType"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		robotAttr.MUserType = userType
		//获取机器人是否自动登陆
		isNeedAutoLogin, err := strconv.Atoi(row["IsNeedAutoLogin"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		if isNeedAutoLogin == 0 {
			robotAttr.MIsNeedAutoLogin = false
		} else {
			robotAttr.MIsNeedAutoLogin = true
		}

		robotAttr.MUserName = row["UserName"]
		robotAttr.MPassWord = row["Password"]
		robotAttr.MDevice = row["Device"]
		robotAttr.MClientIp = row["ClientIp"]
		robotAttr.MPackageflag = row["Packagefla"]
		robotAttr.MTencentToken = row["TencentToken"]
		robotAttr.MTencentCodeId = row["TencentCodeId"]

		//获取血战麻将桌子id
		xzmjTableId, err := strconv.Atoi(row["XzmjTableId"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		robotAttr.MXzmjTableId = xzmjTableId

		//获取血战麻将房间id
		xzmjRoomId, err := strconv.Atoi(row["XzmjRoomId"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		robotAttr.MXzmjRoomId = xzmjRoomId

		//获取斗地主桌子id
		ddzTableId, err := strconv.Atoi(row["DdzTableId"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		robotAttr.MDdzTableId = ddzTableId

		//获取斗地主房间id
		ddzRoomId, err := strconv.Atoi(row["DdzRoomId"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		robotAttr.MDdzRoomId = ddzRoomId

		//获取微游戏ID
		wantClubId, err := strconv.Atoi(row["ClubId"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		robotAttr.MWantClubId = wantClubId

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
			robotAttr.MTaskId = append(robotAttr.MTaskId, taskId)
		}
		logger.Log4.Error("robotAttr:%v", robotAttr)
		err = robot.Init(robotAttr, lTaskMap, systemCfg, "Init")
		if err != nil {
			logger.Log4.Error("UserName-%s: Init Fail", robotAttr.MUserName)
			continue
		}
		gRobots = append(gRobots, &robot)

	}
	var wg sync.WaitGroup
	//启动所有机器人
	for _, robot := range gRobots {
		wg.Add(1)
		robotTmp := robot
		lWaittime := time.Duration(rand.Int31n(int32(*WaitTime*1000)) + 200)
		time.Sleep(lWaittime * time.Millisecond)
		go func() {
			robotTmp.Work()

			wg.Done()
		}()
	}
	wg.Wait()

}
