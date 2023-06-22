package logger

import (
	"path"
	"runtime"
	"strconv"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
)

type Fields = logrus.Fields
type Level = logrus.Level

const (
	PanicLevel = logrus.PanicLevel
	FatalLevel = logrus.FatalLevel
	ErrorLevel = logrus.ErrorLevel
	WarnLevel  = logrus.WarnLevel
	InfoLevel  = logrus.InfoLevel
	DebugLevel = logrus.DebugLevel
	TraceLevel = logrus.TraceLevel
)

// Create a new instance of the logger. You can have any number of instances.
var log = logrus.New()

func Log() *logrus.Logger {
	return log
}

const (
	timestamp_format string = "2006-01-02 15:04:05.000000" // 时间戳格式
	file_rorate      string = ".%Y%m%d_%H"                 // 日期格式
	file_save_day    int64  = 30                           // 文件保存天数
	info_ext         string = ".INFO"                      // Debug/Info
	warn_ext         string = ".WARNING"                   // Warning
	error_ext        string = ".ERROR"                     // Error
	fatal_ext        string = ".FATAL"                     // Fatal/Panic
)

func Init(
	folder string,
	file_name string,
	log_level Level,
) {
	// 输出日志等级
	log.SetLevel(log_level)

	// 输出打印日志代码行
	log.SetReportCaller(true)

	// 修改json数据key中的func和file的内容
	// 如果任何返回值是空字符串, 则对应的key将从json字段删除。
	func_callerPrettyfier := func(frame *runtime.Frame) (function string, file string) {
		function = ""                                                 // 删除函数信息 func
		file = path.Base(frame.File) + ":" + strconv.Itoa(frame.Line) // file: "文件信息:行号信息"
		return
	}

	// 日志Json格式
	log_json_formatter := &logrus.JSONFormatter{
		TimestampFormat:   timestamp_format,      // 时间戳格式
		DisableHTMLEscape: true,                  // 输出中禁用html转义
		CallerPrettyfier:  func_callerPrettyfier, // 定制日志代码行
	}
	log.SetFormatter(log_json_formatter)

	log.AddHook(newLfsHook(folder, file_name, log_json_formatter))
}

func newLfsHook(
	folder string,
	file_name string,
	formatter logrus.Formatter,
) logrus.Hook {
	info_writer, err := rotatelogs.New(
		// log文件地址
		folder+file_name+info_ext+file_rorate,

		// WithLinkName为最新的日志建立软连接，以方便随着找到当前日志文件
		rotatelogs.WithLinkName(folder+file_name+info_ext),

		// WithRotationTime设置日志分割的时间，这里设置为一小时分割一次
		rotatelogs.WithRotationTime(time.Hour),

		// WithMaxAge 和 WithRotationCount 二者只能设置一个
		// WithMaxAge 设置文件清理前的最长保存时间
		rotatelogs.WithMaxAge(time.Duration(file_save_day)*24*time.Hour),

		// WithRotationCount 设置文件清理前最多保存的个数
		// rotatelogs.WithRotationCount(maxRemainCnt),
	)

	warn_writer, err := rotatelogs.New(
		folder+file_name+warn_ext+file_rorate,
		rotatelogs.WithLinkName(folder+file_name+warn_ext),
		rotatelogs.WithRotationTime(time.Hour),
		rotatelogs.WithMaxAge(time.Duration(file_save_day)*24*time.Hour),
	)

	error_writer, err := rotatelogs.New(
		folder+file_name+error_ext+file_rorate,
		rotatelogs.WithLinkName(folder+file_name+error_ext),
		rotatelogs.WithRotationTime(time.Hour),
		rotatelogs.WithMaxAge(time.Duration(file_save_day)*24*time.Hour),
	)

	fatal_writer, err := rotatelogs.New(
		folder+file_name+fatal_ext+file_rorate,
		rotatelogs.WithLinkName(folder+file_name+fatal_ext),
		rotatelogs.WithRotationTime(time.Hour),
		rotatelogs.WithMaxAge(time.Duration(file_save_day)*24*time.Hour),
	)

	if err != nil {
		log.Errorf("config local file system for logger error: %v", err)
	}

	lfsHook := lfshook.NewHook(lfshook.WriterMap{
		logrus.DebugLevel: info_writer,
		logrus.InfoLevel:  info_writer,
		logrus.WarnLevel:  warn_writer,
		logrus.ErrorLevel: error_writer,
		logrus.FatalLevel: fatal_writer,
		logrus.PanicLevel: fatal_writer,
	}, formatter)

	return lfsHook
}
