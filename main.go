package main

import (
	"encoding/binary"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"GStress/msg"

	"GStress/logger"
	"GStress/net"
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"github.com/golang/protobuf/proto"
)

var addr = flag.String("addr", "192.168.157.129:22001", "http service address")

func Int32ToBytes(i int32) []byte {
	var buf = make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(i))
	return buf
}

func main() {
	logger.AddFileFilter("gamesvr", "gamesvr.log")
	logger.Log4.Info("<ENTER> gamesvr start")

	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	lnetClient, err := net.NewNetClient("192.168.157.129:22001", "")
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer lnetClient.Close()

	done := make(chan struct{})

	go func() {
		defer lnetClient.Close()
		defer close(done)
		for {
			cmd, para, data, err := lnetClient.ReadMsg()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %v, %v", cmd, para)

			newTest := &msg.LoginReturnInfo{}
			err = proto.Unmarshal(data, newTest)
			if err != nil {
				log.Fatal("unmarshaling error: ", err)
			}
			log.Printf("lProtoCmd: %+v", newTest)

		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lFlag bool = false

	for {
		select {
		case t := <-ticker.C:

			if lFlag {
				break
			}
			lFlag = true
			md5Ctx := md5.New()
			md5Ctx.Write([]byte("123456"))
			cipherStr := md5Ctx.Sum(nil)
			fmt.Print(cipherStr)
			secret := hex.EncodeToString(cipherStr)
			fmt.Print(secret)

			test := &msg.LoginRequestInfo{
				Account:      proto.String("lll001"),
				Passwd:       proto.String(secret),
				DeviceString: proto.String("12345612133333333422222222222343434234322"),
				Packageflag:  proto.String("QPB_WEB_1"),
			}

			err = lnetClient.SenMsg(int16(msg.EnMainCmdID_LOGIN_MAIN_CMD), int16(msg.EnSubCmdID_REQUEST_LOGIN_SUB_CMD), test)
			if err != nil {
				log.Println("write:", err)
				return
			}
			log.Println("t:", t)
		case <-interrupt:
			log.Println("interrupt")

			select {
			case <-done:
			case <-time.After(time.Second):
			}
			lnetClient.Close()
			return
		}
	}
}
