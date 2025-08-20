package logger

import (
	"fmt"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
	"os"
	"runtime"
	"time"
)

func NewLogger(logType string) *logrus.Logger {
	log := logrus.New()
	logPath := "./logs/" + logType
	_ = os.MkdirAll(logPath, 0755)

	writer, _ := rotatelogs.New(
		logPath+"/"+logType+".log.%Y-%m-%d",
		rotatelogs.WithLinkName(logPath+"/"+logType+".log"),
		rotatelogs.WithRotationTime(24*time.Hour),
		rotatelogs.WithMaxAge(7*24*time.Hour),
	)

	log.SetOutput(writer)
	log.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			// 自定义显示格式：函数名 + 文件路径
			funcName := f.Function
			fileLine := fmt.Sprintf("%s:%d", f.File, f.Line)
			return funcName, fileLine
		},
	})
	log.SetLevel(logrus.InfoLevel)

	return log
}
