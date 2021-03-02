package base

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	inforTaskOnce sync.Once

	sigerTaskOnce sync.Once

	memorInstance *memor
	memorTaskOnce sync.Once

	conferTaskOnce     sync.Once
	conferTaskInstance *Task

	loggerInstance     *logger
	loggerTaskOnce     sync.Once
	loggerTaskInstance *Task

	liverInstance *liver
	liverTaskOnce sync.Once
)

func inforTrigger() {
	inforTaskOnce.Do(func() {
		NewTaskOneTime(&infor{}, "info")
	})
}

func sigerTrigger() {
	sigerTaskOnce.Do(func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR1, syscall.SIGUSR2)
		NewTaskOnChannel(&siger{sig: sig}, "signal", sig)
	})
}

func memorTrigger() {
	memorTaskOnce.Do(func() {
		go func() {
			<-time.After(5 * time.Second)
			memorInstance = &memor{}
			NewTaskOnInterval(memorInstance, "memor", 20*time.Second)
		}()
	})
}

func conferTrigger() {
	conferTaskOnce.Do(func() {
		conferTaskInstance, _ = NewTaskManual(&confer{}, "config")
	})
}

func loggerTrigger() {
	loggerTaskOnce.Do(func() {
		loggerInstance = &logger{}
		loggerTaskInstance, _ = NewTaskManual(loggerInstance, "logger")
	})
}

func liverTrigger() {
	liverTaskOnce.Do(func() {
		liverInstance = &liver{}
		NewTaskOnInterval(liverInstance, "liver", 10*time.Second, false)
	})
}

// infor is used to initialize information of app
type infor struct {
	*TaskBase
}

func (i *infor) Reload(ctx context.Context) error {
	return nil
}

func (i *infor) Retire(ctx context.Context) error {
	return nil
}

func (i *infor) Schedule(ctx context.Context) error {
	startAt = time.Now()

	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	execName, execDir = filepath.Base(executable), filepath.Dir(executable)

	workDir, err = os.Getwd()
	if err != nil {
		panic(err)
	}
	appName = execName

	return nil
}

// siger is used to watch signal
type siger struct {
	*TaskBase

	sig chan os.Signal
}

func (s *siger) Reload(ctx context.Context) error {
	return nil
}

func (s *siger) Retire(ctx context.Context) error {
	signal.Stop(s.sig)
	close(s.sig)
	return nil
}

func (s *siger) Schedule(ctx context.Context) error {
	switch s.Argument.(os.Signal) {
	case syscall.SIGTERM, syscall.SIGINT:
		// Retire after SIGTERM or SIGINT
		s.Log.WithFields(map[string]interface{}{"content": s}).
			Debug("It's time to say goodbye")
		go Retire(0)
	case syscall.SIGUSR1:
		// Reload after SIGUSR1
		s.Log.WithFields(map[string]interface{}{"content": s}).
			Debug("Got a reload signal")
		go Reload()
	}
	return nil
}

// memor is used to prevent memory leak
type memor struct {
	*TaskBase

	limit uint64
}

func (l *memor) Reload(ctx context.Context) error {
	l.limit = viper.GetUint64("memory.limit")

	if l.limit == 0 {
		l.limit = 1024 * 1024 * 1024
	}
	if l.limit < 16*1024*1024 {
		l.limit = 16 * 1024 * 1024
	}

	return nil
}

func (l *memor) Retire(ctx context.Context) error {
	return nil
}

func (l *memor) Schedule(ctx context.Context) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	l.Log.WithFields(map[string]interface{}{"alloc": m.Alloc, "gc": m.NumGC}).
		Trace("Memory checking")

	if l.limit < m.Alloc {
		l.Log.WithFields(map[string]interface{}{"alloc": m.Alloc, "gc": m.NumGC}).
			Warn("Memory over limit")

		Daemon()
	}

	return nil
}

// confer is used for initialize config
type confer struct {
	*TaskBase

	read bool
	mtx  sync.Mutex
}

func (c *confer) Reload(ctx context.Context) error {
	return nil
}

func (c *confer) Retire(ctx context.Context) error {
	return nil
}

func (c *confer) Schedule(ctx context.Context) error {
	if c.read == false {
		return c.init()
	}
	return viper.ReadInConfig()
}

func (c *confer) init() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.read == true {
		return viper.ReadInConfig()
	}
	c.read = true

	if strings.HasSuffix(GetExecName(), "test") {
		viper.SetConfigName("test")
	} else {
		viper.SetConfigName(GetExecName())
	}
	viper.SetConfigType("")

	// Seach directory tree
	for _, base := range []string{GetExecDir(), GetWorkDir()} {
		for ; ; base = path.Dir(base) {
			viper.AddConfigPath(base)
			viper.AddConfigPath(filepath.Join(base, "etc"))
			viper.AddConfigPath(filepath.Join(base, "conf"))
			viper.AddConfigPath(filepath.Join(base, "config"))
			viper.AddConfigPath(filepath.Join(base, "configs"))
			if base == path.Dir(base) {
				break
			}
		}
	}

	// Watch config change
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		c.Log.Debug("Config file has changed, auto reload")
		Reload()
	})

	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}
	return nil
}

// logger is used to initialize logger
type logger struct {
	*TaskBase
	lumberjack.Logger

	loglevel logrus.Level

	set bool
	mtx sync.Mutex
}

func (l *logger) Reload(ctx context.Context) error {
	return nil
}

func (l *logger) Retire(ctx context.Context) error {
	return nil
}

func (l *logger) Schedule(ctx context.Context) error {
	if l.set == false {
		l.init()
	} else {
		l.adjustOutPut()
		l.adjustLogLevel()
	}
	return nil
}

func (l *logger) init() {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	if l.set == true {
		l.adjustOutPut()
		l.adjustLogLevel()
	}
	l.set = true

	// log format
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	})

	// caller
	logrus.SetReportCaller(true)

	// Register fatal handler
	logrus.RegisterExitHandler(func() { Retire(1) })

	// Hooks
	logrus.AddHook(l)
	logrus.AddHook(lfshook.NewHook(l, &logrus.TextFormatter{
		TimestampFormat: time.RFC3339,
		FullTimestamp:   true,
	}))

	// Output to Stderr
	if fi, err := os.Stdout.Stat(); err != nil || (fi.Mode()&os.ModeCharDevice == 0) {
		logrus.SetOutput(ioutil.Discard)
	} else {
		logrus.SetOutput(os.Stderr)
	}
}

func (l *logger) adjustLogLevel() {
	if level, err := logrus.ParseLevel(viper.GetString("log.level")); err != nil {
		l.loglevel = logrus.InfoLevel
	} else {
		l.loglevel = level
	}

	if strings.HasSuffix(GetExecName(), "test") == false && IsLiteMode() == true && logrus.InfoLevel < l.loglevel {
		logrus.SetLevel(logrus.InfoLevel)
	} else {
		logrus.SetLevel(l.loglevel)
	}
}

func (l *logger) adjustOutPut() {
	l.Filename = GetPath(viper.GetString("log.dir"), fmt.Sprintf("%s.log", GetAppName()))
	l.MaxAge = viper.GetInt("log.maxage")
	l.MaxSize = viper.GetInt("log.maxsize")
	l.MaxBackups = viper.GetInt("log.maxbackups")
	l.Compress = viper.GetBool("log.compress")
}

// Logrus Hook / Levels
func (l *logger) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Logrus Hook / Fire
func (l *logger) Fire(entry *logrus.Entry) error {
	// Make caller simpler by cut string
	if entry.Caller != nil {
		if src, ok := entry.Data["src"]; ok == true {
			entry.Data["caller"] = src
		} else {
			entry.Data["caller"] = fmt.Sprintf(
				"%v:%v",
				entry.Caller.File[strings.Index(entry.Caller.File, GetBuildDir())+len(GetBuildDir())+1:],
				entry.Caller.Line,
			)
		}
		entry.Caller = nil
	}

	// Make values clearer by `printf("%+v")`
	for key, value := range entry.Data {
		entry.Data[key] = fmt.Sprintf("%+v", value)
	}

	return nil
}

// live indicates a live instance
type live struct {
	last    time.Time
	timeout time.Duration
}

// liver is used for liveness check
type liver struct {
	*TaskBase

	path string
	data sync.Map
}

func (l *liver) Reload(ctx context.Context) error {
	logdir := viper.GetString("log.dir")
	l.path = GetPath(logdir, fmt.Sprintf("%s.alive", GetAppName()))
	return nil
}

func (l *liver) Retire(ctx context.Context) error {
	return nil
}

func (l *liver) Schedule(ctx context.Context) error {
	l.Log.Trace("Liver is working")

	now := time.Now()
	l.data.Range(func(key, value interface{}) bool {
		lc := value.(*live)
		if lc != nil && lc.timeout < now.Sub(lc.last) {
			go l.Log.WithFields(map[string]interface{}{"key": key, "content": lc}).
				Fatal("A live context was died")
			return false
		}
		return true
	})
	l.aliveTouch(ctx)

	return nil
}

func (l *liver) Register(key string, timeout time.Duration) {
	l.data.Store(key, &live{
		last:    time.Now(),
		timeout: timeout,
	})
	l.Log.WithFields(map[string]interface{}{"key": key}).Debug("Liver register a new key")
}

func (l *liver) Cancel(key string) {
	_, ok := l.data.LoadAndDelete(key)
	if ok == true {
		l.Log.WithFields(map[string]interface{}{"key": key}).Debug("Liver cancel a key")
	} else {
		l.Log.WithFields(map[string]interface{}{"key": key}).Warn("Liver cancel a key that does not exist")
	}
}

// Touch alive file
func (l *liver) aliveTouch(ctx context.Context) {
	_, err := os.Stat(l.path)
	if os.IsNotExist(err) {
		file, err := os.Create(l.path)
		if err != nil {
			l.Log.WithError(err).Error("Liver can't create alive file")
		}
		file.Close()
	} else {
		currentTime := time.Now().Local()
		os.Chtimes(l.path, currentTime, currentTime)
	}
}
