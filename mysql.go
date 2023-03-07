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
)

type GameResult struct {
	GameId    uint64  `gorm:"primaryKey;autoIncrement"`
	Payout    float64 `gorm:"column:payout"`
	WinFields string  `gorm:"column:win_fields;type:text"`
	Profit    float64 `gorm:"column:profit"`
	Coin      string  `gorm:"column:coin"`
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
	err = DB.AutoMigrate(&GameResult{})
	if err != nil {
		panic(err)
	}
}

func AppendBetHistory(gameResult GameResult) {
    defer handlePanic()

    // 开始事务
    tx := DB.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            log.Println("defer recovered from panic:", r)
        }
    }()

    // 获取行锁
    tx.WithContext(context.Background()).Clauses(
		clause.Locking{Strength: "UPDATE"}).Find(&GameResult{})


    // 执行更新操作
    if err := tx.Create(&gameResult).Error; err != nil {
        tx.Rollback()
        panic(err)
    }

    // 提交事务
    if err := tx.Commit().Error; err != nil {
        panic(err)
    }
}


func handlePanic() {
	if r := recover(); r != nil {
		log.Println("recovered from panic:", r)
	}
}
