package logger

import (
	"engines/github.com.blackjack.syslog"
	"fmt"
)

type Logger struct {
	Logprefix string
}

type LoggerInterface interface {
	Log(syslog.Priority, string)
	Logf(syslog.Priority, string, ...interface{})
	Emerg(string)
	Emergf(string, ...interface{})
	Alert(string)
	Alertf(string, ...interface{})
	Crit(string)
	Critf(string, ...interface{})
	Err(string)
	Errf(string, ...interface{})
	Warning(string)
	Warningf(string, ...interface{})
	Notice(string)
	Noticef(string, ...interface{})
	Info(string)
	Infof(string, ...interface{})
	Debug(string)
	Debugf(string, ...interface{})
}

func (l *Logger) Log(priority syslog.Priority, msg string) {
	syslog.Syslog(priority, l.Logprefix+" "+msg)
}
func (l *Logger) Logf(priority syslog.Priority, format string, a ...interface{}) {
	l.Log(priority, fmt.Sprintf(format, a...))
}
func (l *Logger) Emerg(msg string) {
	l.Log(syslog.LOG_EMERG, msg)
}
func (l *Logger) Emergf(format string, a ...interface{}) {
	l.Logf(syslog.LOG_EMERG, format, a...)
}
func (l *Logger) Alert(msg string) {
	l.Log(syslog.LOG_ALERT, msg)
}
func (l *Logger) Alertf(format string, a ...interface{}) {
	l.Logf(syslog.LOG_ALERT, format, a...)
}
func (l *Logger) Crit(msg string) {
	l.Log(syslog.LOG_CRIT, msg)
}
func (l *Logger) Critf(format string, a ...interface{}) {
	l.Logf(syslog.LOG_CRIT, format, a...)
}
func (l *Logger) Err(msg string) {
	l.Log(syslog.LOG_ERR, msg)
}
func (l *Logger) Errf(format string, a ...interface{}) {
	l.Logf(syslog.LOG_ERR, format, a...)
}
func (l *Logger) Warning(msg string) {
	l.Log(syslog.LOG_WARNING, msg)
}
func (l *Logger) Warningf(format string, a ...interface{}) {
	l.Logf(syslog.LOG_WARNING, format, a...)
}
func (l *Logger) Notice(msg string) {
	l.Log(syslog.LOG_NOTICE, msg)
}
func (l *Logger) Noticef(format string, a ...interface{}) {
	l.Logf(syslog.LOG_NOTICE, format, a...)
}
func (l *Logger) Info(msg string) {
	l.Log(syslog.LOG_INFO, msg)
}
func (l *Logger) Infof(format string, a ...interface{}) {
	l.Logf(syslog.LOG_INFO, format, a...)
}
func (l *Logger) Debug(msg string) {
	l.Log(syslog.LOG_DEBUG, msg)
}
func (l *Logger) Debugf(format string, a ...interface{}) {
	l.Logf(syslog.LOG_DEBUG, format, a...)
}
