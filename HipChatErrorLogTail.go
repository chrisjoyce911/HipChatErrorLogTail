package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/andybons/hipchat"
	"github.com/gorilla/mux"
	"github.com/hpcloud/tail"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	logger "github.com/sirupsen/logrus"
)

var t0 = time.Now()
var totallines = 0

// RemoteToken ... used to set debug level and get status
var RemoteToken = "4Me2Test"

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

	formatter := &logger.TextFormatter{
		TimestampFormat: "02-01-2006 15:04:05.000",
		FullTimestamp:   true,
	}
	logger.SetFormatter(formatter)

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
		logger.Fatalln("No HipChat Channel Token")
	}

	if len(filetotail) < 1 {
		logger.Fatalln("The log file you want read is not specified")
	}

	myname, err := os.Hostname()
	if err != nil {
		logger.Fatalln(err)
	}

	// prometheus monitoring
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		logger.Fatal(http.ListenAndServe(":8000", nil))
	}()

	// Add a remote API
	go func() {
		router := mux.NewRouter()
		router.HandleFunc("/health/{token}", GetHealthEndpoint).Methods("GET")
		router.HandleFunc("/loglevel/{level}/{token}", SetLogLevelEndpoint).Methods("PUT")
		logger.Fatal(http.ListenAndServe(":12345", router))
	}()

	messages := make(chan string)
	var summary []LogLine

	go func() {
		mycfg := tail.Config{}
		mycfg.Follow = true
		mycfg.Location = &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END}
		t, err := tail.TailFile(filetotail, mycfg)
		for line := range t.Lines {
			logger.WithFields(logger.Fields{
				"stage": "log line read",
			}).Debug(line.Text)
			messages <- line.Text
		}
		if err != nil {
			logger.Fatal("MAYDAY MAYDAY MAYDAY. Error when reading the logfile")
			return
		}
	}()

	for {
		select {
		case msg := <-messages:
			logger.WithFields(logger.Fields{
				"stage": "processing log line",
			}).Debug(msg)
			result := strings.Fields(msg)
			M := ""

			// This is a hack to deal with Multi lines/ Short lines ..
			if len(result) > 8 {
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
			} else {
				logger.Warning("Short log fline found")
			}
		default:

		}
		t1 := time.Now()
		d := t1.Sub(t0)
		s := d.Seconds()
		if s > reporttime {
			if len(summary) > 0 {
				s := []string{}
				s = append(s, fmt.Sprintf("%s : Error log update %v: ", myname, t1))
				for _, item := range summary {
					s = append(s, fmt.Sprintf("%d %s %s %s %s %s ", item.Count, item.Level, item.File, item.Request, item.IP, item.Message))
				}
				summary = summary[:0]
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
						logger.Printf("Expected no error, but got %q", err)
					}

					return
				}()
			}
			t0 = time.Now()
		}
	}
}

// GetHealthEndpoint ... Remote access health check
func GetHealthEndpoint(w http.ResponseWriter, req *http.Request) {

	params := mux.Vars(req)

	w.Header().Set("Content-Type", "application/json")
	token := params["token"]
	var reply string
	if TestToken(token) {
		reply = fmt.Sprintf("{\"status\": \"good\", \"token\": \"%s\", \"totallines\": \"%d\"}", token, totallines)
	} else {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		reply = ""
	}
	w.Write([]byte(reply))
	return
}

// SetLogLevelEndpoint ... Remote set of logging level
func SetLogLevelEndpoint(w http.ResponseWriter, req *http.Request) {

	params := mux.Vars(req)

	w.Header().Set("Content-Type", "application/json")
	token := params["token"]
	level := params["level"]

	if TestToken(token) {
		http.Error(w, "StatusAccepted", http.StatusAccepted)
		switch level {
		case "Error":
			logger.SetLevel(logger.ErrorLevel)
		case "Warn":
			logger.SetLevel(logger.WarnLevel)
		case "Debug":
			logger.SetLevel(logger.DebugLevel)
		case "Info":
			logger.SetLevel(logger.InfoLevel)
		default:
			logger.WithFields(logger.Fields{
				"level": level,
			}).Info("Incorrect logging level")
			http.Error(w, "BadRequest", http.StatusBadRequest)
		}

	} else {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		logger.WithFields(logger.Fields{
			"level": level,
		}).Warning("Incorrect level")

	}
	w.Write([]byte(""))
	return
}

// TestToken ..  test access token
func TestToken(token string) bool {
	if RemoteToken == token {
		return true
	}
	logger.WithFields(logger.Fields{
		"token": token,
	}).Warning("Incorrect remote access token")

	return false
}
