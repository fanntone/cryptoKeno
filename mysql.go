package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"context"
	"gorm.io/gorm/clause"
	"time"
	"math/big"
)

type GameResult struct {
	GameId    	uint64  		`gorm:"primaryKey;autoIncrement"`
	Payout    	float64 		`gorm:"column:payout"`
	WinFields 	string  		`gorm:"column:win_fields;type:text"`
	Profit    	float64 		`gorm:"column:profit"`
	Coin      	string  		`gorm:"column:coin"`
	CreatedAt  	time.Time 		`gorm:"column:created_at"`
	UpdatedAt  	time.Time 		`gorm:"column:updated_at"`
}

type Member struct {
	MemberId 	uint64 		`gorm:"primaryKey;autoIncrement"`
	Email 		string 		`gorm:"column:email;type:text"`
	Wallet 		string 		`gorm:"column:wallet;type:text"` 
	PrivateKey  string 		`gorm:"column:private_key;type:text"`
	Balance 	float64 	`gorm:"column:balance"`
	Name 		string 		`gorm:"column:name"`
	Password 	string 		`gorm:"cloumn:password"`
	CreatedAt   time.Time 	`gorm:"column:created_at"`
	UpdatedAt   time.Time 	`gorm:"column:updated_at"`
}

const (
	UserName     string = "root"
	Password     string = "123456"
	Addr         string = "127.0.0.1"
	Port         int    = 3306
	Database     string = "Bet"
	MaxLifetime  int    = 10
	MaxOpenConns int    = 10
	MaxIdleConns int    = 10
)

var (
	DB     *gorm.DB
	dbOnce sync.Once
)

// 設置資料表名稱為 game_results
func (GameResult) TableName() string {
	return "game_results"
}

func (Member) TableName() string {
	return "members"
}

func (g *GameResult) BeforeSave(tx *gorm.DB) error {
    winFieldsBytes, err := json.Marshal(g.WinFields)
    if err != nil {
        return err
    }
    if err := json.Unmarshal(winFieldsBytes, &g.WinFields); err != nil {
        return err
    }
    return nil
}

// func (g *GameResult) AfterFind(tx *gorm.DB) error {
//     winFieldsBytes := []byte(fmt.Sprintf("%v", g.WinFields))
//     var winFields []int
//     if err := json.Unmarshal(winFieldsBytes, &winFields); err != nil {
//         return err
//     }
//     g.WinFields = winFields
//     return nil
// }


func InitSQLConnect() {
	defer handlePanic()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True", UserName, Password, Addr, Port, Database)
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	// 初始化表结构
	err = DB.AutoMigrate(&GameResult{}, &Member{})
	if err != nil {
		panic(err)
	}
}

func appendBetHistory(gameResult GameResult, betAmount float64) error {
    // 開始事務
    tx := DB.Begin()
	var err error
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            log.Println("AppendBetHistory defer recovered from panic:", r)
			panic(r)
        }
    }()


    // 獲取行鎖(必須)
    tx.WithContext(context.Background()).Clauses(
		clause.Locking{Strength: "UPDATE"}).Find(&GameResult{})


    // 執行更新
    if err = tx.Create(&gameResult).Error; err != nil {
        tx.Rollback()
        panic(err)
    }

 	if err = updatePlayerBalance(gameResult.Profit, betAmount); err != nil {
		tx.Rollback()
		return err
	}


    // 提交事務
    if err = tx.Commit().Error; err != nil {
        panic(err)
    }
	
	return nil
}


func handlePanic() {
	if r := recover(); r != nil {
		log.Println("recovered from panic:", r)
	}
}

func updatePlayerBalance(profit float64, betAmount float64) error {
	tx := DB.Begin()
	var err error
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            log.Println("updatePlayerBalance defer recovered from panic:", r)
			panic(r)
        }
    }()

	var user Member
	
	// 獲取行鎖(必須)
	err = tx.WithContext(context.Background()).Clauses(
		clause.Locking{Strength: "UPDATE"}).
		Where("wallet = ?", "0x21afd6eeC226Bebcb6Ce290a7710677F1CDE3eF6").
		First(&user).
		Error
	if err != nil {
		panic(err)
	}

	if user.Balance < betAmount {
		tx.Rollback()
		return fmt.Errorf("user.Balance < betAmount")
	}

	balance := bigFloatAdd(user.Balance, profit)
	
	// Update 
	tx.Model(&user).Update("balance", balance)
	
	// 提交事務
	if err := tx.Commit().Error; err != nil {
		panic(err)
	}

	return nil
}

func bigFloatAdd(a float64, b float64) float64{
	x := big.NewFloat(a)
	y := big.NewFloat(b)
	z := new(big.Float).Add(x,y)
	balance, _ := z.Float64()
	balance = floatRound(balance, 8)

	return balance
}

// For API
func getPlayerBalanceFromDB(wallet string) float64{
	var user Member
	DB.Where("wallet", wallet).First(&user)

	return user.Balance
}


func getAllBetHistoryFromDB(game_id uint64) []GameResult {
	var grs []GameResult
	var max uint64 = 20
	var limit int = 20
	
	if game_id > 0 {
		DB.Where("game_id > ? AND game_id <= ?", game_id, game_id + max).
		Order("game_id desc").
		Find(&grs)
		if len(grs) == 0 {
			DB.Order("game_id desc").Limit(limit).Find(&grs)
		}
	} else {
		DB.Order("game_id desc").Limit(limit).Find(&grs)
	}
	
	return grs
}

func getUserDataFromDB(name string) (string,string) {
	var user Member

	DB.Where("name", name).First(&user)
	return user.Name, user.Password
}
