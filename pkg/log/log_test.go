package log_test

import (
	"os"

	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"stackerbuild.io/stacker/pkg/log"
)

func TestLog(t *testing.T) {
	Convey("With timestamps", t, func() {
		handler := log.NewTextHandler(os.Stderr, true)
		So(handler, ShouldNotBeNil)

		So(func() { log.Debugf("debug msg") }, ShouldNotPanic)
		So(func() { log.Infof("info msg") }, ShouldNotPanic)
		So(func() { log.Errorf("error msg") }, ShouldNotPanic)
	})

	Convey("Without timestamps", t, func() {
		handler := log.NewTextHandler(os.Stderr, false)
		So(handler, ShouldNotBeNil)

		So(func() { log.Debugf("debug msg") }, ShouldNotPanic)
		So(func() { log.Infof("info msg") }, ShouldNotPanic)
		So(func() { log.Errorf("error msg") }, ShouldNotPanic)
	})
}
