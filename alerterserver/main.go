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
		// todo 这里仅仅是简单打印下收到的日志，
		// todo 实际上收到日志后应该根据日志里的应用服务名称，报警内容 采取不同的报警策略，比如发到钉钉群。

	})
	http.HandleFunc("/alert_grafana", func(writer http.ResponseWriter, request *http.Request) {
		request.ParseForm()
		data, err := ioutil.ReadAll(request.Body)
		if err != nil {
			panic(err)
		}
		log.Println("收到grafana报警日志", string(data))
		// todo 这里仅仅是简单打印下收到的日志，
		// todo 实际上收到日志后应该根据日志里的应用服务名称，报警内容 采取不同的报警策略，比如发到钉钉群。

	})
	log.Println("alerterserver start ")
	log.Println(http.ListenAndServe(":16060", nil))
}
