//提供一个分等级的日志系统，建议直接使用全局的对象，而不是另外New一个
package logger

import (
	l4g "github.com/alecthomas/log4go"
)

func init() {
	Log4 = l4g.NewLogger()

	Log4.AddFilter("stdout", l4g.DEBUG, l4g.NewConsoleLogWriter())

}

var Log4 l4g.Logger

func AddFileFilter(name string, filename string) error {
	Log4.AddFilter(name, l4g.FINE, l4g.NewFileLogWriter(filename, false))
	return nil
}
