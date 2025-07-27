package adapters

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/myfedi/gargoyle/domain/ports"
)

type loggerFn func(string, ...interface{})

type StdLoggerAdapterConfig struct {
	ShowStacktraces bool
}

// check if StdLoggerAdapter implements the LoggerPort interface
var _ ports.LoggerPort = (*StdLoggerAdapter)(nil)

type StdLoggerAdapter struct {
	showStacktraces bool
}

func NewStdLoggerAdapter(cfg StdLoggerAdapterConfig) *StdLoggerAdapter {
	return &StdLoggerAdapter{
		showStacktraces: cfg.ShowStacktraces,
	}
}
func (l *StdLoggerAdapter) log(out *os.File, level string, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if l.showStacktraces && (level == "ERROR" || level == "FATAL") {
		msg += "\n" + string(debug.Stack())
	}
	fmt.Fprintf(out, "[%s] %s\n", level, msg)
	if level == "FATAL" {
		os.Exit(1)
	}
}

func (l *StdLoggerAdapter) Debugf(format string, args ...interface{}) {
	l.log(os.Stdout, "DEBUG", format, args...)
}

func (l *StdLoggerAdapter) Infof(format string, args ...interface{}) {
	l.log(os.Stdout, "INFO", format, args...)
}

func (l *StdLoggerAdapter) Warnf(format string, args ...interface{}) {
	l.log(os.Stderr, "WARN", format, args...)
}

func (l *StdLoggerAdapter) Errorf(format string, args ...interface{}) {
	l.log(os.Stderr, "ERROR", format, args...)
}
