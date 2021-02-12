package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

func broker() (mssg chan string, reg func(*http.Request) chan string, unreg func(*http.Request)) {
	mssgs := map[*http.Request]chan string{}
	mssg = make(chan string)
	var mtx sync.Mutex
	go func() {
		for {
			_mssg := <-mssg
			mtx.Lock()
			_mssgs := mssgs
			mtx.Unlock()
			for _, __mssg := range _mssgs {
				__mssg <- _mssg
			}
		}
	}()
	reg = func(r *http.Request) (mssg chan string) {
		mssg = make(chan string)
		mtx.Lock()
		mssgs[r] = mssg
		mtx.Unlock()
		return
	}
	unreg = func(r *http.Request) {
		mtx.Lock()
		close(mssgs[r])
		delete(mssgs, r)
		mtx.Unlock()
	}
	return
}
func index(file *os.File) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		io.Copy(w, file)
		file.Seek(0, 0)
	}
}
func sse(reg func(*http.Request) chan string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header()["Content-Type"] = []string{"text/event-stream"}
		flusher, _ := w.(http.Flusher)
		flusher.Flush()
		mssg := reg(r)
		for {
			_mssg := <-mssg
			println(_mssg)
			fmt.Fprintf(w, "data: %s\n\n", _mssg)
			flusher.Flush()
		}
	}
}
func message(mssg chan string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mssg <- r.PostFormValue("mssg")
	}
}
func cl0s3(unreg func(*http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		unreg(r)
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
