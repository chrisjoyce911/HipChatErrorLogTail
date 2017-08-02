package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/andybons/hipchat"
	"github.com/hpcloud/tail"
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
	roomPtr := flag.String("r", "Error Logs", "HipChat room")
	secondsPtr := flag.Float64("s", 30, "Seconds (int)")

	flag.Parse()

	var accesstoken = *wordPtr
	var filetotail = *filePtr
	var hipchatroom = *roomPtr
	var reporttime = *secondsPtr

	if len(accesstoken) < 1 {
		os.Exit(3)
	}

	if len(filetotail) < 1 {
		os.Exit(3)
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

			thisline.Count = 1
			thisline.Level = result[4]
			thisline.File = result[5]
			thisline.Request = result[6]
			thisline.IP = result[7]
			thisline.Message = M

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
			if len(summary) > 0 {
				s := []string{}
				s = append(s, fmt.Sprintf("Error log update %v: ", t1))
				for _, item := range summary {
					s = append(s, fmt.Sprintf("%d %s %s %s %s %s ", item.Count, item.Level, item.File, item.Request, item.IP, item.Message))
				}
				summary = summary[:0]
				// fmt.Println(strings.Join(s, "\n"))
				var m = strings.Join(s, "\n")
				go func() {
					c := hipchat.Client{AuthToken: accesstoken}

					req := hipchat.MessageRequest{
						RoomId:        hipchatroom,
						From:          "Error Log Tail",
						Message:       m,
						Color:         hipchat.ColorRed,
						MessageFormat: hipchat.FormatText,
						Notify:        true,
					}
					if err := c.PostMessage(req); err != nil {
						log.Printf("Expected no error, but got %q", err)
					}

					return
				}()
			}
			t0 = time.Now()
		}
	}
}
