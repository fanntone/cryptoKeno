package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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
	SelectedFields []int   `json:"SelectedField"`
	BetAmount      float64 `json:"BetAmount"`
	CoinType       string  `json:"CoinType"`
}

type cryptoKeonResponse struct {
	Payout    string  `json:"Payout"` // 倍率
	WinFields []int   `json:"WinFields"`
	Profit    float64 `json:"Profit"`
	CoinType  string  `json:"CoinType"`
}

type keonRequest struct {
	Req  cryptoKeonRequest
	Resp chan cryptoKeonResponse
}

const (
	maxWorkers = 5
	maxQueue   = 50
)

func main() {

	InitSQLConnect()

	var (
		queueSize  int64
		activeTask int64
	)

	taskQueue := make(chan request, maxQueue)
    taskQueue2 := make(chan keonRequest, maxQueue)


	// create workers
	for i := 0; i < maxWorkers; i++ {
        go initSampleTaskQueue(taskQueue, queueSize, activeTask)
		go initBetTaskQueue(taskQueue2, queueSize, activeTask)
	}

	// test

	// Set up the http handler function
	http.HandleFunc("/testrand", handleRequest(&queueSize, &activeTask, taskQueue))
	http.HandleFunc("/cryptokeon", cryptoKeon(&queueSize, &activeTask, taskQueue2))

	// API
	http.HandleFunc("/getPlayerBalance", getPlayerBalance)
	http.HandleFunc("/getAllBetHistory", getAllBetHistory)
	http.HandleFunc("/handleLogin", handleLogin)

	// Start server
	log.Fatal(http.ListenAndServe(":5566", nil))
}

func initSampleTaskQueue(taskQueue chan request, queueSize int64, activeTask int64) {
    for req := range taskQueue {
        resp := randResponse{
            Result: GetRandom(req.Req.Range),
        }
        req.Resp <- resp
        atomic.AddInt64(&queueSize, -1)
        atomic.AddInt64(&activeTask, -1)
    }
}

func initBetTaskQueue(taskQueue2 chan keonRequest, queueSize int64, activeTask int64) {
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
        grs := GameResult{
            Payout:    payout,
            WinFields: string(wf),
            Profit:    profit,
            Coin:      req.Req.CoinType,
        }

        // response
        resp := cryptoKeonResponse{
            Payout:    strconv.FormatFloat(payout, 'f', 2, 64),
            WinFields: winfields,
            Profit:    profit,
            CoinType:  req.Req.CoinType,
        }

        if (!AppendBetHistory(grs, req.Req.BetAmount)){
            continue
        }
        req.Resp <- resp
        atomic.AddInt64(&queueSize, -1)
        atomic.AddInt64(&activeTask, -1)
    }
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

		if !tokenVerfiy(w, r) {
			return
		}

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

func crosSettings(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:8080")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
}

func getPlayerBalance(w http.ResponseWriter, r *http.Request) {
	crosSettings(w)

	if !tokenVerfiy(w, r) {
		return
	}

	type PlayerBalanceRequest struct {
		Wallet string `json:"wallet"`
	}
	var req PlayerBalanceRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	type PlayerBalanceResponse struct {
		Amount float64 `json:"amount"`
	}
	resp := PlayerBalanceResponse{
		Amount: getPlayerBalanceFromDB(req.Wallet),
	}

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Println("Failed to write response:", err)
	}
}

func getAllBetHistory(w http.ResponseWriter, r *http.Request) {
	crosSettings(w)

	if !tokenVerfiy(w, r) {
		return
	}

	type AllBetHistoryRequest struct {
		GameId uint64 `json:"game_id"`
	}
	var req AllBetHistoryRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	type AllBetHistoryReponse struct {
		Records []GameResult `json:"records"`
	}
	resp := AllBetHistoryReponse{
		Records: getAllBetHistoryFromDB(req.GameId),
	}

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Println("Failed to write response:", err)
	}
}
