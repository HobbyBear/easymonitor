package main

import (
	"io/ioutil"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/alert_log", func(writer http.ResponseWriter, request *http.Request) {
		request.ParseForm()
		data, err := ioutil.ReadAll(request.Body)
		if err != nil {
			panic(err)
		}
		log.Println("收到报警日志", string(data))
	})
	log.Println("webhookserver start ")
	log.Println(http.ListenAndServe(":16060", nil))
}
