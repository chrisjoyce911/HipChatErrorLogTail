package main

import (
	"flag"
	"fmt"
	"html"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hpcloud/tail"
	"github.com/tbruyelle/hipchat-go/hipchat"
)

var t0 = time.Now()

// LogLine .. LogLine
type LogLine struct {
	ID      int
	Count   int
	Level   string
	File    string
	Request string
	IP      string
	Message string
}

func main() {

	wordPtr := flag.String("t", "", "HipChat Channel token")
	filePtr := flag.String("f", "", "Logfile to tail")
	roomPtr := flag.String("r", "Integration Testing", "HipChat room")
	secondsPtr := flag.Float64("s", 30, "Seconds (int)")

	flag.Parse()

	var accesstoken = *wordPtr
	var filetotail = *filePtr
	var hipchatroom = *roomPtr
	var reporttime = *secondsPtr

	if len(accesstoken) < 1 {
		log.Fatalln("No HipChat Channel Token")
	}

	if len(filetotail) < 1 {
		log.Fatalln("The log file you want read is not specified")
	}

	myname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	messages := make(chan string)
	var summary []LogLine

	go func() {
		mycfg := tail.Config{}
		mycfg.Follow = true
		mycfg.Location = &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END}
		t, err := tail.TailFile(filetotail, mycfg)
		for line := range t.Lines {
			messages <- line.Text
		}
		if err != nil {
			return
		}
	}()

	for {
		select {
		case msg := <-messages:
			result := strings.Fields(msg)
			M := ""

			for i := 8; i < len(result); i++ {
				M = M + result[i] + " "
			}
			var thisline LogLine

			if len(summary) > 0 {
				thisline.ID = 1
			} else {
				thisline.ID = len(summary) + 1
			}

			if len(result) > 8 {
				thisline.Count = 1
				thisline.Level = result[4]
				thisline.File = result[5]
				thisline.Request = result[6]
				thisline.IP = result[7]
				thisline.Message = M
			} else {
				thisline.Message += msg
			}
			// log.Println(thisline.Message)

			needtoadd := true
			for i, item := range summary {
				if item.File == thisline.File {
					summary[i].Count = summary[i].Count + 1
					needtoadd = false
				}
			}
			if needtoadd {
				summary = append(summary, thisline)
			}
		default:

		}
		t1 := time.Now()
		d := t1.Sub(t0)
		s := d.Seconds()
		if s > reporttime {
			log.Println("Report")
			if len(summary) > 0 {
				log.Println("Sumary")
				s := []string{}
				for _, item := range summary {
					log.Println(item.Message)
					s = append(s, fmt.Sprintf("<strong>%d</strong> <i>%s</i> %s %s %s %s ", item.Count, item.Level, item.File, item.Request, item.IP, html.EscapeString(item.Message)))
				}
				summary = summary[:0]
				// fmt.Println(strings.Join(s, "\n"))
				var m = strings.Join(s, "<br>")
				go func() {
					c := hipchat.NewClient(accesstoken)

					//see:
					//https://www.hipchat.com/docs/apiv2/method/send_room_notification
					req := &hipchat.NotificationRequest{
						From:          fmt.Sprintf("Host(%s)", myname),
						Message:       m,
						Color:         "red",
						MessageFormat: "html",
						Notify:        true,
					}
					if _, err := c.Room.Notification(hipchatroom, req); err != nil {
						log.Printf("Expected no error, but got %q", err)
					}

					return
				}()
			} else {
				log.Println("No summary")
			}
			t0 = time.Now()
		}
	}
}
