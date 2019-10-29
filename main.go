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
	"sync"
	"sync/atomic"
	"time"
)

var activeConnNum int32

var isServer bool
var port int
var serverAddr string
var laddrs string
var connNum int
var interval int
var duration int

func init() {
	flag.BoolVar(&isServer, "s", false, "server mode")
	flag.IntVar(&port, "p", 9000, "port")
	flag.StringVar(&serverAddr, "c", "127.0.0.1", "server addresses")
	flag.StringVar(&laddrs, "l", "0.0.0.0", "local addresses")
	flag.IntVar(&connNum, "n", 10, "number of tcp connection")
	flag.IntVar(&interval, "i", 1, "heartbead interval")
	flag.IntVar(&duration, "d", 10, "test duration")
	flag.Parse()
}

func main() {
	if isServer {
		err := runServer(port)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(0)
		}
	} else {
		runClient(serverAddr, port, interval, connNum, duration)
	}
}

func runClient(serverAddr string, serverPort int, interval int, num int, duration int) {
	var wg sync.WaitGroup
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(duration)*time.Second)
	for i := 0; i < num; i++ {
		wg.Add(1)
		go clientDial(i, &wg, ctx, serverAddr, serverPort, interval)
	}

	wg.Wait()
}

func clientDial(index int, wg *sync.WaitGroup, ctx context.Context, serverAddr string, serverPort int, interval int) {
	conn, err := net.Dial("tcp", serverAddr+":"+strconv.Itoa(serverPort))
	if err != nil {
		fmt.Println("Error dialing: ", err.Error())
		return
	}

	_, err = conn.Write([]byte("ping\n"))
	if err != nil {
		fmt.Println("Error sending first ping: ", err.Error())
		return
	}

	bufferReader := bufio.NewReader(conn)
	tick := time.Tick(time.Duration(interval) * time.Second)
	for {
		select {
		case <-ctx.Done():
			_ = conn.Close()
			wg.Done()
			fmt.Println("Num " + strconv.Itoa(index) + " done.")
			return
		case <-tick:
			var buf string
			buf, err = bufferReader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					fmt.Printf("Server %s is close!\n", conn.RemoteAddr().String())
				}
				return
			}

			if strings.TrimSpace(string(buf)) == "pong" {
				fmt.Println(strconv.Itoa(index) + ": ping pong")
				atomic.AddInt32(&activeConnNum, 1)

				_, err = conn.Write([]byte("ping\n"))
				if err != nil {
					fmt.Println("Error sending ping: ", err.Error())
					return
				}
			}
		}
	}
}

func runServer(serverPort int) (err error) {
	l, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(serverPort))

	if err != nil {
		fmt.Println("Error listening: ", err.Error())
		_ = l.Close()
		return
	}

	for {
		conn, err := l.Accept()
		if err == nil {
			go serverProcess(conn)
		} else {
			fmt.Println(err.Error())
		}
	}
}

func serverProcess(conn net.Conn) {
	bufferReader := bufio.NewReader(conn)
	for {
		buf, err := bufferReader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Printf("client %s is close!\n", conn.RemoteAddr().String())
			}
			return
		}

		if strings.TrimSpace(string(buf)) == "ping" {
			_, _ = conn.Write([]byte("pong\n"))
		}
	}
}
