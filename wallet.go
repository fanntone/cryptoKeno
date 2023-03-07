package main

import (
	"fmt"
	"github.com/miguelmota/go-ethereum-hdwallet"
	bip39 "github.com/tyler-smith/go-bip39"
	"log"

)

func generateWallet(n int) ([]string,[]string){
	defer handlePanic()

	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		log.Fatal(err)
	}

	mnemonic, _ := bip39.NewMnemonic(entropy)
	seed := bip39.NewSeed(mnemonic, "")

	numAccounts := n

	wallet, err := hdwallet.NewFromSeed(seed)
	if err != nil {
		panic(err)
	}

	accounts := make([]string, numAccounts)
	privateKeys := make([]string, numAccounts)

	for i := 0; i < numAccounts; i++ {
		path := hdwallet.MustParseDerivationPath(fmt.Sprintf("m/44'/60'/0'/0/%d", i))
		account, err := wallet.Derive(path, false)
		if err != nil {
			panic(err)
		}

		accounts[i] = account.Address.String()
		privateKeys[i], err = wallet.PrivateKeyHex(account)
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("Accounts:", accounts)
	fmt.Println("Private keys:", privateKeys)
	fmt.Println("===================================")
	
	return accounts, privateKeys
}
