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
	SelectedFields []int   `json:"SelectedField"`
	BetAmount      float64 `json:"BetAmount"`
	CoinType       string  `json:"CoinType"`
}

type cryptoKeonResponse struct {
	Payout    float64 `json:"Payout"` // 倍率
	WinFields []int   `json:"WinFields"`
	Profit    float64 `json:"Profit"`
	CoinType  string  `json:"CoinType"`
}

type keonRequest struct {
	Req  cryptoKeonRequest
	Resp chan cryptoKeonResponse
    Name string  `json:"Name"`
}

const (
	maxWorkers = 2
	maxQueue   = 100
)

func main() {

	InitSQLConnect()

	var (
		queueSize  int64
		activeTask int64
	)

    taskQueue := make(chan keonRequest, maxQueue)
    errChan := make(chan error)

	// create workers
	for  i := 0; i < maxWorkers; i++ {
		go initBetTaskQueue(taskQueue, &queueSize, &activeTask, errChan)
	}

	// Set up the http handler function
	http.HandleFunc("/cryptokeon", cryptoKeon(&queueSize, &activeTask, taskQueue, errChan))

	// API
	http.HandleFunc("/getPlayerBalance", getPlayerBalance)
	http.HandleFunc("/getAllBetHistory", getAllBetHistory)
	http.HandleFunc("/handleLogin", handleLogin)

	// Start server
	log.Fatal(http.ListenAndServe(":5566", nil))
}

func initBetTaskQueue(taskQueue chan keonRequest, queueSize *int64, activeTask *int64, errChan chan error) {
    defer func() {
        if r := recover(); r != nil {
            log.Println("initBetTaskQueue defer recovered from panic:", r)
            errChan <- fmt.Errorf("%v",r)
        }
        
        close(errChan)
    }()

    var err error
    for req := range taskQueue {
		// Check if there are too many active tasks
		if atomic.LoadInt64(activeTask) >= maxWorkers {
            log.Println("activeTask >= maxWorkers")
            log.Println("queueSize:", atomic.LoadInt64(queueSize), "activeTask:", atomic.LoadInt64(activeTask))
			atomic.AddInt64(queueSize, -1)
            continue 
		}

        atomic.AddInt64(activeTask, 1)
        payout, winfields, profit := SettleKeno(req.Req.SelectedFields, req.Req.BetAmount)
        // 不同幣種的下限值會不一樣, 這邊還需要再優化
        switch req.Req.CoinType {
        case "ETH":
            if req.Req.BetAmount < 0.0001 {
                err = fmt.Errorf("ether bet amount low to minbet")
                errChan <- err
                continue
            }
        case "USDT":
            if req.Req.BetAmount < 1.0 {
                err = fmt.Errorf("usdt bet amount low to minbet")
                errChan <- err
                continue
            }

        default:
            err = fmt.Errorf("bet coin not supported")
            errChan <- err
            continue
        }
        wf, err := json.Marshal(winfields)
        if err != nil {
            panic(err)
        }
        grs := GameResult{
            Payout:    payout,
            WinFields: string(wf),
            Profit:    profit,
            Coin:      req.Req.CoinType,
            Name:      req.Name,
        }

        // response
        resp := cryptoKeonResponse{
            Payout:    payout,
            WinFields: winfields,
            Profit:    profit,
            CoinType:  req.Req.CoinType,
        }

        if err := appendBetHistory(grs, req.Req.BetAmount); err != nil { 
            continue
        }
        
        req.Resp <- resp
        atomic.AddInt64(queueSize, -1)
        atomic.AddInt64(activeTask, -1)
    }
}

func cryptoKeon(queueSize *int64, activeTask *int64, taskQueue chan keonRequest, errChan chan error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		crosSettings(w)

        verify, name := tokenVerfiy(w, r)
        if !verify {
            return
        }

		if r.Method != "POST" {
			http.Error(w, "Only POST requests are supported", http.StatusMethodNotAllowed)
			return
		}

		// Limit the number of requests to maxQueue
		if atomic.LoadInt64(queueSize) >= maxQueue {
			http.Error(w, "Too many requests queued", http.StatusServiceUnavailable)
			log.Println("queueSize:", atomic.LoadInt64(queueSize), "activeTask:", atomic.LoadInt64(activeTask))
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
            Name: name,
		}

		taskQueue <- req
        
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

        case err := <-errChan:
            http.Error(w, err.Error(), http.StatusInternalServerError)
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

    verify, _ := tokenVerfiy(w, r)
	if !verify {
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

    verify, _ := tokenVerfiy(w, r)
	if !verify {
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
