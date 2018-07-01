package main

type PlayerBaseInfo struct {
	MUserID      int
	MAccid       int
	MNickName    string
	MFaceID      string
	MSex         int
	MUserGold    int
	MUserDiamond int
	MDescription string
	MVipLevel    int
	MUserPoint   int
	MUserIp      string
	MUserLng     int
	MUserLat     int
}

/*
message SeatCardInfo{
    required uint32 mSeatId = 1; //座位id
    required uint32 mTagSeatId = 2; //自己的座位id
    required UserState mUserState = 3; //当前座位玩家状态
    required uint32 mRejectSuit = 4; //缺色
    repeated uint32 mDealCards =5;//出的牌
    repeated uint32 mHandCards = 6;//手牌
    repeated uint32 mMingCards = 7;//明杠的牌
    repeated uint32 mAnGangCards = 8;//按杠的牌
    repeated uint32 mBuGangCards = 9;//补杠的牌
    repeated uint32 mPengCards = 10;//碰的牌
    required uint32 mHuCards = 11;//胡的牌
    optional MMjOperateRecord mLastOperateRecord = 12; //最后操作记录
    repeated uint32 mHuanCards = 13;//换出的三张牌
    repeated uint32 mHuanInCards = 14;//换进的三张牌
	required MJ_SETTLEMENT_TYPE mSettlementType = 15;//结算类型
}

*/

type XzmjTableUser struct {
	MSeatId      int
	MUserState   int
	MoffLineFlag bool
	MBaseInfo    PlayerBaseInfo
	mRejectSuit  int
	mDealCards   []int
	mHandCards   []int
	mMingCards   []int
	mAnGangCards []int
	mBuGangCards []int
	mPengCards   []int
	mHuCards     int
	mHuanCards   []int
	mHuanInCards []int
}
