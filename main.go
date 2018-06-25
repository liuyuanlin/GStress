package main

import (
	"GStress/logger"
	"strconv"
	"sync"
)

func main() {
	logger.AddFileFilter("GStree", "gstree.log")
	defer logger.Log4.Close()

	logger.Log4.Debug("<ENTER>")
	defer logger.Log4.Debug("<LEAVE>")

	var systemCfg ExcelCfg
	systemCfg.Parser("./cfg/robotClientSys0.xlsx", "robotClientSys0")
	var taskCfg ExcelCfg
	taskCfg.Parser("./cfg/robotTask0.xlsx", "robotTask0")
	var RobotsCfg ExcelCfg
	RobotsCfg.Parser("./cfg/robots0.xlsx", "robots0")

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
		userid, err := strconv.Atoi(row["Uid"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		robotAttr.MUserId = userid
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
		robotAttr.MTencentToken = row["TencentToken"]
		robotAttr.MTencentCodeId = row["TencentCodeId"]

		//获取积分房间id
		appointRoomId, err := strconv.Atoi(row["AppointRoomId"])
		if err != nil {
			logger.Log4.Error("err:%s", err)
			return
		}
		robotAttr.MApointRoomId = appointRoomId

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
		go func() {
			robot.Work()
			wg.Done()
		}()
	}
	wg.Wait()

}
