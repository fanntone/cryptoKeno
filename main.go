package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
    "strconv"
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
    SelectedFields  []int       `json:"SelectedField"`
    BetAmount       float64     `json:"BetAmount"`
    CoinType        string      `json:"CoinType"`     
}

type cryptoKeonResponse struct {
    Payout     string    `json:"Payout"`    // 倍率
    WinFields  []int     `json:"WinFields"`  
    Profit     float64   `json:"Profit"`    
    CoinType   string    `json:"CoinType"`     
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

    InitSQLConnect()

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
                payout, winfields, profit := SettleKeno(req.Req.SelectedFields, req.Req.BetAmount)
				// 不同幣種的下限值會不一樣, 這邊還需要再優化
                switch req.Req.CoinType {
                case "ETH":
                    if req.Req.BetAmount < 0.0001 {
                        continue
                    }
                case "USDT":
                    if req.Req.BetAmount < 1.0 {
                        continue
                    }

                default:
                    continue
                }                           
                wf, err := json.Marshal(winfields)
                if err != nil {
                    continue
                }
                grs := GameResult {
                    Payout: payout,
                    WinFields: string(wf),
                    Profit: profit,
                    Coin:req.Req.CoinType,
                }

                // response
                resp := cryptoKeonResponse{
					Payout: strconv.FormatFloat(payout, 'f', 2, 64),
                    WinFields: winfields,
                    Profit: profit,
                    CoinType: req.Req.CoinType,
				}

                AppendBetHistory(grs)
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
        crosSettings(w)

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
        crosSettings(w)

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
			Req: cryptokeonReq,
			Resp: respChan,
		}
        // log.Println("req.Req", cryptokeonReq)

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

func crosSettings(w http.ResponseWriter) {
    w.Header().Set("Access-Control-Allow-Origin", "http://localhost:8080")
    w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
    w.Header().Set("Access-Control-Allow-Credentials", "true")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
}