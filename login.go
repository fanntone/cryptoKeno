package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
)

var secretKey = []byte("my_secret_key")

type Claims struct {
    Username string `json:"username"`
    jwt.StandardClaims
}

type loginRequest struct {
	UserName 	string 		`json:"username"`
	Password 	string 		`json:"password"`
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
    // 解析用戶提交的用戶名稱和密碼
	var req loginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

    username := req.UserName
    password := req.Password
	nameDB, pwdDB := getUserDataFromDB(username)
    // 假設這裡進行用戶名和密碼驗證
    if username != nameDB || password != pwdDB {
        w.WriteHeader(http.StatusUnauthorized)
        fmt.Fprintln(w, "Invalid username or password")
        return
    }

    expirationTime := time.Now().Add(30 * time.Minute)
    claims := &Claims{
        Username: username,
        StandardClaims: jwt.StandardClaims{
            ExpiresAt: expirationTime.Unix(),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signedToken, err := token.SignedString(secretKey)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        fmt.Fprintln(w, "Error signing token")
        return
    }
    fmt.Fprintln(w, signedToken)
}

func tokenVerfiy(w http.ResponseWriter, r *http.Request) bool {
    // 檢查用戶的JWT是否合規
    tokenString := r.Header.Get("Authorization")
    if tokenString == "" {
        w.WriteHeader(http.StatusUnauthorized)
        fmt.Fprintln(w, "Missing token in request header")
        return false
    }
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return secretKey, nil
    })
    if err != nil {
        if err == jwt.ErrSignatureInvalid {
            w.WriteHeader(http.StatusUnauthorized)
            fmt.Fprintln(w, "Invalid token signature")
            return false
        }
        w.WriteHeader(http.StatusBadRequest)
        fmt.Fprintln(w, "Invalid token")
        return false
    }
    if !token.Valid {
        w.WriteHeader(http.StatusUnauthorized)
        fmt.Fprintln(w, "Invalid token")
        return false
    }
    _, ok := token.Claims.(*Claims)
    if !ok {
        w.WriteHeader(http.StatusInternalServerError)
        fmt.Fprintln(w, "Error getting claims from token")
        return false
    }
    // fmt.Fprintf(w, "Welcome %s!", claims.Username)
	return true
}
