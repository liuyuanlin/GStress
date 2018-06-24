游戏压力测试系统
	fsm::stack fsm;
	CSocket mSocket;
	deque<RobotEventData> mFsmEventQueue;
	TaskMng mTaskMng;
	TimerMap mTimers;
	unsigned int mCurTaskId;
	TaskType mCurTaskType;
	TaskStep mCurTaskStep;
	TaskResult mCurTaskStepReuslt;
	SystemConfig mSystemConfig;
	RobotData mRobotData;
    RobotMjTable mRobotTable;
    bool mIsWorkEnd;
	
	
	struct RobotEventData
{
	RobotState state = RobotStateNone;
	RobotEvent event = RobotEventNone;
	const void * data = NULL;
};