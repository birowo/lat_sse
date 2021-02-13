package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

func setSsnId(w http.ResponseWriter, ttl time.Duration) {
	kuki := []byte("ssnid=01234567891123456789212345678931; Expires=01234567891123456789212345678; HttpOnly")
	//              01234567891123456789212345678931234567894123456789512345678961234567897123456789
	rand.Read(kuki[14:38])
	base64.URLEncoding.Encode(kuki[6:38], kuki[14:38])
	copy(kuki[48:77], time.Now().Add(ttl).Format(http.TimeFormat))
	w.Header()["Set-Cookie"] = []string{string(kuki)}
}
func getSsnId(r *http.Request) (id string, err error) {
	_kuki := r.Header["Cookie"]
	if len(_kuki) == 0 {
		err = errors.New("1.Sess ID Err")
		return
	}
	kuki := _kuki[0]
	kukiLen := len(kuki)
	if kukiLen == 0 {
		err = errors.New("2.Sess ID Err")
		return
	}
	i := 6
	for i < kukiLen && string(kuki[i-6:i]) != "ssnid=" {
		i++
	}
	if (i + 32) <= kukiLen {
		id = kuki[i : i+32]
	} else {
		err = errors.New("3.Sess ID Err")
	}
	return
}
func delSsnId(w http.ResponseWriter) {
	w.Header()["Set-Cookie"] = []string{"ssnid=; Expires=Sat, 01 Jan 2000 00:00:00 GMT"}
}
func broker() (mssg chan string, reg func(string) chan string, unreg func(string)) {
	mssgs := map[string]chan string{}
	mssg = make(chan string, 1)
	var mtx sync.Mutex
	go func() {
		for {
			_mssg := <-mssg
			for _, __mssg := range mssgs {
				__mssg <- _mssg
			}
		}
	}()
	reg = func(id string) (mssg chan string) {
		mssg = make(chan string, 1)
		mtx.Lock()
		mssgs[id] = mssg
		mtx.Unlock()
		return
	}
	unreg = func(id string) {
		mtx.Lock()
		_, ok := mssgs[id]
		if ok {
			println("mssgs len:", len(mssgs), " mssgs[id] len:", len(mssgs[id]))
			delete(mssgs, id)
		} else {
			println("id:", id, " does not exist")
		}
		mtx.Unlock()
	}
	return
}
func index(file *os.File) http.HandlerFunc {
	const expires = 31556952 * time.Second
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := getSsnId(r)
		if id != "" {
			w.Write([]byte("already opened"))
			return
		}
		setSsnId(w, expires)
		io.Copy(w, file)
		file.Seek(0, 0)
	}
}
func sse(reg func(string) chan string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := getSsnId(r)
		if err != nil {
			println(err.Error())
			return
		}
		println("id:", id)
		mssg := reg(id)
		w.Header()["Content-Type"] = []string{"text/event-stream"}
		flusher, _ := w.(http.Flusher)
		flusher.Flush()
		for {
			fmt.Fprintf(w, "data: %s\n\n", <-mssg)
			flusher.Flush()
		}
	}
}
func message(mssg chan string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mssg <- r.PostFormValue("mssg")
	}
}
func cl0s3(unreg func(string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := getSsnId(r)
		if err != nil {
			println(err.Error())
			return
		}
		delSsnId(w)
		unreg(id)
	}
}
func main() {
	file, err := os.Open("client/index.html")
	if err != nil {
		println(err.Error())
		return
	}
	defer file.Close()
	http.HandleFunc("/", index(file))
	mssg, reg, unreg := broker()
	http.HandleFunc("/sse", sse(reg))
	http.HandleFunc("/message", message(mssg))
	http.HandleFunc("/close", cl0s3(unreg))
	http.ListenAndServe(":8080", nil)
}
