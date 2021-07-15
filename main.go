package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

var channels = make(map[int](chan string))
var ids = make(map[string](int))
var maxId = 0

func dispatch(c net.Conn, channel chan string) {
	for {
		_, err := c.Write([]byte(fmt.Sprintf("%s\n", <-channel)))
		if err != nil {
			break
		}
	}

}

func cleanChannel(id int) {
	delete(channels, id)
}

func handleConnection(c net.Conn) {
	fmt.Printf("Serving %s\n", c.RemoteAddr().String())
	c.SetReadDeadline(time.Now().Add(50 * time.Second))
	auth, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		fmt.Println(err)
		_ = c.Close()
		return
	}
	auth = strings.TrimSpace(auth)

	if strings.Contains(auth, " ") {
		c.Close()
		return
	}
	id, ok := ids[auth]
	if !ok {
		maxId += 1
		ids[auth] = maxId
		id = maxId
	}
	channels[id] = make(chan string)
	defer cleanChannel(id)
	c.SetReadDeadline(time.Time{})

	go dispatch(c, channels[id])
	channels[id] <- strconv.Itoa(id)
	for {
		netData, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			fmt.Println(err)
			return
		}
		line := strings.Fields(strings.TrimSpace(netData))
		if len(line) < 2 {
			break
		}
		send := fmt.Sprintf("%d ", id)

		for _, i := range line[1:] {
			conv, err := strconv.Atoi(i)
			if err != nil {
				break
			}
			send += fmt.Sprintf("%d ", conv)
		}

		if target, err := strconv.Atoi(line[0]); err == nil {
			channel, ok := channels[target]
			if ok {
				channel <- send
				c.Write([]byte("-1\n"))
			} else {
				c.Write([]byte("-2\n"))
			}
		} else {
			break
		}

	}
	c.Close()
}

func closeDb(db *os.File) {
	db.Seek(0, 0)
	db.Truncate(0)
	for key, value := range ids {
		db.Write([]byte(fmt.Sprintf("%s %d\n", key, value)))
	}
	db.Close()
}

func main() {
	db, err := os.OpenFile("db.txt", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer closeDb(db)
	scanner := bufio.NewScanner(db)
	for scanner.Scan() {
		line := strings.Fields(scanner.Text())
		if len(line) > 0 {
			ids[line[0]], err = strconv.Atoi(line[1])
			if ids[line[0]] > maxId {
				maxId = ids[line[0]]
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide a port number!")
		return
	}

	PORT := ":" + arguments[1]
	l, err := net.Listen("tcp4", PORT)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			l.Close()
			closeDb(db)
			os.Exit(0)
		}
	}()

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		go handleConnection(c)
	}
}
