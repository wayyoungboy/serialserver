package main

import (
	"fmt"
	"github.com/tarm/serial"
	"log"
	"net"
	"os"
	"runtime"

)

func main() {
	fmt.Println("starting the server")
	//key := pbkdf2.Key([]byte("demo pass"), []byte("demo salt"), 1024, 32, sha1.New)
	//block, _ := kcp.NewAESBlockCrypt(key)
	var remoteAddress, _ = net.ResolveTCPAddr("tcp4", "47.101.214.72:9010") //生成一个net.TcpAddr对像。
	var conn, err = net.DialTCP("tcp4", nil, remoteAddress)               //传入协议，本机地址（传了nil），远程地址，获取连接。
	if err != nil {                                                       //如果连接失败。则返回。
		fmt.Println("连接出错：", err)
		return
	}
	c := &serial.Config{Name: "COM1", Baud: 115200}
	serial, _ := serial.OpenPort(c)
	go func() {
		buf := make([]byte, 512)
		for {
			len, err := conn.Read(buf)
			if err != nil {
				fmt.Println("Error reading", err.Error())
				return //终止程序
			}

			bytetoserial_TCP(serial, buf[:len])
		}
	}()
	//接收串口来的数据，通过TCP发送回PC
	go func() {

		buf := make([]byte, 512)
		for {
			data := serialtobyte_TCP(serial, buf)
			_, err = conn.Write(data)
			if err != nil {
				log.Println(err)
				return
			}
			fmt.Printf("[SEVER]  Serial->TCP: %v\n", string(data))
		}
	}()
	go runtime.GC()
}








func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

func bytetoserial_TCP(serialport *serial.Port, usrdataget []byte) {
	//log.Println("bytetoserial")
	d := usrdataget
	_, err := serialport.Write(d)
	if err != nil {
		log.Fatal(err)
	}
	d = nil
	return
}

//从串口读取数据并存至通道缓存
func serialtobyte_TCP(serialport *serial.Port, dserialtobyte []byte) (data []byte) {
	//func serialtobyte(serialport *serial.Port, dserialtobyte chan []byte) {

	//log.Println("serialtobyte")
	serialbuf := make([]byte, 256)
	n, err := serialport.Read(serialbuf)
	if err != nil {
		serialtobyte_TCP(serialport, dserialtobyte)
	}
	//	fmt.Println("serial to byte:", string(serialbuf[:n]))
	//dserialtobyte <- serialbuf[:n]
	data = serialbuf[:n]
	return data

}
