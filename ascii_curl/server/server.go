package server

import (
	"ascii_curl/frames"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

var NotFoundMessage = map[string]string{
	"error": "Frames not found. Navigate to /list for list of frames. Navigate to https://github.com/hugomd/ascii-live to submit new frames.",
}

var NotCurledMessage = map[string]string{
	"error": "You almost ruined a good surprise. Come on, curl it in terminal.",
}

var UpdateMessage = map[string]string{
	"message": "Frames updated.",
}

var availableFrames []string

func init() {
	for k := range frames.FrameMap {
		availableFrames = append(availableFrames, k)
	}
}

func writeJson(w http.ResponseWriter, r *http.Request, res interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(res)
	if err != nil {
		return
	}
	w.WriteHeader(status)
	fmt.Fprint(w, string(data))
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	writeJson(w, r, map[string][]string{"frames": availableFrames}, http.StatusOK)
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	frames.UpdateFrames()
	writeJson(w, r, UpdateMessage, http.StatusOK)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeJson(w, r, NotFoundMessage, http.StatusNotFound)
}

func notCurledHandler(w http.ResponseWriter, r *http.Request) {
	writeJson(w, r, NotCurledMessage, http.StatusExpectationFailed)
}

func handler(w http.ResponseWriter, r *http.Request) {
	cn := w.(http.CloseNotifier)
	flusher := w.(http.Flusher)

	vars := mux.Vars(r)
	frameSource := vars["frameSource"]
	glog.Infof("Frame source %s", frameSource)

	frames, ok := frames.FrameMap[frameSource]
	if !ok {
		notFoundHandler(w, r)
		return
	}

	userAgent := r.Header.Get("User-Agent")
	if !strings.Contains(userAgent, "curl") {
		notCurledHandler(w, r)
		return
	}

	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	i := 0
	for {
		select {
		// Handle client disconnects
		case <-cn.CloseNotify():
			glog.Infof("Client stopped listening")
			return
		default:
			if i >= frames.GetLength() {
				i = 0
			}
			// Artificially wait between reponses.
			time.Sleep(frames.GetDelay())

			// Clear screen
			clearScreen := "\033[2J\033[H"
			newLine := "\n"

			// Write frames
			fmt.Fprintf(w, clearScreen+frames.GetFrame(i)+newLine)
			i++

			// Send some data.
			flusher.Flush()
		}
	}
}

// Server.
func InitServer() {
	flag.Parse()
	// Don't write to /tmp - doesn't work in docker scratch
	flag.Set("logtostderr", "true")

	r := mux.NewRouter()
	r.HandleFunc("/list", listHandler).Methods("GET")
	r.HandleFunc("/update", updateHandler).Methods("GET")
	r.HandleFunc("/{frameSource}", handler).Methods("GET")
	r.NotFoundHandler = http.HandlerFunc(notFoundHandler)

	port := os.Getenv("PORT")
	if port == "" {
		glog.Error("Environment variable PORT not set")
		return
	}
	srv := &http.Server{
		Handler: r,
		Addr:    ":" + port,
		// Set unlimited read/write timeouts
		ReadTimeout:  0,
		WriteTimeout: 0,
	}

	glog.Infof("Serving on 0.0.0.0:%s ...", port)
	glog.Fatal(srv.ListenAndServe())
}
