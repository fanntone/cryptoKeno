package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

type randRequest struct {
	Range int `json:"range"`
}

type randResponse struct {
	Result int `json:"result"`
}

type request struct {
	Req  randRequest
	Resp chan randResponse
}

type cryptoKeonRequest struct {
    SelectedFields	  []int      `json:"selectedField"`
    BetAmount         float64    `json:"betAmount"`
}

type cryptoKeonResponse struct {
    Payout     string    `json:"Payout"`  
    WinFields  []int     `json:"WinFields"`
    Profit     float64   `json:"Profit"`
}

type keonRequest struct {
    Req cryptoKeonRequest
    Resp chan cryptoKeonResponse
}

const (
	maxWorkers = 3
	maxQueue   = 5
)

func main() {
	var (
		queueSize  int64
		activeTask int64
	)

	taskQueue  := make(chan request, maxQueue)
    taskQueue2 := make(chan keonRequest, maxQueue) 

	// create workers
	for i := 0; i < maxWorkers; i++ {
		go func() {
			for req := range taskQueue {
				resp := randResponse{
					Result: GetRandom(req.Req.Range),
				}
				req.Resp <- resp
				atomic.AddInt64(&queueSize, -1)
				atomic.AddInt64(&activeTask, -1)
			}
		}()

        go func() {
			for req := range taskQueue2 {
				resp := cryptoKeonResponse{
					Payout: "0.000x",
                    WinFields: []int{39,24,37,34,6,2,11,18,32,19},
                    Profit: -0.00000100,
				}
				req.Resp <- resp
				atomic.AddInt64(&queueSize, -1)
				atomic.AddInt64(&activeTask, -1)
			}
		}()
	}

	// Set up the http handler function
	http.HandleFunc("/testrand", handleRequest(&queueSize, &activeTask, taskQueue))
    http.HandleFunc("/cryptokeon", cryptoKeon(&queueSize, &activeTask, taskQueue2))

	log.Fatal(http.ListenAndServe(":5566", nil))
}

func handleRequest(queueSize *int64, activeTask *int64, taskQueue chan request) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "http://localhost:8081")
        w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method != "POST" {
			http.Error(w, "Only POST requests are supported", http.StatusMethodNotAllowed)
			return
		}

		// Limit the number of requests to maxQueue
		if atomic.LoadInt64(queueSize) >= maxQueue {
			http.Error(w, "Too many requests queued", http.StatusServiceUnavailable)
			fmt.Println("queueSize:", atomic.LoadInt64(queueSize), "activeTask:", atomic.LoadInt64(activeTask))
			return
		}
		atomic.AddInt64(queueSize, 1)

		// Decode the request
		var randReq randRequest
		err := json.NewDecoder(r.Body).Decode(&randReq)
		if err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Create a channel for the response
		respChan := make(chan randResponse)

		// Wrap the request in a task function and send it to the task queue
		req := request{
			Req:  randReq,
			Resp: respChan,
		}

		// Check if there are too many active tasks
		if atomic.LoadInt64(activeTask) >= maxWorkers {
			http.Error(w, "Too many active tasks", http.StatusServiceUnavailable)
			atomic.AddInt64(queueSize, -1)
			return
		}

		taskQueue <- req
		atomic.AddInt64(activeTask, 1)

		// Wait for the response
		select {
		case resp := <-respChan:
			close(respChan)
			err := json.NewEncoder(w).Encode(resp)
			if err != nil {
				log.Println("Failed to write response:", err)
			}
		case <-time.After(10 * time.Second):
			http.Error(w, "Request timed out", http.StatusRequestTimeout)
			atomic.AddInt64(queueSize, -1)
			atomic.AddInt64(activeTask, -1)
		}
	}
}

func cryptoKeon(queueSize *int64, activeTask *int64, taskQueue chan keonRequest) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "http://192.168.0.133:8080")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Credentials", "true")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
        
        if r.Method != "POST" {
			http.Error(w, "Only POST requests are supported", http.StatusMethodNotAllowed)
			return
		}

		// Limit the number of requests to maxQueue
		if atomic.LoadInt64(queueSize) >= maxQueue {
			http.Error(w, "Too many requests queued", http.StatusServiceUnavailable)
			fmt.Println("queueSize:", atomic.LoadInt64(queueSize), "activeTask:", atomic.LoadInt64(activeTask))
			return
		}
		atomic.AddInt64(queueSize, 1)

		// Decode the request
		var cryptokeonReq cryptoKeonRequest
		err := json.NewDecoder(r.Body).Decode(&cryptokeonReq)
		if err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Create a channel for the response
		respChan := make(chan cryptoKeonResponse)

		// Wrap the request in a task function and send it to the task queue
		req := keonRequest{
			Req:  cryptokeonReq,
			Resp: respChan,
		}

		// Check if there are too many active tasks
		if atomic.LoadInt64(activeTask) >= maxWorkers {
			http.Error(w, "Too many active tasks", http.StatusServiceUnavailable)
			atomic.AddInt64(queueSize, -1)
			return
		}

		taskQueue <- req
		atomic.AddInt64(activeTask, 1)

		// Wait for the response
		select {
		case resp := <-respChan:
			close(respChan)
			err := json.NewEncoder(w).Encode(resp)
			if err != nil {
				log.Println("Failed to write response:", err)
			}
		case <-time.After(10 * time.Second):
			http.Error(w, "Request timed out", http.StatusRequestTimeout)
			atomic.AddInt64(queueSize, -1)
			atomic.AddInt64(activeTask, -1)
		}
	}
}