package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var BuildTime string
var AppVersion = "0.0.2 build on " + BuildTime

var activeConnNum int32

var isShowVersion bool
var isServer bool
var port int
var serverAddr string
var laddrStart string
var laddrEnd string
var connNum int
var interval int
var dataLen int
var duration int

func init() {
	flag.BoolVar(&isShowVersion, "v", false, "show version")
	flag.BoolVar(&isServer, "s", false, "server mode")
	flag.IntVar(&port, "p", 9000, "port")
	flag.StringVar(&serverAddr, "c", "127.0.0.1", "server addresses")
	flag.StringVar(&laddrStart, "ls", "0.0.0.0", "start ip of local addresses")
	flag.StringVar(&laddrEnd, "le", "0.0.0.0", "end ip of local addresses")
	flag.IntVar(&connNum, "n", 10, "number of tcp connection")
	flag.IntVar(&interval, "i", 1, "heartbead interval")
	flag.IntVar(&dataLen, "l", 100, "test data length")
	flag.IntVar(&duration, "d", 10, "test duration")
	flag.Parse()

	if isShowVersion {
		fmt.Println(AppVersion)
		os.Exit(0)
	}
}

func main() {
	if isServer {
		err := runServer(port, dataLen)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(0)
		}
	} else {
		runClient(laddrStart, laddrEnd, serverAddr, port, interval, dataLen, connNum, duration)
	}
}

func runClient(laddrStart string, laddrEnd string, serverAddr string, serverPort int, interval int, dataLen int, num int, duration int) {
	var clients []*client
	laddr := laddrStart
	newStart := true
	for i := 0; i < num; i++ {
		if laddr == laddrEnd {
			laddr = laddrStart
			newStart = true
		}

		if newStart {
			newStart = false
		} else {
			laddr = getNextIP(laddr, 1)
		}

		sendData := make([]byte, dataLen)
		copy(sendData, "ping")
		sendData = append(sendData, []byte("\n")...)
		c := client{index: i, localAddr: laddr, serverAddr: serverAddr, serverPort: serverPort, sendData: sendData}
		clients = append(clients, &c)
		go c.Start()
	}

	tick := time.Tick(time.Duration(interval) * time.Second)
	ctx, cancle := context.WithTimeout(context.Background(), time.Duration(duration)*time.Second)
	defer cancle()

	loop := 0
	for {
		select {
		case <-ctx.Done():
			for _, c := range clients {
				c.quit <- 0
			}
			time.Sleep(1 * time.Second)
			fmt.Println(time.Now().Format("2006-01-02 15:04:05") + " test finished")
			return
		case <-tick:
			fmt.Println(time.Now().Format("2006-01-02 15:04:05") + " active connection " + strconv.Itoa(int(activeConnNum)))
			atomic.StoreInt32(&activeConnNum, 0)
			for _, c := range clients {
				c.tick <- loop
			}
			loop++
		}
	}
}

type client struct {
	index      int
	localAddr  string
	serverAddr string
	serverPort int
	sendData   []byte
	conn       *net.Conn
	tick       chan int
	quit       chan int
}

func (c *client) Start() {
	c.tick = make(chan int, 10)
	c.quit = make(chan int, 1)

	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{
			IP:   net.ParseIP(c.localAddr),
			Port: 0,
		},
	}
	conn, err := dialer.Dial("tcp", c.serverAddr+":"+strconv.Itoa(c.serverPort))
	if err != nil {
		fmt.Println("Error dialing: ", err.Error())
		return
	}
	c.conn = &conn
	fmt.Printf("%d %s --> %s\n", c.index, conn.LocalAddr().String(), conn.RemoteAddr().String())
	_, err = conn.Write(c.sendData)
	if err != nil {
		fmt.Println("Error sending first ping: ", err.Error())
		return
	}

	bufferReader := bufio.NewReader(conn)

	for {
		select {
		case loop := <-c.tick:
			var buf string
			buf, err = bufferReader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					fmt.Printf("Server %s is close!\n", conn.RemoteAddr().String())
				}
				return
			}

			atomic.AddInt32(&activeConnNum, 1)

			if strings.Contains(strings.TrimSpace(buf), "pong") {
				_, err = conn.Write(c.sendData)
				if err != nil {
					fmt.Println(strconv.Itoa(loop)+" Error sending ping: ", err.Error())
					return
				}
			}
		case <-c.quit:
			err = conn.Close()
			if err != nil {
				fmt.Println("Error to close connection: " + err.Error())
			}
			return
		}
	}
}

func runServer(serverPort int, dataLen int) (err error) {
	l, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(serverPort))

	if err != nil {
		fmt.Println("Error listening: ", err.Error())
		_ = l.Close()
		return
	}

	sendData := make([]byte, dataLen)
	copy(sendData, "pong")
	sendData = append(sendData, []byte("\n")...)

	for {
		conn, err := l.Accept()
		if err == nil {
			go serverProcess(conn, sendData)
		} else {
			fmt.Println(err.Error())
		}
	}
}

func serverProcess(conn net.Conn, sendData []byte) {
	bufferReader := bufio.NewReader(conn)
	for {
		buf, err := bufferReader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Printf("client %s is close!\n", conn.RemoteAddr().String())
			}
			return
		}

		if strings.Contains(strings.TrimSpace(buf), "ping") {
			_, _ = conn.Write(sendData)
		}
	}
}

func getNextIP(ip string, inc uint) string {
	i := net.ParseIP(ip).To4()
	v := uint(i[0])<<24 + uint(i[1])<<16 + uint(i[2])<<8 + uint(i[3])
	v += inc
	v3 := byte(v & 0xFF)
	v2 := byte((v >> 8) & 0xFF)
	v1 := byte((v >> 16) & 0xFF)
	v0 := byte((v >> 24) & 0xFF)
	return net.IPv4(v0, v1, v2, v3).String()
}
