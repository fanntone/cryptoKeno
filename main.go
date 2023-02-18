package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
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

const (
	maxWorkers = 10
	maxQueue   = 10
)

func main() {
	var (
		mutex      sync.Mutex
		queueSize  int64
		activeTask int64
	)

	taskQueue := make(chan request, maxQueue)

	// create workers
	for i := 0; i < maxWorkers; i++ {
		go func() {
			for req := range taskQueue {
				resp := randResponse{
					Result: GetRandom(req.Req.Range),
				}
				req.Resp <- resp
				mutex.Lock()
				queueSize--
				activeTask--
				fmt.Println("queueSize:", queueSize, "activeTask:", activeTask)
				mutex.Unlock()
			}
		}()
	}

	http.HandleFunc("/testrand", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Only POST requests are supported", http.StatusMethodNotAllowed)
			return
		}

		// Limit the number of requests to maxQueue
		mutex.Lock()
		if queueSize >= maxQueue {
			mutex.Unlock()
			http.Error(w, "Too many requests queued", http.StatusServiceUnavailable)
			return
		}
		queueSize++
		mutex.Unlock()

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
		taskQueue <- req
		activeTask++
		fmt.Println("queueSize:", queueSize, "activeTask:", activeTask)

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
			mutex.Lock()
			queueSize--
			activeTask--
			mutex.Unlock()
		}
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
