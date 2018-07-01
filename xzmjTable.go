package main

import (
	"GStress/logger"
	"GStress/msg/XueZhanMj"
	"GStress/net"
	"errors"

	"github.com/golang/protobuf/proto"
)

//桌子状态
const (
	Table_State_Init          = 1
	Table_State_Start         = 2
	Table_State_Gaming        = 3
	Table_State_Game_End      = 4 //游戏结束
	Table_State_Dissolve      = 5 //桌子已解散
	Table_State_Wait_Dissolve = 6 //等待解散投票
)

//玩家状态
const (
	Mj_User_State_Init    = 1 //初始化
	Mj_User_State_Sit     = 2 //坐下
	Mj_User_State_Ready   = 3 //准备
	Mj_User_State_Playing = 4 //比赛过程中
	Mj_User_State_Hu      = 5 // 胡牌
	Mj_User_State_GiveUp  = 6 //认输
)

//扑克操作码
const (
	MJ_OPERATE_MASK_NONE           = 1
	MJ_OPERATE_MASK_DEAL           = 2  //出牌
	MJ_OPERATE_MASK_GUO            = 3  //不出牌
	MJ_OPERATE_MASK_PENG           = 4  //碰牌
	MJ_OPERATE_MASK_MING_GANG      = 5  //明杠
	MJ_OPERATE_MASK_AN_GANG        = 6  //暗杠
	MJ_OPERATE_MASK_BU_GANG        = 7  //补杠
	MJ_OPERATE_MASK_REJECTSUIT     = 8  //定缺
	MJ_OPERATE_MASK_HU             = 9  //胡牌
	MJ_OPERATE_MASK_MO_PAI         = 10 //摸牌,服务器使用
	MJ_OPERATE_MASK_GIVE_UP        = 11 //认输
	MJ_OPERATE_MASK_HUAN_SAN_ZHANG = 12 //换三张

)

//扑克结算类型
const (
	MJ_SETTLEMENT_TYPE_NONE       = 0
	MJ_SETTLEMENT_TYPE_HU         = 1  //点胡
	MJ_SETTLEMENT_TYPE_ZI_MO      = 2  //自摸
	MJ_SETTLEMENT_TYPE_QIANG_GANG = 3  //抢杠
	MJ_SETTLEMENT_TYPE_GANG_HUA   = 4  //杠上花
	MJ_SETTLEMENT_TYPE_GANG_PAO   = 5  //杠上炮
	MJ_SETTLEMENT_TYPE_GUA_FENG   = 6  //刮风
	MJ_SETTLEMENT_TYPE_XIA_YU     = 7  //下雨
	MJ_SETTLEMENT_TYPE_CHA_JIAO   = 8  //查叫
	MJ_SETTLEMENT_TYPE_TUI_SHUI   = 9  //退税
	MJ_SETTLEMENT_TYPE_HUA_ZHU    = 10 //花猪
	MJ_SETTLEMENT_TYPE_ZHUAN_YU   = 11 //转雨
	MJ_SETTLEMENT_TYPE_TIAN_HU    = 12 //天胡
	MJ_SETTLEMENT_TYPE_DI_HU      = 13 //地胡

)

//最终胡牌类型
const (
	FINAL_CARD_TYPE_NONE         = 0x00
	FINAL_CARD_TYPE_HU           = 0x01 //胡
	FINAL_CARD_TYPE_DA_DUI       = 0x02 //大对子
	FINAL_CARD_TYPE_JIANG        = 0x04 //将
	FINAL_CARD_TYPE_YAO_JIU      = 0x08 //幺九
	FINAL_CARD_TYPE_QING         = 0x10 //清一色
	FINAL_CARD_TYPE_QI_DUI       = 0x20 //七对
	FINAL_CARD_TYPE_JIN_GOU_DIAO = 0x40 //金钩吊
	FINAL_CARD_TYPE_HAI_DI_LAO   = 0x80 //海底捞月

)

type XzmjTableAttribute struct {
	MGameCurrencyType       int
	MPayRoomRateType        int
	MPlanGameCount          int
	MDiZhu                  int
	MEnterScore             int
	MLeaveScore             int
	MBeiShuLimit            int
	MChairCount             int
	MIsAllowEnterAfterStart int
	MTableType              int
	MRoomRate               int
	MServerRate             int
	// 高级参数部分
	MZiMoJiaFan              bool
	MZiMoMoreThanMaxFan      bool
	MJinGouDiao              bool
	MHaiDiLaoYue             bool
	MDaXiaoYu                bool
	MDianGangHuaZiMo         bool
	MYaoJiu                  bool
	MJiang                   bool
	MHuanSanZhangType        int
	MStartGameMinPlayerCount int
	MOwnerUserId             int
	MTableAdvanceParam       string
	MTableName               string
	MRoomId                  int
	MTableId                 int
	MKindId                  int
	MClubId                  int
	MIsIpWarn                int
	MIsGpsWarn               int
}

/*
	required stTableAttribute mTableAttribute = 1;  //桌子属性
    required uint32 mBankerSeatId = 2; //庄家id
	required uint32 mFirstDice = 3; //第一个色子数值
    required uint32 mSecondDice = 4;//第二个色子数值
    required uint32 mCardNum = 5;//剩余卡牌数量
    required TableState mTableState = 6;//当前桌子状态
    repeated SeatInfo mSeatInfos = 7; //桌子座位信息
	required uint32 mGameCount = 8;	//已完成游戏局数
	required uint32 mRoundCount = 9; //当局轮数
	required int64 mWaitReadyTime = 10; //准备时间
	required int64 mWaitEndTime = 11;//结束时间
	required int64 mAllWaitOperateTime = 12;//操作总时间
*/

type XzmjTable struct {
	MRobot              *Robot
	MBankerSeatId       int
	MFirstDice          int
	MSecondDice         int
	MCardNum            int
	MTableState         int
	MGameCount          int
	MWaitReadyTime      int
	MWaitEndTime        int
	MAllWaitOperateTime int
	MSelfSeatId         int
	MTableAttribute     XzmjTableAttribute
	MTableUser          [4]XzmjTableUser
}

func (t *XzmjTable) Init(robot *Robot) error {
	if robot == nil {
		logger.Log4.Error("robot param error")
		return errors.New("ERR_PARAM")
	}
	t.MRobot = robot
	logger.Log4.Debug("<ENTER> :UserId-%d", t.MRobot.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", t.MRobot.mRobotData.MUId)

	for i, user := range t.MTableUser {
		user.MSeatId = i
	}

	t.MSelfSeatId = -1

	return nil
}

func (t *XzmjTable) HandelGameMainMsg(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", t.MRobot.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", t.MRobot.mRobotData.MUId)

	if msgHead == nil {
		return
	}
	switch msgHead.MSubCmd {
	case int16(XueZhanMj.EnGameFrameID_GAME_SUB_SC_USER_TABLE_NOTIFY):
		t.HandleTableInfoNotify(msgHead)
		break
	default:
		break
	}

	return
}

func (t *XzmjTable) HandleTableInfoNotify(msgHead *net.MsgHead) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", t.MRobot.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", t.MRobot.mRobotData.MUId)

	if msgHead == nil {
		return
	}

	tableInfoNotify := &XueZhanMj.SC_TableInfoNotify{}
	err := proto.Unmarshal(msgHead.MData, tableInfoNotify)
	if err != nil {
		logger.Log4.Debug("unmarshal SC_TableInfoNotify error: %s", err)
	}
	logger.Log4.Debug("tableInfoNotify: %+v", tableInfoNotify)

	tableInfo := tableInfoNotify.GetMTableInfo()

	//更新桌子信息
	//1.桌子属性
	tableAttr := tableInfo.GetMTableAttribute()
	//更新桌子属性
	t.UpdateTableAttribute(tableAttr)
	t.MBankerSeatId = int(tableInfo.GetMBankerSeatId())
	t.MFirstDice = int(tableInfo.GetMFirstDice())
	t.MSecondDice = int(tableInfo.GetMSecondDice())
	t.MCardNum = int(tableInfo.GetMCardNum())
	t.MGameCount = int(tableInfo.GetMGameCount())
	t.MAllWaitOperateTime = int(tableInfo.GetMAllWaitOperateTime())
	t.MTableState = int(tableInfo.GetMTableState())
	t.MWaitEndTime = int(tableInfo.GetMWaitEndTime())
	t.MWaitReadyTime = int(tableInfo.GetMWaitReadyTime())

	//更新桌子用户信息
	t.UpdateTableUserInfo(tableInfo.GetMSeatInfos())

	//处理相关逻辑
	if t.MTableState == Table_State_Game_End {
		t.MRobot.ReportXzmjGameEnd()
	}

}

func (t *XzmjTable) UpdateTableUserInfo(seatInfo []*XueZhanMj.SeatInfo) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", t.MRobot.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", t.MRobot.mRobotData.MUId)
	if seatInfo == nil {
		return
	}
	for _, user := range seatInfo {
		lSeatId := user.GetMSeatId()
		if lSeatId < 0 || lSeatId >= 4 {
			logger.Log4.Error("<LEAVE>:UserId-%d: seatd id %d is error", t.MRobot.mRobotData.MUId, lSeatId)
			continue
		}
		t.MTableUser[lSeatId].MoffLineFlag = bool(user.GetMOffLineFlag())
		t.MTableUser[lSeatId].MUserState = int(user.GetMUserState())

		//更新基本信息
		userBaseInfo := user.GetMBaseInfo()
		t.MTableUser[lSeatId].MBaseInfo.MUserID = int(userBaseInfo.GetMUserID())
		t.MTableUser[lSeatId].MBaseInfo.MAccid = int(userBaseInfo.GetMAccid())
		t.MTableUser[lSeatId].MBaseInfo.MNickName = userBaseInfo.GetMNickName()
		t.MTableUser[lSeatId].MBaseInfo.MFaceID = userBaseInfo.GetMFaceID()
		t.MTableUser[lSeatId].MBaseInfo.MSex = int(userBaseInfo.GetMSex())
		t.MTableUser[lSeatId].MBaseInfo.MUserGold = int(userBaseInfo.GetMUserGold())
		t.MTableUser[lSeatId].MBaseInfo.MUserDiamond = int(userBaseInfo.GetMUserDiamond())
		t.MTableUser[lSeatId].MBaseInfo.MDescription = userBaseInfo.GetMDescription()
		t.MTableUser[lSeatId].MBaseInfo.MVipLevel = int(userBaseInfo.GetMVipLevel())
		t.MTableUser[lSeatId].MBaseInfo.MUserPoint = int(userBaseInfo.GetMUserPoint())
		t.MTableUser[lSeatId].MBaseInfo.MUserIp = userBaseInfo.GetMUserIp()
		t.MTableUser[lSeatId].MBaseInfo.MUserLng = int(userBaseInfo.GetMUserLng())
		t.MTableUser[lSeatId].MBaseInfo.MUserLat = int(userBaseInfo.GetMUserLat())

		if t.MTableUser[lSeatId].MBaseInfo.MUserID == t.MRobot.mRobotData.MUserId {
			t.MSelfSeatId = int(lSeatId)
		}

	}
}

func (t *XzmjTable) UpdateTableAttribute(tableAttr *XueZhanMj.StTableAttribute) {
	logger.Log4.Debug("<ENTER> :UserId-%d:", t.MRobot.mRobotData.MUId)
	defer logger.Log4.Debug("<LEAVE>:UserId-%d:", t.MRobot.mRobotData.MUId)

	if tableAttr == nil {
		return
	}
	t.MTableAttribute.MGameCurrencyType = int(tableAttr.GetGameCurrencyType())
	t.MTableAttribute.MPayRoomRateType = int(tableAttr.GetPayRoomRateType())
	t.MTableAttribute.MPlanGameCount = int(tableAttr.GetPlanGameCount())
	t.MTableAttribute.MDiZhu = int(tableAttr.GetDiZhu())
	t.MTableAttribute.MEnterScore = int(tableAttr.GetEnterScore())
	t.MTableAttribute.MLeaveScore = int(tableAttr.GetLeaveScore())
	t.MTableAttribute.MBeiShuLimit = int(tableAttr.GetBeiShuLimit())
	t.MTableAttribute.MChairCount = int(tableAttr.GetChairCount())
	t.MTableAttribute.MIsAllowEnterAfterStart = int(tableAttr.GetIsAllowEnterAfterStart())
	t.MTableAttribute.MTableType = int(tableAttr.GetTableType())
	t.MTableAttribute.MRoomRate = int(tableAttr.GetRoomRate())
	t.MTableAttribute.MServerRate = int(tableAttr.GetServerRate())
	// 高级参数部分
	t.MTableAttribute.MZiMoJiaFan = tableAttr.GetZiMoJiaFan()
	t.MTableAttribute.MZiMoMoreThanMaxFan = tableAttr.GetZiMoMoreThanMaxFan()
	t.MTableAttribute.MJinGouDiao = tableAttr.GetJinGouDiao()
	t.MTableAttribute.MHaiDiLaoYue = tableAttr.GetHaiDiLaoYue()
	t.MTableAttribute.MDaXiaoYu = tableAttr.GetDaXiaoYu()
	t.MTableAttribute.MDianGangHuaZiMo = tableAttr.GetDianGangHuaZiMo()
	t.MTableAttribute.MYaoJiu = tableAttr.GetYaoJiu()
	t.MTableAttribute.MJiang = tableAttr.GetJiang()
	t.MTableAttribute.MHuanSanZhangType = int(tableAttr.GetHuanSanZhangType())
	t.MTableAttribute.MStartGameMinPlayerCount = int(tableAttr.GetStartGameMinPlayerCount())
	t.MTableAttribute.MOwnerUserId = int(tableAttr.GetMOwnerUserId())
	t.MTableAttribute.MTableAdvanceParam = tableAttr.GetTableAdvanceParam()
	t.MTableAttribute.MTableName = tableAttr.GetMTableName()
	t.MTableAttribute.MRoomId = int(tableAttr.GetMRoomId())
	t.MTableAttribute.MTableId = int(tableAttr.GetMTableId())
	t.MTableAttribute.MKindId = int(tableAttr.GetMKindId())
	t.MTableAttribute.MClubId = int(tableAttr.GetMClubId())
	t.MTableAttribute.MIsIpWarn = int(tableAttr.GetMIsIpWarn())
	t.MTableAttribute.MIsGpsWarn = int(tableAttr.GetMIsGpsWarn())

	logger.Log4.Debug("UserId-%d: MTableAttribute:%v", t.MRobot.mRobotData.MUId, t.MTableAttribute)

	return

}
