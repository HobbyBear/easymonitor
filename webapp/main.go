package main

import (
	"easymonitor/infra"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strings"
)

func main() {
	infra.RegPrometheusClient()
	infra.InitHookDb([]*infra.DbInfo{
		{
			MaxConn: 10,
			Timeout: 10,
			ConnStr: "root:1234567@tcp(mydb:3306)/test",
			DbName:  "test",
		},
	})
	infra.SqlParser.SetFixName(func(name string) string {
		// 对分表的处理
		if strings.Contains(name, "t_user_") {
			return "t_user"
		}
		return name
	})

	router := http.NewServeMux()
	// 创建一个处理程序函数
	handler := http.HandlerFunc(handleRequest)
	// 使用中间件包装处理程序函数
	middleware := infra.MetricMiddleware(handler)
	// 注册处理程序和中间件到路由器
	router.Handle("/", middleware)
	log.Infof("webapp start")
	// 启动HTTP服务器
	http.ListenAndServe(":8080", router)
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
