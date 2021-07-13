package zsshlib

import (
	"fmt"
	"github.com/mgutz/ansi"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetFormatter(&logrusFormatter{})
}
type Billly struct {

}
type logrusFormatter struct {
}

func (fa *logrusFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	level := toLevel(entry)
	return []byte(fmt.Sprintf("%s\t%s\n", level, entry.Message)), nil
}

func toLevel(entry *logrus.Entry) string {
	switch entry.Level {
	case logrus.PanicLevel:
		return panicColor
	case logrus.FatalLevel:
		return fatalColor
	case logrus.ErrorLevel:
		return errorColor
	case logrus.WarnLevel:
		return warnColor
	case logrus.InfoLevel:
		return infoColor
	case logrus.DebugLevel:
		return debugColor
	case logrus.TraceLevel:
		return traceColor
	default:
		return infoColor
	}
}

var panicColor = ansi.Red + "PANIC" + ansi.DefaultFG
var fatalColor = ansi.Red + "FATAL" + ansi.DefaultFG
var errorColor = ansi.Red + "ERROR" + ansi.DefaultFG
var warnColor = ansi.Yellow + "WARN " + ansi.DefaultFG
var infoColor = ansi.LightGreen + "INFO " + ansi.DefaultFG
var debugColor = ansi.LightBlue + "DEBUG" + ansi.DefaultFG
var traceColor = ansi.LightBlack + "TRACE" + ansi.DefaultFG