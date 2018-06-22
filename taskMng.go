package main

import (
	"GStress/logger"
	"errors"

	sq "github.com/yireyun/go-queue"
)

type TaskResult int

const (
	TaskResultNone = iota
	TaskResultSuccess
	TaskResultSocketErr
)

type TaskState int

const (
	TaskStateInit = iota
	TaskStateExecuting
	TaskStateCompleted
)

type TaskType int

const (
	TaskTypeNone     = 0
	TaskTypeLogin    = 101
	TaskTypeMatch    = 201
	TaskTypeRecharge = 301
)

type TaskStep int

const (
	TaskStepNone            = 0
	TaskStepLoginAccountSvr = 10101
	TaskStepLoginGameSvr    = 10102

	TaskTypeMatchCreateRoom = 20101
	TaskTypeMatchEnterRoom  = 20102
	TaskTypeMatchStartGame  = 20103

	TaskTypeRechargeGetOrder    = 30101
	TaskTypeRechargeAuthReceipt = 30102
)

type TaskInfo struct {
	MTaskId   int32
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
	MCurTaskStepPlace int32
	MTaskStepReport   []TaskStepReport
}

type TaskAttr struct {
	MTaskInfo   TaskInfo
	MTaskReport TaskReport
}

type TaskMap map[int32]TaskInfo

type TaskMng struct {
	MCurTask         *TaskAttr
	MTaskInfo        TaskMap
	MUnCompletedTask *sq.EsQueue
	MCompletedTask   *sq.EsQueue
	MUserId          int32
}

func (t *TaskMng) Init(taskMap TaskMap, robotAttr RobotAttr) error {
	logger.Log4.Info("<ENTER> gamesvr start")
	var lRetErr error
	//1.初始化相关
	t.MUnCompletedTask = sq.NewQueue(1024 * 1024)
	t.MCompletedTask = sq.NewQueue(1024 * 1024)
	t.MTaskInfo = make(TaskMap)
	t.MUserId = robotAttr.MUserId
	t.MCurTask = nil

	for _, taskId := range robotAttr.MTaskId {
		lTaskInfo, ok := t.MTaskInfo[taskId]
		if ok {
			//任务已经添加过了
			continue
		}
		var lTaskAttr TaskAttr
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

func (t *TaskMng) LoadTaskStep(taskAttr *TaskAttr) error {
	var lRetErr error
	lRetErr = errors.New("LOAD_TASK_FAIL")

	return lRetErr
}
