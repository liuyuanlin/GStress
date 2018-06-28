package main

import (
	"GStress/logger"
	"errors"
	"fmt"
	"os"
	"time"

	sq "github.com/yireyun/go-queue"
)

type TaskResult int

const (
	TaskResultNone      = 0
	TaskResultSuccess   = 1
	TaskResultSocketErr = 2

	TaskResultLogin_Loginsvr_ConnectFail                = 3
	TaskResultLogin_Loginsvr_SendRegisterRequestFail    = 4
	TaskResultLogin_Loginsvr_SendRegisterRequestTimeOut = 5
	TaskResultLogin_Loginsvr_RegisterResponseFail       = 6

	TaskResultLogin_Loginsvr_SendLoginRequestFail    = 7
	TaskResultLogin_Loginsvr_SendLoginRequestTimeOut = 8
	TaskResultLogin_Loginsvr_LoginResponseFail       = 9

	TaskResultLogin_Gatesvr_ConnectFail              = 10
	TaskResultLogin_Lobbysvr_SendLoginRequestFail    = 11
	TaskResultLogin_Lobbysvr_SendLoginRequestTimeOut = 12
	TaskResultLogin_Lobbysvr_LoginResponseFail       = 13

	TaskResultClub_CfgErr                           = 14
	TaskResultClub_SendRequestCreateClubFail        = 15
	TaskResultClub_SendRequestCreateClubTimeOut     = 16
	TaskResultClub_SendRequestCreateClubReponseFail = 17

	TaskResultClub_SendRequestJoinClubFail        = 18
	TaskResultClub_SendRequestJoinClubTimeOut     = 19
	TaskResultClub_SendRequestJoinClubReponseFail = 20
)

type TaskState int

const (
	TaskStateInit = iota
	TaskStateExecuting
	TaskStateCompleted
)

type TaskType int

const (
	TaskTypeNone  = 0
	TaskTypeLogin = 101
	TaskTypeClub  = 201
	TaskTypeXzmj  = 301
)

type TaskStep int

const (
	TaskStepNone          = 0
	TaskStepRegister      = 10101
	TaskStepLoginLoginSvr = 10102
	TaskStepLoginLobbySvr = 10103

	TaskStepClubEnter = 20101

	TaskStepXzmjCreateRoom = 30101
	TaskStepXzmjEnterRoom  = 30102
	TaskStepXzmjStartGame  = 30103
)

type TaskInfo struct {
	MTaskId   int
	MTaskType TaskType
	MParm     []string
	MCountent string
}
type TaskStepReport struct {
	MTaskStep   TaskStep
	MTaskResult TaskResult
	MTaskState  TaskState
}

type TaskReport struct {
	MTaskResult       TaskResult
	MTaskState        TaskState
	MCurTaskStepPlace int
	MTaskStepReport   []TaskStepReport
}

type TaskAttr struct {
	MTaskInfo   TaskInfo
	MTaskReport TaskReport
}

type TaskMap map[int]TaskInfo

type TaskMng struct {
	MCurTask         *TaskAttr
	MTaskInfo        TaskMap
	MUnCompletedTask *sq.EsQueue
	MCompletedTask   *sq.EsQueue
	MUId             int
}

func (t *TaskMng) Init(taskMap TaskMap, robotAttr RobotAttr) error {
	logger.Log4.Info("UserId-%d:<ENTER>", robotAttr.MUId)
	defer logger.Log4.Debug("UserId-%d:<LEAVE>", robotAttr.MUId)
	var lRetErr error
	//1.初始化相关
	t.MUnCompletedTask = sq.NewQueue(1024 * 1024)
	t.MCompletedTask = sq.NewQueue(1024 * 1024)
	t.MTaskInfo = make(TaskMap)
	t.MUId = robotAttr.MUId
	t.MCurTask = nil

	for _, taskId := range robotAttr.MTaskId {
		lTaskInfo, ok := t.MTaskInfo[taskId]
		if ok {
			//任务已经添加过了
			continue
		}
		var lTaskAttr TaskAttr
		lTaskInfo = taskMap[taskId]
		lTaskAttr.MTaskInfo.MTaskId = lTaskInfo.MTaskId
		lTaskAttr.MTaskInfo.MTaskType = lTaskInfo.MTaskType
		lTaskAttr.MTaskInfo.MParm = lTaskInfo.MParm
		lTaskAttr.MTaskInfo.MCountent = lTaskInfo.MCountent
		err := t.LoadTaskStep(&lTaskAttr)
		if err != nil {
			logger.Log4.Info("任务加载失败，原因：%s", err)
			lRetErr = err
			goto END
		}
		t.MUnCompletedTask.Put(&lTaskAttr)
	}

END:
	return lRetErr

}

//加载任务具体执行步骤
func (t *TaskMng) LoadTaskStep(taskAttr *TaskAttr) error {
	logger.Log4.Debug("UserId-%d:<ENTER>", t.MUId)
	defer logger.Log4.Debug("UserId-%d:<LEAVE>", t.MUId)

	var lRetErr error
	if taskAttr == nil {
		lRetErr = errors.New("ERR_PARAM")
		return lRetErr
	}

	switch taskAttr.MTaskInfo.MTaskType {
	case TaskTypeLogin:

		//注册账号
		var lTaskStepReport0 TaskStepReport
		lTaskStepReport0.MTaskStep = TaskStepRegister
		taskAttr.MTaskReport.MTaskStepReport = append(taskAttr.MTaskReport.MTaskStepReport, lTaskStepReport0)

		//登陆登陆服务
		var lTaskStepReport1 TaskStepReport
		lTaskStepReport1.MTaskStep = TaskStepLoginLoginSvr
		taskAttr.MTaskReport.MTaskStepReport = append(taskAttr.MTaskReport.MTaskStepReport, lTaskStepReport1)

		//登陆大厅服务
		var lTaskStepReport2 TaskStepReport
		lTaskStepReport2.MTaskStep = TaskStepLoginLobbySvr
		taskAttr.MTaskReport.MTaskStepReport = append(taskAttr.MTaskReport.MTaskStepReport, lTaskStepReport2)
	case TaskTypeClub:
		//进入俱乐部
		var lTaskStepReport1 TaskStepReport
		lTaskStepReport1.MTaskStep = TaskStepClubEnter
		taskAttr.MTaskReport.MTaskStepReport = append(taskAttr.MTaskReport.MTaskStepReport, lTaskStepReport1)
	case TaskTypeXzmj:
		//创建创建房间
		var lTaskStepReport0 TaskStepReport
		lTaskStepReport0.MTaskStep = TaskStepXzmjCreateRoom
		taskAttr.MTaskReport.MTaskStepReport = append(taskAttr.MTaskReport.MTaskStepReport, lTaskStepReport0)
		//进入房间
		var lTaskStepReport1 TaskStepReport
		lTaskStepReport1.MTaskStep = TaskStepXzmjEnterRoom
		taskAttr.MTaskReport.MTaskStepReport = append(taskAttr.MTaskReport.MTaskStepReport, lTaskStepReport1)
		//开始游戏
		var lTaskStepReport2 TaskStepReport
		lTaskStepReport2.MTaskStep = TaskStepXzmjStartGame
		taskAttr.MTaskReport.MTaskStepReport = append(taskAttr.MTaskReport.MTaskStepReport, lTaskStepReport2)

	default:
		lRetErr = errors.New("ERR_TASK_TYPE")
		return lRetErr

	}

	return lRetErr
}

//报告任务结果
func (t *TaskMng) ReportTaskStepCompleteResult(taskId int, taskType TaskType, taskStep TaskStep, taskResult TaskResult) error {
	logger.Log4.Debug("UserId-%d:<ENTER>", t.MUId)
	defer logger.Log4.Debug("UserId-%d:<LEAVE>", t.MUId)
	var lRetErr error
	if t.MCurTask == nil {
		lRetErr = errors.New("ERR_NO_CUAR_TASK")
		return lRetErr
	}

	if taskId != t.MCurTask.MTaskInfo.MTaskId {
		logger.Log4.Debug("User-%d:The current task id does not match!,The report taskid is %d ,but curt task id is %d",
			t.MUId, taskId, t.MCurTask.MTaskInfo.MTaskId)
		lRetErr = errors.New("ERR_TASKID")
		return lRetErr
	}
	if taskType != t.MCurTask.MTaskInfo.MTaskType {
		logger.Log4.Debug("User-%d:The current task type does not match!,The report taskid is %d ,but curt task type is %d",
			t.MUId, taskType, t.MCurTask.MTaskInfo.MTaskType)
		lRetErr = errors.New("ERR_TASKTYPE")
		return lRetErr
	}
	curTaskStepPlace := t.MCurTask.MTaskReport.MCurTaskStepPlace
	curTaskStepReport := &(t.MCurTask.MTaskReport.MTaskStepReport[curTaskStepPlace])

	if taskStep != curTaskStepReport.MTaskStep {
		logger.Log4.Debug("User-%d:The current task step does not match!,The report taskid is %d ,but curt task step is %d",
			t.MUId, taskType, t.MCurTask.MTaskInfo.MTaskType)
		lRetErr = errors.New("ERR_TASKStep")
		return lRetErr
	}
	curTaskStepReport.MTaskResult = taskResult
	curTaskStepReport.MTaskState = TaskStateCompleted

	return lRetErr
}

//获取当前任务ID
func (t *TaskMng) GetCurTaskId() (int, error) {
	logger.Log4.Debug("UserId-%d:<ENTER>", t.MUId)
	defer logger.Log4.Debug("UserId-%d:<LEAVE>", t.MUId)
	var lRetErr error

	if t.MCurTask == nil {
		logger.Log4.Debug("UserId-%d:no cur task", t.MUId)
		lRetErr = errors.New("ERR_PARAM")
		return 0, lRetErr
	}
	return t.MCurTask.MTaskInfo.MTaskId, nil
}

//分发任务步骤
func (t *TaskMng) DispatchTaskStep() (TaskStep, error) {
	logger.Log4.Debug("UserId-%d:<ENTER>", t.MUId)
	defer logger.Log4.Debug("UserId-%d:<LEAVE>", t.MUId)
	var lRetErr error
	var taskStep TaskStep = TaskStepNone
	if t.MCurTask == nil {
		logger.Log4.Debug("UserId-%d:no cur task", t.MUId)
		lRetErr = errors.New("ERR_PARAM")
		return taskStep, lRetErr
	}
	curTaskStepPlace := t.MCurTask.MTaskReport.MCurTaskStepPlace
	curTaskStepReport := &(t.MCurTask.MTaskReport.MTaskStepReport[curTaskStepPlace])
	if TaskStateCompleted == curTaskStepReport.MTaskState {
		if TaskResultSuccess != curTaskStepReport.MTaskResult ||
			curTaskStepPlace >= (len(t.MCurTask.MTaskReport.MTaskStepReport)-1) {
			//任务意外中断或全部完成都算完成状态
			taskStep = TaskStepNone
			t.MCurTask.MTaskReport.MTaskResult = curTaskStepReport.MTaskResult
			t.MCurTask.MTaskReport.MTaskState = TaskStateCompleted
		} else {
			t.MCurTask.MTaskReport.MCurTaskStepPlace++
			curTaskStepPlace = t.MCurTask.MTaskReport.MCurTaskStepPlace
			taskStep = t.MCurTask.MTaskReport.MTaskStepReport[curTaskStepPlace].MTaskStep
			t.MCurTask.MTaskReport.MTaskStepReport[curTaskStepPlace].MTaskState = TaskStateExecuting
		}
	} else {
		taskStep = curTaskStepReport.MTaskStep
		curTaskStepReport.MTaskState = TaskStateExecuting

	}
	return taskStep, lRetErr
}

//分发任务
func (t *TaskMng) DispatchTask() (TaskType, error) {
	logger.Log4.Debug("UserId-%d:<ENTER>", t.MUId)
	defer logger.Log4.Debug("UserId-%d:<LEAVE>", t.MUId)
	var lRetErr error
	var taskType TaskType = TaskTypeNone

	if t.MCurTask != nil {
		t.OutputTaskReport()
		t.MCompletedTask.Put(t.MCurTask)
		t.MCurTask = nil
	}
	if t.MUnCompletedTask.Quantity() > 0 {
		task, ok, quantity := t.MUnCompletedTask.Get()
		if !ok {
			logger.Log4.Error("UserId-%d:Get Task Fail,the UnCompleted Size is %d", t.MUId, quantity)
		} else {
			t.MCurTask = task.(*TaskAttr)
			taskType = t.MCurTask.MTaskInfo.MTaskType
			t.OutputTaskReport()
		}
	}

	return taskType, lRetErr
}

func (t *TaskMng) GetTaskPrarm(place int) string {
	logger.Log4.Debug("UserId-%d:<ENTER>", t.MUId)
	defer logger.Log4.Debug("UserId-%d:<LEAVE>", t.MUId)
	if place < 0 {
		return ""
	}
	if t.MCurTask == nil {
		return ""
	}
	if place >= len(t.MCurTask.MTaskInfo.MParm) {
		return ""
	}

	return t.MCurTask.MTaskInfo.MParm[place]

}

func (t *TaskMng) GetTaskPrarmNum() int {
	logger.Log4.Debug("UserId-%d:<ENTER>", t.MUId)
	defer logger.Log4.Debug("UserId-%d:<LEAVE>", t.MUId)

	if t.MCurTask == nil {
		return 0
	}

	return len(t.MCurTask.MTaskInfo.MParm)

}
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (t *TaskMng) OutputTaskReport() {
	logger.Log4.Debug("UserId-%d:<ENTER>", t.MUId)
	defer logger.Log4.Debug("UserId-%d:<LEAVE>", t.MUId)

	//判断文件夹是否存在
	isExist, _ := PathExists("./log")
	if !isExist {
		os.Mkdir("./log", os.ModeDir|os.ModePerm)
	}

	var fleName string
	fleName = fmt.Sprintf("./log/robotTaskReport_%d.txt", t.MUId)
	f, err := os.OpenFile(fleName, os.O_RDWR|os.O_CREATE|os.O_APPEND, os.ModeAppend|os.ModePerm)
	if err != nil {
		logger.Log4.Error("UserId-%d:Err:", t.MUId, err)
		return
	}
	defer f.Close()
	//记录头信息
	recordTime := fmt.Sprintf("\n===============%d report time:%s========\n", t.MUId, time.Now().Format("2006-01-02 15:04:05"))
	_, err = f.WriteString(recordTime)
	if err != nil {
		logger.Log4.Error("UserId-%d:Err:", t.MUId, err)
		return
	}
	//记录任务信息
	taskInfo := fmt.Sprintf(`
	taskId                             %-15d
	taskType                           %-15d
	taskContent                        %-15s
	taskState                          %-15d
	taskResult                         %-15d
	taskStepNum                        %-15d
	taskCurStepNum                     %-15d
	`,
		t.MCurTask.MTaskInfo.MTaskId,
		t.MCurTask.MTaskInfo.MTaskType,
		t.MCurTask.MTaskInfo.MCountent,
		t.MCurTask.MTaskReport.MTaskState,
		t.MCurTask.MTaskReport.MTaskResult,
		len(t.MCurTask.MTaskReport.MTaskStepReport),
		t.MCurTask.MTaskReport.MCurTaskStepPlace+1,
	)
	_, err = f.WriteString(taskInfo)
	if err != nil {
		logger.Log4.Error("UserId-%d:Err:", t.MUId, err)
		return
	}
	//记录单步任务状态
	for i := 0; i <= t.MCurTask.MTaskReport.MCurTaskStepPlace; i++ {
		//记录任务信息
		taskStepInfo := fmt.Sprintf(`
		curTaskStepNum                     %-15d
		taskStep                           %-15d
		taskStepState                      %-15d
		taskStepResult                     %-15d
		`,
			i+1,
			t.MCurTask.MTaskReport.MTaskStepReport[i].MTaskStep,
			t.MCurTask.MTaskReport.MTaskStepReport[i].MTaskState,
			t.MCurTask.MTaskReport.MTaskStepReport[i].MTaskResult,
		)
		_, err = f.WriteString(taskStepInfo)
		if err != nil {
			logger.Log4.Error("UserId-%d:Err:", t.MUId, err)
			return
		}
	}
}
