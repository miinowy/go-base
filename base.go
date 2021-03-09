// Package base provide some base functions of GoLang
package base

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// init
func init() {
	inforTrigger()
	sigerTrigger()
	memorTrigger()

	conferTrigger()
	loggerTrigger()

	// Initialize by Reload function
	Reload()
}

var (
	// Time
	startAt  time.Time
	reloadAt time.Time

	// App information
	appName  string
	execDir  string
	execName string
	version  string
	workDir  string

	// Build information
	buildGo   string
	buildHash string
	buildTime string
	buildDir  string

	// Functions to execute when reload
	reloadFuncs sync.Map
	// Functions to execute when retire
	retireFuncs sync.Map

	// Avoid reload in parallel
	reloadMtx sync.Mutex
	// Avoid retire in parallel
	retireMtx sync.Mutex

	// Lite mode
	isLiteMode bool = true
)

// GetStartTime return start time of app
func GetStartTime() time.Time {
	return startAt
}

// GetReloadTime return reload time of app
func GetReloadTime() time.Time {
	return reloadAt
}

// GetAppName return name of app
func GetAppName() string {
	return appName
}

// GetExecDir return directory of execute file
func GetExecDir() string {
	return execDir
}

// GetExecName return file name of execute file
func GetExecName() string {
	return execName
}

// GetVersion return version of app
func GetVersion() string {
	return version
}

// GetWorkDir return working directory at staring
func GetWorkDir() string {
	return workDir
}

// GetBuildGo return version of go at building
func GetBuildGo() string {
	return buildGo
}

// GetBuildHash return Git HEAD hash of source code at building
func GetBuildHash() string {
	return buildHash
}

// GetBuildTime return time at building
func GetBuildTime() string {
	return buildTime
}

// GetBuildDir return directory at building
func GetBuildDir() string {
	return buildDir
}

// GetIP return outer IP of host
func GetIP() net.IP {
	conn, err := net.Dial("udp", "1.1.1.1:53")
	if err != nil {
		return nil
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP
}

// GetPath return the absolute path relative to the binary file by input
func GetPath(p ...string) string {
	var tree []string
	if base := viper.GetString("base_dir"); base != "" {
		tree = append(tree, base)
	} else {
		tree = append(tree, execDir)
	}
	if 0 < len(p) && !strings.HasPrefix(p[0], "/") {
		p = append(tree, p...)
	}
	return path.Join(p...)
}

// GetAssetsPath return the absolute path of assets
func GetAssetsPath(p ...string) string {
	p = append([]string{"../assets"}, p...)
	return GetPath(p...)
}

// IsLiteMode return whether it is currently in lite mode
func IsLiteMode() bool {
	return isLiteMode
}

// SetLiteMode set whether it is in lite mode
func SetLiteMode(mode bool) {
	isLiteMode = mode
	loggerInstance.adjustLogLevel()
}

// LiverRegister is used to register a liver hunter
func LiverRegister(key string, timeout time.Duration) {
	liverTrigger()
	liverInstance.Register(key, timeout)
}

// LiverCancel is used to cancel a liver hunter
func LiverCancel(key string) {
	liverTrigger()
	liverInstance.Cancel(key)
}

// ReloadRegister is used to register a function to be executed when reload
func ReloadRegister(function func() error, key string) {
	reloadFuncs.Store(key, function)
}

// ReloadCancel is used to cancel a function to be executed when reload
func ReloadCancel(key string) {
	reloadFuncs.Delete(key)
}

// RetireRegister is used to register a function to be executed when retire
func RetireRegister(function func() error, key string) {
	retireFuncs.Store(key, function)
}

// RetireCancel is used to cancel a function to be executed when retire
func RetireCancel(key string) {
	retireFuncs.Delete(key)
}

// Daemon will retire current process and start a daemon process
func Daemon() {
	Retire(0, true)
}

// Reload configure and logger, and functions in reloadFuncs
func Reload() {
	reloadMtx.Lock()
	defer reloadMtx.Unlock()

	// configer and logger
	conferTaskInstance.Fire()
	loggerTaskInstance.Fire()

	// reload functions
	groupRun(&reloadFuncs, 10*time.Second)

	reloadAt = time.Now()
}

// Retire will execute all functions in retireFuncs and exit by code
func Retire(code int, args ...bool) {
	retireMtx.Lock()
	defer retireMtx.Unlock()

	var daemon bool
	if 0 < len(args) {
		daemon = args[0]
	}

	// print call stack in log
	if code != 0 && daemon == false {
		logrus.WithFields(logrus.Fields{"exit_code": code}).
			Error("Ops... Somebody called Retire() with a none zero exit code, call stack:\n", string(debug.Stack()))
	}

	// retireFuncs
	groupRun(&retireFuncs, 10*time.Second)

	if daemon == true {
		logrus.Info("See you in daemon~")
		cmd := exec.Command(os.Args[0], os.Args[1:]...)
		cmd.Stderr = os.Stdout
		cmd.Stdout = os.Stderr
		cmd.Start()
	}
	logrus.Info("Bye~")
	os.Exit(code)
}

// groupRun execute all functions by goroutine
func groupRun(functions *sync.Map, timeout time.Duration) {
	var wg sync.WaitGroup
	var functionStatus sync.Map

	functions.Range(func(key, value interface{}) bool {
		wg.Add(1)
		fn := value.(func() error)
		functionStatus.Store(key, false)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				logrus.WithFields(logrus.Fields{"tip": value}).
					WithError(err).Warn("An error occured while group run")
			}
			functionStatus.Store(key, true)
		}()
		return true
	})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	go func() {
		wg.Wait()
		cancel()
	}()

	<-ctx.Done()
	if ctx.Err() != context.Canceled {
		functionStatus.Range(func(key, value interface{}) bool {
			logrus.WithFields(map[string]interface{}{"key": key, "status": value}).Error("function statuses")
			return true
		})
		logrus.WithError(ctx.Err()).Panic("Group run was hang")
	}
}
