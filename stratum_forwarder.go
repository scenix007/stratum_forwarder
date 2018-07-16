package main

import (
	"bufio"
	"log"
	"flag"
	"fmt"
	"io"
	"net"
	"time"
	"os"
	"strings"
	"sync"
	"encoding/json"
)

var lock sync.Mutex
var trueList []string
var ip string
var list string
var timeout int

const (
	MaxReqSize = 10 * 1024
)

type JSONRpcReq struct {
	Id     *int      `json:"id"`
	Method string   `json:"method"`
	Params []string `json:"params"`
}


func main() {
	flag.StringVar(&ip, "l", ":8090", "-l=0.0.0.0:9897 指定服务监听的端口")
	flag.StringVar(&list, "d", "127.0.0.1:8080", "-d=127.0.0.1:1789,127.0.0.1:1788 指定后端的IP和端口,多个用','隔开")
	flag.IntVar(&timeout, "t", 60, "-t=60 链接超时时间， 默认六十秒")
	flag.Parse()
	trueList = strings.Split(list, ",")
	if len(trueList) <= 0 {
		fmt.Println("后端IP和端口不能空,或者无效")
		os.Exit(1)
	}
	log.Printf("Server Starting...")
	server()
}

func server() {
	lis, err := net.Listen("tcp", ip)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer lis.Close()
	for {
		conn, err := lis.Accept()
		if err != nil {
			fmt.Println("建立连接错误:%v\n", err)
			continue
		}
		fmt.Println(conn.RemoteAddr(), conn.LocalAddr())
		go handle(conn)
	}
}

func handle(sconn net.Conn) {
	defer sconn.Close()
	ip, ok := getIP()
	if !ok {
		return
	}
	dconn, err := net.Dial("tcp", ip)
	if err != nil {
		fmt.Printf("连接%v失败:%v\n", ip, err)
		return
	}
	ExitChan := make(chan bool, 1)
	go func(sconn net.Conn, dconn net.Conn, Exit chan bool) {
	connbuff := bufio.NewReaderSize(sconn, MaxReqSize)
	for{
			sconn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
			//log.Printf("waiting for sending")
			data, isPrefix, err := connbuff.ReadLine()
			if isPrefix {
				log.Printf("Socket flood detected")
				break
				// TODO: Ban client
			} else if err == io.EOF {
				log.Printf("Client disconnected")
				break
			} else if err != nil {
				log.Printf("Error reading while sending: %v", err)
				break
			}
			go parseReq(data) 
			_, err = dconn.Write(data)
			if err != nil {
				fmt.Printf("往%v发送数据失败:%v\n", ip, err)
				break
			}
			_, err = io.WriteString(dconn, "\n")
			if err != nil {
				fmt.Printf("往%v发送数据失败:%v\n", ip, err)
				break
			}
		}
		//	<-ExitChan
		log.Printf("send func ends!")
		ExitChan <- true
		//_, err := io.Copy(dconn, sconn)
		//fmt.Printf("往%v发送数据失败:%v\n", ip, err)
	}(sconn, dconn, ExitChan)
	go func(sconn net.Conn, dconn net.Conn, Exit chan bool) {
		connbuff := bufio.NewReaderSize(dconn, MaxReqSize)
		for {
			sconn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
			//log.Printf("waiting for receiving")
			data, isPrefix, err := connbuff.ReadLine()
			if isPrefix {
				log.Printf("Socket flood detected")
				break
				// TODO: Ban client
			} else if err == io.EOF {
				log.Printf("Client disconnected")
				break
			} else if err != nil {
				log.Printf("Error reading while receiving: %v", err)
				break
			}
			//go parseReq(data)
			_, err = sconn.Write(data)
			if err != nil {
				fmt.Printf("从%v接收数据失败:%v\n", ip, err)
				break
			}
			_, err = io.WriteString(sconn, "\n")
			if err != nil {
				fmt.Printf("从%v接收数据失败:%v\n", ip, err)
				break
			}
		}
		log.Printf("receive func ends!")
		ExitChan <- true
	}(sconn, dconn, ExitChan)
	<-ExitChan
	dconn.Close()
	log.Printf("dconnect close")
}

func getIP() (string, bool) {
	lock.Lock()
	defer lock.Unlock()

	if len(trueList) < 1 {
		return "", false
	}
	ip := trueList[0]
	trueList = append(trueList[1:], ip)
	return ip, true
}

func parseReq(data []byte) {
	if len(data) > 1 {
		var req JSONRpcReq
		err := json.Unmarshal(data, &req)
		if err != nil {
			log.Printf("Malformed request: %v", err)
			return
		}
		log.Printf("req raw %s", data)
		if req.Id == nil {
			err := fmt.Errorf("Server disconnect request")
			log.Println(err)
			return
		} else if req.Params == nil {
			err := fmt.Errorf("Server RPC request params")
			log.Println(err)
			return
		}
		if req.Method == "mining.submit" {
			if len(req.Params) != 5 {
				err := fmt.Errorf("Number of submit params is not 5")
				log.Println(err)
				return
			}
			workerName  := req.Params[0]
			jobId       := req.Params[1]
			extraNonce2 := req.Params[2]
			nTime       := req.Params[3]
			nOnce       := req.Params[4]
			log.Printf("Submit Detail: workerName %s; jobId %s; extraNonce2 %s; nTime %s; nOnce %s",
			workerName,jobId,extraNonce2,nTime,nOnce)
		}
	}
}
