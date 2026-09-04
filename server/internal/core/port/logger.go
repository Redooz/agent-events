package port

import "time"

type Field struct {
	Key   string
	Value any
}

func Str(key, value string) Field { return Field{Key: key, Value: value} }

func Int(key string, value int) Field { return Field{Key: key, Value: value} }

func Int64(key string, value int64) Field { return Field{Key: key, Value: value} }

func Bool(key string, value bool) Field { return Field{Key: key, Value: value} }

func Duration(key string, value time.Duration) Field { return Field{Key: key, Value: value} }

func Err(err error) Field { return Field{Key: "error", Value: err} }

func Any(key string, value any) Field { return Field{Key: key, Value: value} }

type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
}
