package infra

import (
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"time"
)

func init() {
	os.Mkdir("/logs", os.ModePerm)
	path := filepath.Base(os.Args[0])
	writer, _ := rotatelogs.New(
		"/logs/"+path+".%Y%m%d.log",
		rotatelogs.WithMaxAge(time.Hour*24*3),
		rotatelogs.WithRotationTime(24*time.Hour),
	)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(writer)
}
