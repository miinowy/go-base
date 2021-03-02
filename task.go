package base

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type taskType uint

const (
	taskTypeOnetime taskType = 1 << iota
	taskTypeOnTCP
	taskTypeOnReload
	taskTypeOnChannel
	taskTypeOnInterval
	taskTypeOnFsChange
	taskTypeManual
)

// Tasker Interface
type Tasker interface {
	// Schedule will execute when trigger fire
	Schedule(context.Context) error

	// Reload will execute when start or reload
	Reload(context.Context) error
	// Retire will execute when destruct
	Retire(context.Context) error
}

// TaskBase must be wrapped in struct which implement Tasker interface by unnamed pointer
type TaskBase struct {
	taskType taskType

	id   string
	Name string

	Log *logrus.Entry

	// Trigger's type is according task type
	//   taskOnTcp:      net.Listener
	//   taskOnChannel:  Channel
	//   taskOnInterval: time.Duration
	//   taskOnFsChange: *fsnotify.Watcher
	Trigger interface{}

	// Argument's value is difference according task type
	//   taskOnTcp:      net.Conn that accept from net.Listener
	//   taskOnChannel:  value that out from channel
	//   taskOnInterval: disable liver hunter if value is false
	//   taskOnFsChange: event that from *fsnotify.Watcher last time
	Argument interface{}

	// Start immediately if this flag is true
	Immediately bool
	// TaskOnInterval will sleep if this flag is true
	Sleep bool
}

// newTaskBase initialize *TaskBase
func newTaskBase(w *TaskBase, args ...interface{}) *TaskBase {
	for _, arg := range args {
		switch arg.(type) {
		case bool:
			if w.taskType == taskTypeOnInterval {
				w.Argument = arg.(bool)
			} else {
				w.Immediately = arg.(bool)
			}
		case string:
			w.Name = arg.(string)
		case time.Duration:
			w.Trigger = arg.(time.Duration)
		case *logrus.Entry:
			w.Log = arg.(*logrus.Entry)
		default:
			val := reflect.ValueOf(arg)
			if val.Kind() == reflect.Chan {
				w.Trigger = val.Interface()
			}
		}
	}

	if w.id == "" {
		w.id = uuid.New().String()
	}
	if w.Name == "" {
		w.Name = fmt.Sprintf("anonymous/%s", w.id)
	}
	if w.Log == nil {
		w.Log = logrus.WithFields(logrus.Fields{"context": w.Name, "id": w.id})
	}

	return w
}

// Task context
type Task struct {
	Tasker

	id string
	tb *TaskBase

	onceTb    sync.Once
	onceStop  sync.Once
	onceStart sync.Once
	mtxReload sync.Mutex

	// life context indicates the whole life cycle
	life context.Context
	die  context.CancelFunc

	// nap context is used for trigger
	nap  context.Context
	fire context.CancelFunc

	// retire context is used for destruct
	retire  context.Context
	retired context.CancelFunc
}

// newTask initialize *Task
func newTask(tasker Tasker, taskBase *TaskBase) (*Task, error) {
	t := &Task{Tasker: tasker}
	tb := t.getTaskBase()
	*tb = *taskBase
	t.id = fmt.Sprintf("%v/%v", tb.Name, tb.id)
	t.life, t.die = context.WithCancel(context.Background())
	tb.Log.Debug("Get a new task")

	if err := t.Reload(); err != nil {
		tb.Log.WithError(err).Fatal("Failed to reload the new task")
	}

	return t, t.Start()
}

// NewTaskOneTime return taskTypeOneTime
func NewTaskOneTime(task Tasker, args ...interface{}) (*Task, error) {
	return newTask(task, newTaskBase(&TaskBase{taskType: taskTypeOnetime}, args...))
}

// NewTaskOnTCP return taskTypeOnTCP
func NewTaskOnTCP(task Tasker, listen string, args ...interface{}) (*Task, error) {
	return newTask(task, newTaskBase(&TaskBase{taskType: taskTypeOnTCP, Argument: listen}, args...))
}

// NewTaskOnReload return taskTypeOnReload
func NewTaskOnReload(task Tasker, args ...interface{}) (*Task, error) {
	return newTask(task, newTaskBase(&TaskBase{taskType: taskTypeOnReload, Immediately: true}, args...))
}

// NewTaskOnChannel return taskTypeOnChannel
func NewTaskOnChannel(task Tasker, args ...interface{}) (*Task, error) {
	return newTask(task, newTaskBase(&TaskBase{taskType: taskTypeOnChannel}, args...))
}

// NewTaskOnInterval return taskTypeOnInterval
func NewTaskOnInterval(task Tasker, args ...interface{}) (*Task, error) {
	return newTask(task, newTaskBase(&TaskBase{taskType: taskTypeOnInterval, Argument: true, Immediately: true}, args...))
}

// NewTaskOnFsChange return taskTypeOnFsChange
func NewTaskOnFsChange(task Tasker, path string, args ...interface{}) (*Task, error) {
	return newTask(task, newTaskBase(&TaskBase{taskType: taskTypeOnFsChange, Argument: path}, args...))
}

// NewTaskManual return taskTypeManual
func NewTaskManual(task Tasker, args ...interface{}) (*Task, error) {
	return newTask(task, newTaskBase(&TaskBase{taskType: taskTypeManual}, args...))
}

// Get TaskBase in Tasker
func (t *Task) getTaskBase() (tb *TaskBase) {
	t.onceTb.Do(func() {
		val := reflect.ValueOf(t.Tasker)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		val = val.FieldByName("TaskBase")

		if val.Kind() == reflect.Ptr {
			if val.IsNil() == true {
				val.Set(reflect.ValueOf(&TaskBase{}))
			}
			// No check type
			t.tb = val.Interface().(*TaskBase)
		}
	})
	return t.tb
}

// Died return true if task has done
func (t *Task) Died() bool {
	select {
	case <-t.life.Done():
		return true
	default:
		return false
	}
}

// Fire will trigger task immediately
func (t *Task) Fire() error {
	if t.Died() {
		return fmt.Errorf("Task to fire has already died")
	}
	tb := t.getTaskBase()
	if tb.taskType == taskTypeManual {
		return t.Tasker.Schedule(t.life)
	} else if t.fire != nil {
		t.fire()
	}
	return nil
}

// Reload is used to reload task
func (t *Task) Reload() (err error) {
	t.mtxReload.Lock()
	defer t.mtxReload.Unlock()

	tb := t.getTaskBase()
	tb.Log.WithFields(logrus.Fields{"task": t, "tasker": t.Tasker, "taskBase": tb}).
		Debug("Task is reloading")

	if err = t.Tasker.Reload(t.life); err != nil {
		return err
	}
	if t.fire != nil {
		t.fire()
	}

	tb.Log.WithFields(logrus.Fields{"task": t, "tasker": t.Tasker, "taskBase": tb}).
		Debug("Task reloaded")
	return nil
}

// Stop the Task
func (t *Task) Stop() (err error) {
	t.onceStop.Do(func() {
		err = t.stop()
	})
	return err
}
func (t *Task) stop() (err error) {
	tb := t.getTaskBase()
	tb.Log.Debug("Task is stopping")

	// Cancel functions
	RetireCancel(t.id)
	ReloadCancel(t.id)

	// Cancel life context
	t.retire, t.retired = context.WithTimeout(context.Background(), 3*time.Second)
	t.die()

	// Clean Trigger
	switch tb.taskType {
	case taskTypeOnTCP:
		defer tb.Trigger.(net.Listener).Close()
	case taskTypeOnInterval:
		if tb.Argument.(bool) == true {
			defer LiverCancel(t.id)
		}
	case taskTypeOnFsChange:
		defer tb.Trigger.(*fsnotify.Watcher).Close()
	}

	<-t.retire.Done()
	if err = t.retire.Err(); err == context.Canceled {
		err = nil
	} else if err != nil {
		err = fmt.Errorf("Stop task timeout, task id: %v, reason: %v", t.id, err)
	}

	return err
}

// Start the Task
func (t *Task) Start() (err error) {
	t.onceStart.Do(func() {
		err = t.start()
	})
	return err
}
func (t *Task) start() (err error) {
	tb := t.getTaskBase()

	// check died
	if t.Died() {
		tb.Log.Error("Task to start has already died")
		return fmt.Errorf("This task has already died")
	}

	tb.Log.Debug("Task is starting")

	// initialize
	switch tb.taskType {
	case taskTypeManual:
		if tb.Immediately == true {
			return t.Tasker.Schedule(t.life)
		}
		return nil
	case taskTypeOnetime:
		defer t.die()
		return t.Tasker.Schedule(t.life)
	case taskTypeOnTCP:
		if tb.Trigger, err = net.Listen("tcp", tb.Argument.(string)); err != nil {
			defer t.die()
			return err
		}
	case taskTypeOnFsChange:
		if tb.Trigger, err = fsnotify.NewWatcher(); err != nil {
			defer t.die()
			return err
		}
		if err = tb.Trigger.(*fsnotify.Watcher).Add(tb.Argument.(string)); err != nil {
			defer t.die()
			defer tb.Trigger.(*fsnotify.Watcher).Close()
			return err
		}
	}

	// run
	if tb.Immediately == true && tb.Sleep == false {
		tb.Log.Trace("Task fire")
		if err = t.Tasker.Schedule(t.life); err == context.Canceled {
			err = nil
		}
	}
	go t.routine()

	// register functions
	RetireRegister(t.Stop, t.id)
	ReloadRegister(t.Reload, t.id)

	return err
}

// Goroutine
func (t *Task) routine() {
	var ok bool
	var err error
	var val reflect.Value
	var cancel context.CancelFunc
	var tb = t.getTaskBase()
	tb.Log.Trace("Task's goroutine started successfully")

	for {
		switch tb.taskType {
		case taskTypeOnInterval:
			if tb.Sleep == true {
				t.nap, t.fire = context.WithCancel(context.Background())
				LiverCancel(t.id)
			} else {
				t.nap, t.fire = context.WithTimeout(context.Background(), tb.Trigger.(time.Duration))
				if tb.Argument.(bool) == true {
					LiverRegister(t.id, tb.Trigger.(time.Duration)*4)
				}
			}
		case taskTypeOnReload:
			t.nap, t.fire = context.WithCancel(context.Background())
		case taskTypeManual:
			t.nap, t.fire = context.WithCancel(context.Background())
		case taskTypeOnTCP:
			t.nap, cancel = context.WithCancel(context.Background())
			go func() {
				// Stop Task will close Listener, Accept will return an error and Died() equals true, goroutine won't leak
				defer cancel()
				for {
					if tb.Argument, err = tb.Trigger.(net.Listener).Accept(); err == nil || t.Died() == true {
						return
					}
					tb.Log.WithError(err).Error("Accept TCP listener error")
				}
			}()
		case taskTypeOnChannel:
			t.nap, cancel = context.WithCancel(context.Background())
			go func() {
				// Must close channel in Tasker.Retire(), else maybe blocked here, goroutine leak
				if val, ok = reflect.ValueOf(tb.Trigger).Recv(); ok == false {
					t.die()
					return
				}
				defer cancel()
				tb.Argument = val.Interface()
			}()
		case taskTypeOnFsChange:
			t.nap, cancel = context.WithCancel(context.Background())
			go func() {
				// Stop Task will done life context done, goroutine won't leak
				defer cancel()
				watcher := tb.Trigger.(*fsnotify.Watcher)
				for {
					select {
					case tb.Argument, ok = <-watcher.Events:
						if ok == true {
							return
						}
					case _, _ = <-watcher.Errors:
					case <-t.life.Done():
					}
				}
			}()
		}
		select {
		case <-t.nap.Done():
		case <-t.life.Done():
			t.Tasker.Retire(context.TODO())
			tb.Log.Debug("Task's goroutine is stopping")
			t.retired()
			return
		}

		tb.Log.Trace("Task fire")
		if tb.taskType == taskTypeOnInterval {
			if tb.Sleep == false {
				ctx, cancel := context.WithTimeout(t.life, tb.Trigger.(time.Duration))
				defer cancel()
				t.Tasker.Schedule(ctx)
			}
		} else {
			t.Tasker.Schedule(t.life)
		}
	}
}
