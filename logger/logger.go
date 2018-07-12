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

func AddConsoleFilterError() error {
	Log4.AddFilter("stdout", l4g.ERROR, l4g.NewConsoleLogWriter())
	return nil
}
func AddConsoleFilterWarn() error {
	Log4.AddFilter("stdout", l4g.WARNING, l4g.NewConsoleLogWriter())
	return nil
}
func AddConsoleFilterInfo() error {
	Log4.AddFilter("stdout", l4g.INFO, l4g.NewConsoleLogWriter())
	return nil
}

func AddFileFilter(name string, filename string) error {
	Log4.AddFilter(name, l4g.FINE, l4g.NewFileLogWriter(filename, false))
	return nil
}

func AddFileFilterError(name string, filename string) error {
	Log4.AddFilter(name, l4g.ERROR, l4g.NewFileLogWriter(filename, false))
	return nil
}
func AddFileFilterWarn(name string, filename string) error {
	Log4.AddFilter(name, l4g.WARNING, l4g.NewFileLogWriter(filename, false))
	return nil
}
func AddFileFilterInfo(name string, filename string) error {
	Log4.AddFilter(name, l4g.INFO, l4g.NewFileLogWriter(filename, false))
	return nil
}
