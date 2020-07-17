package main

import (
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

/*
Logger
通过调用zap.NewProduction()/zap.NewDevelopment()或者zap.Example()创建一个Logger。
上面的每一个函数都将创建一个logger。唯一的区别在于它将记录的信息不同。
例如production logger默认记录调用函数信息、日期和时间等。
通过Logger调用Info/Error等。
默认情况下日志都会打印到应用程序的console界面。
*/

var logger *zap.Logger

func mainDemo1() {
	InitLogger1()
	defer logger.Sync()
	simpleHttpGet1("www.sogou.com")
	simpleHttpGet1("http://www.sogou.com")
}

func InitLogger1() {
	logger, _ = zap.NewProduction()
}

func simpleHttpGet1(url string) {
	resp, err := http.Get(url)
	if err != nil {
		logger.Error(
			"Error fetching url..",
			zap.String("url", url),
			zap.Error(err))
	} else {
		logger.Info("Success..",
			zap.String("statusCode", resp.Status),
			zap.String("url", url))
		resp.Body.Close()
	}
}

/*
=============================================================
Sugared Logger
现在让我们使用Sugared Logger来实现相同的功能。

大部分的实现基本都相同。
惟一的区别是，我们通过调用主logger的. Sugar()方法来获取一个SugaredLogger。
然后使用SugaredLogger以printf格式记录语句
下面是修改过后使用SugaredLogger代替Logger的代码：
*/
var sugarLogger *zap.SugaredLogger

func mainDemo2() {
	InitLogger2()
	defer sugarLogger.Sync()
	simpleHttpGet2("www.google.com")
	simpleHttpGet2("http://www.google.com")
}

func InitLogger2() {
	//logger, _ := zap.NewProduction()//外部mian未使用logger，使用的是sugarLogger，所以可全局可局部logger变量
	logger, _ = zap.NewProduction()
	sugarLogger = logger.Sugar()
}
func simpleHttpGet2(url string) {
	sugarLogger.Debugf("Trying to hit GET request for %s", url)
	resp, err := http.Get(url)
	if err != nil {
		sugarLogger.Errorf("Error fetching URL %s : Error = %s", url, err)
	} else {
		sugarLogger.Infof("Success! statusCode = %s for URL %s", resp.Status, url)
		resp.Body.Close()
	}
}

/*
=============================================================
使用Lumberjack进行日志切割归档
*/
func mainDemo3() {
	InitLogger3()
	defer sugarLogger.Sync()

	for i := 0; i < 10000; i++ {
		sugarLogger.Info("test log")
	}
	simpleHttpGet2("www.sogou.com")
	simpleHttpGet2("http://www.sogou.com")
}

func InitLogger3() {
	writeSyncer := getLogWriter()
	encoder := getEncoder()
	core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)

	//logger := zap.New(core)
	/*
		接下来，我们将修改zap logger代码，添加将调用函数信息记录到日志中的功能。为此，我们将在zap.New(..)函数中添加一个Option。
	*/

	//logger := zap.New(core, zap.AddCaller())//外部main函数要使用全局logger，注意不能使用局部logger
	logger = zap.New(core, zap.AddCaller())
	sugarLogger = logger.Sugar()
}

func getEncoder() zapcore.Encoder {
	//return zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	/*
		将编码器从JSON Encoder更改为普通Encoder。为此，我们需要将NewJSONEncoder()更改为NewConsoleEncoder()。
		覆盖默认的ProductionConfig()，并进行以下更改:
		修改时间编码器
		在日志文件中使用大写字母记录日志级别
	*/
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

/*
func getLogWriter() zapcore.WriteSyncer {
	file, _ := os.OpenFile("./test.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0744)
	return zapcore.AddSync(file)
}
实际输出日志文件要进行切割，防止日志文件过大，改造如下
要在zap中加入Lumberjack支持，我们需要修改WriteSyncer代码。我们将按照下面的代码修改getLogWriter()函数：
*/
func getLogWriter() zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   "./test.log", //日志文件的位置
		MaxSize:    1,            //在进行切割之前，日志文件的最大大小（以MB为单位）
		MaxBackups: 5,            //保留旧文件的最大个数
		MaxAge:     30,           //保留旧文件的最大天数
		Compress:   false,        //是否压缩/归档旧文件
	}
	return zapcore.AddSync(lumberJackLogger)
}

//==================================================
//使用zap接收gin框架默认的日志并配置日志归档
func mainDemo4() {
	InitLogger3()
	//r := gin.Default()//不使用默认default中的logger
	r := gin.New()
	r.Use(GinLogger(logger), GinRecovery(logger, true))
	r.GET("/hello", func(c *gin.Context) {
		c.String(http.StatusOK, "hello!")
	})
	r.Run()
}

/*
基于zap的中间件，如果不想自己实现，可以使用github上有别人封装好的https://github.com/gin-contrib/zap。
我们可以模仿Logger()和Recovery()的实现，使用我们的日志库来接收gin框架默认输出的日志。
这里以zap为例，我们实现两个中间件如下：
*/
// GinLogger 接收gin框架默认的日志
func GinLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()

		cost := time.Since(start)
		logger.Info(path,
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			zap.Duration("cost", cost),
		)
	}
}

// GinRecovery recover掉项目可能出现的panic，并使用zap记录相关日志
func GinRecovery(logger *zap.Logger, stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				if brokenPipe {
					logger.Error(c.Request.URL.Path,
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
					// If the connection is dead, we can't write a status to it.
					c.Error(err.(error)) // nolint: errcheck
					c.Abort()
					return
				}

				if stack {
					logger.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
						zap.String("stack", string(debug.Stack())),
					)
				} else {
					logger.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

func main() {
	//mainDemo1()
	//mainDemo2()
	//mainDemo3()
	mainDemo4()
}
