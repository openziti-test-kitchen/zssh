/*
	Copyright NetFoundry, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package zsshlib

import (
	"fmt"
	"github.com/mgutz/ansi"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetFormatter(&logrusFormatter{})
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