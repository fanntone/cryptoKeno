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
	CreatedAt  time.Time 	`gorm:"column:created_at"`
	UpdatedAt  time.Time 	`gorm:"column:updated_at"`
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

func AppendBetHistory(gameResult GameResult) {
    // 開始事務
    tx := DB.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            log.Println("defer recovered from panic:", r)
        }
    }()

    // 獲取行鎖(必須)
    tx.WithContext(context.Background()).Clauses(
		clause.Locking{Strength: "UPDATE"}).Find(&GameResult{})


    // 執行更新
    if err := tx.Create(&gameResult).Error; err != nil {
        tx.Rollback()
        panic(err)
    }

	updatePlayerBalance(gameResult.Profit)

    // 提交事務
    if err := tx.Commit().Error; err != nil {
        panic(err)
    }
}


func handlePanic() {
	if r := recover(); r != nil {
		log.Println("recovered from panic:", r)
	}
}

func updatePlayerBalance(profit float64) {
	tx := DB.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            log.Println("defer recovered from panic:", r)
        }
    }()

	var user Member
	
	// 獲取行鎖(必須)
	err := tx.WithContext(context.Background()).Clauses(
		clause.Locking{Strength: "UPDATE"}).
		Where("Wallet = ?", "0x21afd6eeC226Bebcb6Ce290a7710677F1CDE3eF6").
		First(&user).
		Error
	if err != nil {
		panic(err)
	}
	log.Println("profit:", profit)
	log.Println("balance1: ", user.Balance)
	x := big.NewFloat(user.Balance)
	y := big.NewFloat(profit)
	z := new(big.Float).Add(x,y)
	balance, _ := z.Float64()
	balance = floatRound(balance, 8)
	log.Println("balance2: ", balance)
	
	// Update 
	tx.Model(&user).Update("balance", balance)
	
	// 提交事務
	if err := tx.Commit().Error; err != nil {
		panic(err)
	}
}
