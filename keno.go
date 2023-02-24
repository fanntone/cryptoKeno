// 宣告程式屬於哪個 package
package main

// 引入套件
import (
	// "fmt"	
	"math/rand"
	"time"
	"log"
	"strconv"
	"math/big"
	"math"

	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	bip39 "github.com/tyler-smith/go-bip39"
)

// GetSefeRandomSeed is prodiver safe seed function
func GetSefeRandomSeed() int64 {
	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		log.Fatal(err)
	}

	mnemonic, _ := bip39.NewMnemonic(entropy)
	seed := bip39.NewSeed(mnemonic, "")

	// fmt.Println("memonic: ", mnemonic)
	// fmt.Println("seed: ", seed)

	wallet, err := hdwallet.NewFromSeed(seed)
	if err != nil {
		log.Fatal(err)
	}

	path := hdwallet.MustParseDerivationPath("m/44'/60'/0'/0/0")
	account, err := wallet.Derive(path, false)
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Println(account.Address.Hex())
	str := account.Address.Hex()
	subs := str[2:9]
	res, err := strconv.ParseInt(subs, 16, 64)

	if err != nil {
		log.Fatal(err)
	}

	return res
}

// GetRandom is a call func
// retrun 0 ~ num-1.
func GetRandom(num int) int {
	s1 := rand.NewSource(GetSefeRandomSeed())
	r1 := rand.New(s1)
	x := r1.Intn(num)

	return x
}

func RandomList() []int {
    //Provide seed
    rand.Seed(time.Now().UnixNano())

    //Generate a random array of length form 0~num-1
    perm := rand.Perm(40)
	list := []int{0,0,0,0,0,0,0,0,0,0}
    for i := range perm {
		if i > 9 {
			break
		}
        list[i] = perm[i]+1
    }
	return list
}

func SettleKeno(selectedFields []int, betAmount float64) (string, []int, float64){
    randNums := RandomList() // 產生一個亂數陣列

    inputNums := selectedFields // 輸入的不定長度陣列

    // 建立一個 map 來記錄亂數陣列中每個數字出現的次數
    randNumsCount := make(map[int]int)
    for _, n := range randNums {
        randNumsCount[n]++
    }

    // 比對輸入陣列中每個數字在亂數陣列中出現的次數，計算相同的個數
    sameCount := 0
    for _, n := range inputNums {
        if count, ok := randNumsCount[n]; ok && count > 0 {
            sameCount++
            randNumsCount[n]--
        }
    }
	payout := PayoutMap(len(selectedFields))[sameCount]

	// f
	profit := CalProfit(betAmount, payout)
	return strconv.FormatFloat(payout, 'f', 2, 64), randNums, profit
}

func CalProfit(amount float64, pay float64) float64 {
		// Multiply two big floats
		betAmount := big.NewFloat(amount)
		payout := big.NewFloat(pay)
		profit := new(big.Float).Sub(
			new(big.Float).Mul(betAmount, payout),
			betAmount,
		)
		roundedProfit, _ := profit.Float64()
		roundedProfit = floatRound(roundedProfit, 8)

		return roundedProfit
}

func floatRound(x float64, prec int) float64 {
	pow := math.Pow(10, float64(prec))
	return math.Round(x*pow) / pow
}

func PayoutMap(len int) []float64 {
	if len == 1 {
		return []float64{0.0, 3.96}
	} else if len == 2 {
		return []float64{0.0, 1.9, 4.5}
	} else if len == 3 {
		return []float64{0.0, 1.0, 3.1, 10.4}
	} else if len == 4 {
		return []float64{0.0, 0.8, 1.8, 5.0, 22.5}
	} else if len == 5 {
		return []float64{0.0, 0.25, 1.4, 4.1, 16.5, 36.0}
	} else if len == 6 {
		return []float64{0.0, 0.0, 1.0, 3.68, 7.0, 16.5, 40.0}
	} else if len == 7 {
		return []float64{0.0, 0.0 ,0.47, 3.0, 4.5, 14.0, 31.0, 60.0}
	} else if len == 8 {
		return []float64{0.0, 0.0, 0.0, 2.2, 4.0, 13.0, 22.0, 55.0, 70.0}
	} else if len == 9 {
		return []float64{0.0, 0.0, 0.0, 1.55, 3.0, 8.0, 15.0, 44.0, 60.0, 70.0}
	} else if len == 10 {
		return []float64{0.0, 0.0, 0.0, 1.4, 2.25, 4.5, 8.0, 17.0, 50.0, 80.0, 100.0}
	}

	return []float64{0}
}