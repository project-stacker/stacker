package log

import (
	"fmt"
	"io"
	"time"

	"github.com/apex/log"
)

var thisIsAStackerLog struct{}

func addStackerLogSentinel(e *log.Entry) *log.Entry {
	return e.WithField("isStacker", &thisIsAStackerLog)
}

func isStackerLog(e *log.Entry) bool {
	v, ok := e.Fields["isStacker"]
	return ok && v == &thisIsAStackerLog
}

type stackerLogFilterer struct {
	underlying log.Handler
}

func (h stackerLogFilterer) HandleLog(e *log.Entry) error {
	if !isStackerLog(e) {
		return nil
	}

	delete(e.Fields, "isStacker")

	return h.underlying.HandleLog(e)
}

func FilterNonStackerLogs(handler log.Handler, level log.Level) {
	log.SetHandler(stackerLogFilterer{handler})
	log.SetLevel(level)
}

func Debugf(msg string, v ...interface{}) {
	addStackerLogSentinel(log.NewEntry(log.Log.(*log.Logger))).Debugf(msg, v...)
}

func Infof(msg string, v ...interface{}) {
	addStackerLogSentinel(log.NewEntry(log.Log.(*log.Logger))).Infof(msg, v...)
}

func Errorf(msg string, v ...interface{}) {
	addStackerLogSentinel(log.NewEntry(log.Log.(*log.Logger))).Errorf(msg, v...)
}

func Fatalf(msg string, v ...interface{}) {
	addStackerLogSentinel(log.NewEntry(log.Log.(*log.Logger))).Fatalf(msg, v...)
}

type TextHandler struct {
	out       io.StringWriter
	timestamp bool
}

func NewTextHandler(out io.StringWriter, timestamp bool) log.Handler {
	return &TextHandler{out, timestamp}
}

func (th *TextHandler) HandleLog(e *log.Entry) error {
	if th.timestamp {
		_, err := th.out.WriteString(fmt.Sprintf("%s ", e.Timestamp.Format(time.RFC3339)))
		if err != nil {
			return err
		}
	}

	_, err := th.out.WriteString(fmt.Sprintf(e.Message))
	if err != nil {
		return err
	}

	for _, name := range e.Fields.Names() {
		_, err = th.out.WriteString(fmt.Sprintf(" %s=%s", name, e.Fields.Get(name)))
		if err != nil {
			return err
		}
	}

	_, err = th.out.WriteString("\n")
	if err != nil {
		return err
	}

	return nil
}
