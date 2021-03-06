package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"

	"github.com/peterpla/lead-expert/pkg/appengine"
	"github.com/peterpla/lead-expert/pkg/config"
	"github.com/peterpla/lead-expert/pkg/database"
	"github.com/peterpla/lead-expert/pkg/middleware"
	"github.com/peterpla/lead-expert/pkg/queue"
	"github.com/peterpla/lead-expert/pkg/request"
	"github.com/peterpla/lead-expert/pkg/serviceInfo"
)

var prefix = "TaskTaggingQAComplete"
var logPrefix = "tagging-qa-complete.main.init(),"
var cfg config.Config
var repo request.RequestRepository
var q queue.Queue
var qi = queue.QueueInfo{}
var qs queue.QueueService

// use a single instance of Validate, it caches struct info
var validate *validator.Validate

func init() {
	if err := config.GetConfig(&cfg, prefix); err != nil {
		msg := fmt.Sprintf(logPrefix+" GetConfig error: %v", err)
		panic(msg)
	}

	// make ServiceName and QueueName available to other packages
	serviceInfo.RegisterServiceName(cfg.ServiceName)
	serviceInfo.RegisterQueueName(cfg.QueueName)
	serviceInfo.RegisterNextServiceName(cfg.NextServiceName)
}

func main() {
	sn := serviceInfo.GetServiceName()
	// Creating App Engine task handlers: https://cloud.google.com/tasks/docs/creating-appengine-handlers

	defer catch() // implements recover so panics reported

	// connect to the Request database
	repo = database.NewFirestoreRequestRepository(cfg.ProjectID, cfg.DatabaseRequests)

	if cfg.IsGAE {
		q = queue.NewGCTQueue(&qi) // use Google Cloud Tasks for queueing
	} else {
		q = queue.NewNullQueue(&qi) // use null queue, requests thrown away on exit
	}

	qs = queue.NewService(q)
	_ = qs

	router := httprouter.New()
	router.POST("/task_handler", taskHandler(q))
	router.GET("/", indexHandler)
	router.NotFound = http.HandlerFunc(myNotFound)
	cfg.Router = router

	port := os.Getenv("PORT") // Google App Engine complains if "PORT" env var isn't checked
	if !cfg.IsGAE {
		port = viper.GetString(prefix + "Port")
	}
	if port == "" {
		panic("PORT undefined")
	}

	validate = validator.New()

	log.Printf("Starting service %s listening on port %s, requests will be added to queue %s",
		sn, port, cfg.QueueName)

	// run ListenAndServe in a separate go routine so main can listen for signals
	go startListening(":"+port, middleware.LogReqResp(router))

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	s := <-signals
	log.Printf("\n%s.main, received signal %s, terminating", sn, s.String())
}

func startListening(addr string, handler http.Handler) {
	if err := http.ListenAndServe(addr, handler); err != http.ErrServerClosed {
		log.Fatalf("%s.startListening, ListenAndServe returned err: %+v\n", serviceInfo.GetServiceName(), err)
	}
}

// catch recover() and log it
func catch() {
	defer func() {
		if r := recover(); r != nil {
			log.Fatalf("=====> RECOVER in %s.main.catch, recover() returned: %v\n", serviceInfo.GetServiceName(), r)
		}
	}()
}

// ********** ********** ********** ********** ********** **********

// taskHandler processes task requests.
func taskHandler(q queue.Queue) httprouter.Handle {
	sn := serviceInfo.GetServiceName()

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// log.Printf("%s.taskHandler, request: %+v, params: %+v\n", sn, r, p)
		startTime := time.Now().UTC().Format(time.RFC3339Nano)

		// pull task and queue names from App Engine headers
		taskName, queueName := appengine.GetAppEngineInfo(w, r)

		incomingRequest := request.Request{}
		if err := incomingRequest.ReadRequest(w, r, p, validate); err != nil {
			// ReadRequest called http.Error so we just return
			return
		}

		newRequest := incomingRequest

		// TODO: implement tagging QA complete processing
		//
		// The current default processing is TBD

		// add timestamps and get duration
		var duration time.Duration
		var err error
		if duration, err = newRequest.AddTimestamps("BeginTaggingQAComplete", startTime, "EndTaggingQAComplete"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// TODO: write updated Request to the Requests database
		_ = repo

		// create task on the next pipeline stage's queue with updated request
		if err := q.Add(&qi, &newRequest); err != nil {
			log.Printf("%s.taskHandler, q.Add error: %+v\n", sn, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// send response to Cloud Tasks
		w.WriteHeader(http.StatusOK)
		// Set a non-2xx status code to indicate a failure in task processing that should be retried.
		// For example, http.Error(w, "Internal Server Error: Task Processing", http.StatusInternalServerError)

		log.Printf("%s.taskHandler completed in %v: queue %q, task %q, newRequest: %+v",
			sn, duration, queueName, taskName, newRequest)
	}
}

// ********** ********** ********** ********** ********** **********

// indexHandler responds to requests with "service running"
func indexHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sn := serviceInfo.GetServiceName()

	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// I'm not dead yet
	fmt.Fprintf(w, "%q service running\n", sn)
}

// ********** ********** ********** ********** ********** **********

func myNotFound(w http.ResponseWriter, r *http.Request) {
	var msg404 = []byte("<h2>404 Not Foundw</h2>")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(msg404)
}
