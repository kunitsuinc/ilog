package ilog

import (
	"fmt"
	"io"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type implLoggerConfig struct {
	levelKey        string
	level           Level
	timestampKey    string
	timestampFormat string
	timestampZone   *time.Location
	callerKey       string
	callerSkip      int
	useLongCaller   bool
	messageKey      string
	separator       string
	writer          io.Writer
}

type implLogger struct {
	config implLoggerConfig
	fields []byte
}

func NewBuilder(level Level, w io.Writer) implLoggerConfig { //nolint:revive
	return implLoggerConfig{
		levelKey:        "severity",
		level:           level,
		timestampKey:    "timestamp",
		timestampFormat: time.RFC3339Nano,
		timestampZone:   time.Local, //nolint:gosmopolitan
		callerKey:       "caller",
		callerSkip:      4,
		useLongCaller:   false,
		messageKey:      "message",
		separator:       "\n",
		writer:          w,
	}
}

func (c implLoggerConfig) SetLevelKey(key string) implLoggerConfig { //nolint:revive
	c.levelKey = key
	return c
}

func (c implLoggerConfig) SetTimestampKey(key string) implLoggerConfig { //nolint:revive
	c.timestampKey = key
	return c
}

func (c implLoggerConfig) SetTimestampFormat(format string) implLoggerConfig { //nolint:revive
	c.timestampFormat = format
	return c
}

func (c implLoggerConfig) SetTimestampZone(zone *time.Location) implLoggerConfig { //nolint:revive
	c.timestampZone = zone
	return c
}

func (c implLoggerConfig) SetCallerKey(key string) implLoggerConfig { //nolint:revive
	c.callerKey = key
	return c
}

func (c implLoggerConfig) UseShortCaller(useShortCaller bool) implLoggerConfig { //nolint:revive
	c.useLongCaller = !useShortCaller
	return c
}

func (c implLoggerConfig) SetMessageKey(key string) implLoggerConfig { //nolint:revive
	c.messageKey = key
	return c
}

func (c implLoggerConfig) SetSeparator(separator string) implLoggerConfig { //nolint:revive
	c.separator = separator
	return c
}

func (c implLoggerConfig) Build() Logger {
	return &implLogger{
		config: c,
		fields: make([]byte, 0, 1024),
	}
}

func (l *implLogger) Level() Level {
	return l.config.level
}

func (l *implLogger) SetLevel(level Level) Logger {
	l.config.level = level
	return l
}

func (l *implLogger) AddCallerSkip(skip int) Logger {
	l.config.callerSkip += skip
	return l
}

func (l *implLogger) Copy() Logger {
	copied := *l
	copied.fields = make([]byte, len(l.fields))
	copy(copied.fields, l.fields)
	return &copied
}

func (l *implLogger) Any(key string, value interface{}) LogEntry {
	return l.new().Any(key, value)
}

func (l *implLogger) Bool(key string, value bool) LogEntry {
	return l.new().Bool(key, value)
}

func (l *implLogger) Bytes(key string, value []byte) LogEntry {
	return l.new().Bytes(key, value)
}

func (l *implLogger) Duration(key string, value time.Duration) LogEntry {
	return l.new().Duration(key, value)
}

func (l *implLogger) Err(err error) LogEntry {
	return l.new().Err(err)
}

func (l *implLogger) ErrWithKey(key string, err error) LogEntry {
	return l.new().ErrWithKey(key, err)
}

func (l *implLogger) Float32(key string, value float32) LogEntry {
	return l.new().Float32(key, value)
}

func (l *implLogger) Float64(key string, value float64) LogEntry {
	return l.new().Float64(key, value)
}

func (l *implLogger) Int(key string, value int) LogEntry {
	return l.new().Int(key, value)
}

func (l *implLogger) Int32(key string, value int32) LogEntry {
	return l.new().Int32(key, value)
}

func (l *implLogger) Int64(key string, value int64) LogEntry {
	return l.new().Int64(key, value)
}

func (l *implLogger) String(key, value string) LogEntry {
	return l.new().String(key, value)
}

func (l *implLogger) Time(key string, value time.Time) LogEntry {
	return l.new().Time(key, value)
}

func (l *implLogger) Uint(key string, value uint) LogEntry {
	return l.new().Uint(key, value)
}

func (l *implLogger) Uint32(key string, value uint32) LogEntry {
	return l.new().Uint32(key, value)
}

func (l *implLogger) Uint64(key string, value uint64) LogEntry {
	return l.new().Uint64(key, value)
}

func (l *implLogger) Debugf(format string, args ...interface{}) {
	_ = l.new().logf(DebugLevel, format, args...)
}

func (l *implLogger) Infof(format string, args ...interface{}) {
	_ = l.new().logf(InfoLevel, format, args...)
}

func (l *implLogger) Warnf(format string, args ...interface{}) {
	_ = l.new().logf(WarnLevel, format, args...)
}

func (l *implLogger) Errorf(format string, args ...interface{}) {
	_ = l.new().logf(ErrorLevel, format, args...)
}

func (l *implLogger) Logf(level Level, format string, args ...interface{}) {
	_ = l.new().logf(level, format, args...)
}

func (l *implLogger) Write(p []byte) (int, error) {
	if err := l.new().logf(l.config.level, string(p)); err != nil {
		return 0, fmt.Errorf("w.logf: %w", err)
	}
	return len(p), nil
}

func (l *implLogger) new() *implLogEntry {
	buffer, put := getBytesBuffer()
	return &implLogEntry{
		logger:      l,
		bytesBuffer: buffer,
		put:         put,
	}
}

//nolint:errname
type implLogEntry struct {
	logger      *implLogger
	bytesBuffer *bytesBuffer
	put         func()
}

func (*implLogEntry) Error() string {
	return ErrLogEntryIsNotWritten.Error()
}

func (e *implLogEntry) null(key string) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, null...)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, ',')
	return e
}

//nolint:cyclop,funlen
func (e *implLogEntry) Any(key string, value interface{}) LogEntry {
	switch v := value.(type) {
	case bool:
		return e.Bool(key, v)
	case *bool:
		if v != nil {
			return e.Bool(key, *v)
		}
		return e.null(key)
	case byte:
		return e.String(key, string(v))
	case []byte:
		return e.Bytes(key, v)
	case time.Duration:
		return e.Duration(key, v)
	case error:
		return e.ErrWithKey(key, v)
	case float32:
		return e.Float32(key, v)
	case float64:
		return e.Float64(key, v)
	case int:
		return e.Int(key, v)
	case int8:
		return e.Int(key, int(v))
	case int16:
		return e.Int(key, int(v))
	case int32:
		return e.Int32(key, v)
	case int64:
		return e.Int64(key, v)
	case string:
		return e.String(key, v)
	case time.Time:
		return e.Time(key, v)
	case uint:
		return e.Uint(key, v)
	// NOTE: uint8 == byte
	// case uint8:
	// 	return w.Uint(key, uint(v))
	case uint16:
		return e.Uint(key, uint(v))
	case uint32:
		return e.Uint32(key, v)
	case uint64:
		return e.Uint64(key, v)
	case fmt.Formatter:
		return e.String(key, fmt.Sprintf("%+v", v))
	case fmt.Stringer:
		// NOTE: Even if v is nil, it is not judged as nil because it has type information. Calling v.String() causes panic.
		// if v != nil {
		// 	return w.String(key, v.String())
		// } else {
		// 	return w.null(key)
		// }
		return e.String(key, fmt.Sprintf("%s", v)) //nolint:gosimple
	default:
		return e.String(key, fmt.Sprintf("%v", v))
	}
}

func (e *implLogEntry) Bool(key string, value bool) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = strconv.AppendBool(e.bytesBuffer.bytes, value)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, ',')
	return e
}

func (e *implLogEntry) Bytes(key string, value []byte) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, '"')
	e.bytesBuffer.bytes = appendJSONEscapedString(e.bytesBuffer.bytes, string(value))
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, '"', ',')
	return e
}

func (e *implLogEntry) Duration(key string, value time.Duration) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, '"')
	e.bytesBuffer.bytes = appendJSONEscapedString(e.bytesBuffer.bytes, value.String())
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, '"', ',')
	return e
}

func (e *implLogEntry) Err(err error) LogEntry {
	return e.ErrWithKey("error", err)
}

func (e *implLogEntry) ErrWithKey(key string, err error) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	// NOTE: Even if err is your unique error type and nil, it is not judged as nil because it has type information. Calling err.Error() causes panic.
	// if err != nil {
	formatter, ok := err.(fmt.Formatter) //nolint:errorlint
	if ok && formatter != nil {
		e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, '"')
		e.bytesBuffer.bytes = appendJSONEscapedString(e.bytesBuffer.bytes, fmt.Sprintf("%+v", formatter))
		e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, '"')
	} else {
		e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, '"')
		e.bytesBuffer.bytes = appendJSONEscapedString(e.bytesBuffer.bytes, err.Error())
		e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, '"')
	}
	// } else {
	// 	w.bytesBuffer.bytes = append(w.bytesBuffer.bytes, null...)
	// }
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, ',')
	return e
}

func (e *implLogEntry) Float32(key string, value float32) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = appendFloatFieldValue(e.bytesBuffer.bytes, float64(value), 32)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, ',')
	return e
}

func (e *implLogEntry) Float64(key string, value float64) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = appendFloatFieldValue(e.bytesBuffer.bytes, value, 64)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, ',')
	return e
}

func (e *implLogEntry) Int(key string, value int) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = strconv.AppendInt(e.bytesBuffer.bytes, int64(value), 10)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, ',')
	return e
}

func (e *implLogEntry) Int32(key string, value int32) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = strconv.AppendInt(e.bytesBuffer.bytes, int64(value), 10)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, ',')
	return e
}

func (e *implLogEntry) Int64(key string, value int64) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = strconv.AppendInt(e.bytesBuffer.bytes, value, 10)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, ',')
	return e
}

func (e *implLogEntry) String(key string, value string) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, '"')
	e.bytesBuffer.bytes = appendJSONEscapedString(e.bytesBuffer.bytes, value)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, '"', ',')
	return e
}

func (e *implLogEntry) Time(key string, value time.Time) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, '"')
	e.bytesBuffer.bytes = appendJSONEscapedString(e.bytesBuffer.bytes, value.Format(e.logger.config.timestampFormat))
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, '"', ',')
	return e
}

func (e *implLogEntry) Uint(key string, value uint) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = strconv.AppendUint(e.bytesBuffer.bytes, uint64(value), 10)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, ',')
	return e
}

func (e *implLogEntry) Uint32(key string, value uint32) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = strconv.AppendUint(e.bytesBuffer.bytes, uint64(value), 10)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, ',')
	return e
}

func (e *implLogEntry) Uint64(key string, value uint64) LogEntry {
	e.bytesBuffer.bytes = appendKey(e.bytesBuffer.bytes, key)
	e.bytesBuffer.bytes = strconv.AppendUint(e.bytesBuffer.bytes, value, 10)
	e.bytesBuffer.bytes = append(e.bytesBuffer.bytes, ',')
	return e
}

func (e *implLogEntry) Logger() Logger {
	l := *e.logger
	l.fields = make([]byte, len(e.bytesBuffer.bytes))
	copy(l.fields, e.bytesBuffer.bytes)
	return &l
}

func (e *implLogEntry) Debugf(format string, args ...interface{}) {
	_ = e.logf(DebugLevel, format, args...)
}

func (e *implLogEntry) Infof(format string, args ...interface{}) {
	_ = e.logf(InfoLevel, format, args...)
}

func (e *implLogEntry) Warnf(format string, args ...interface{}) {
	_ = e.logf(WarnLevel, format, args...)
}

func (e *implLogEntry) Errorf(format string, args ...interface{}) {
	_ = e.logf(ErrorLevel, format, args...)
}

func (e *implLogEntry) Logf(level Level, format string, args ...interface{}) {
	_ = e.logf(level, format, args...)
}

func (e *implLogEntry) Write(p []byte) (int, error) {
	if err := e.logf(e.logger.config.level, string(p)); err != nil {
		return 0, fmt.Errorf("w.logf: %w", err)
	}
	return len(p), nil
}

//nolint:cyclop
func (e *implLogEntry) logf(level Level, format string, args ...interface{}) error {
	defer e.put()
	if level < e.logger.config.level {
		return nil
	}

	b, put := getBytesBuffer()
	defer put()

	b.bytes = append(b.bytes, '{')

	if len(e.logger.config.levelKey) > 0 {
		b.bytes = appendKey(b.bytes, e.logger.config.levelKey)
		b.bytes = appendLevelField(b.bytes, level)
		b.bytes = append(b.bytes, ',')
	}
	if len(e.logger.config.timestampKey) > 0 {
		b.bytes = appendKey(b.bytes, e.logger.config.timestampKey)
		b.bytes = append(b.bytes, '"')
		b.bytes = appendJSONEscapedString(b.bytes, time.Now().In(e.logger.config.timestampZone).Format(e.logger.config.timestampFormat))
		b.bytes = append(b.bytes, '"', ',')
	}
	if len(e.logger.config.callerKey) > 0 {
		b.bytes = appendKey(b.bytes, e.logger.config.callerKey)
		b.bytes = append(b.bytes, '"')
		b.bytes = appendCaller(b.bytes, e.logger.config.callerSkip, e.logger.config.useLongCaller)
		b.bytes = append(b.bytes, '"', ',')
	}
	if len(e.logger.config.messageKey) > 0 {
		b.bytes = appendKey(b.bytes, e.logger.config.messageKey)
		b.bytes = append(b.bytes, '"')
		if len(args) > 0 {
			b.bytes = appendJSONEscapedString(b.bytes, fmt.Sprintf(format, args...))
		} else {
			b.bytes = appendJSONEscapedString(b.bytes, format)
		}
		b.bytes = append(b.bytes, '"', ',')
	}

	if len(e.logger.fields) > 0 {
		b.bytes = append(b.bytes, e.logger.fields...)
	}

	if len(e.bytesBuffer.bytes) > 0 {
		b.bytes = append(b.bytes, e.bytesBuffer.bytes...)
	}

	if b.bytes[len(b.bytes)-1] == ',' {
		b.bytes[len(b.bytes)-1] = '}'
	} else {
		b.bytes = append(b.bytes, '}')
	}

	if _, err := e.logger.config.writer.Write(append(b.bytes, e.logger.config.separator...)); err != nil {
		err = fmt.Errorf("w.logger.writer.Write: p=%s: %w", b.bytes, err)
		defer Global().Errorf(err.Error())
		return err
	}

	return nil
}

type (
	bytesBuffer struct {
		bytes []byte
	}
	pcBuffer struct {
		pc []uintptr
	}
)

// nolint: gochecknoglobals
var (
	_bufferPool   = &sync.Pool{New: func() interface{} { return &bytesBuffer{make([]byte, 0, 1024)} }}
	_pcBufferPool = &sync.Pool{New: func() interface{} { return &pcBuffer{make([]uintptr, 64)} }} // NOTE: both len and cap are needed.
)

func getBytesBuffer() (buf *bytesBuffer, put func()) {
	b := _bufferPool.Get().(*bytesBuffer) //nolint:forcetypeassert
	b.bytes = b.bytes[:0]
	return b, func() {
		_bufferPool.Put(b)
	}
}

func getPCBuffer() (buf *pcBuffer, put func()) {
	b := _pcBufferPool.Get().(*pcBuffer) //nolint:forcetypeassert
	return b, func() {
		_pcBufferPool.Put(b)
	}
}

const null = "null"

// nolint: cyclop
// appendJSONEscapedString.
func appendJSONEscapedString(dst []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		if s[i] != '"' && s[i] != '\\' && s[i] > 0x1F {
			dst = append(dst, s[i])

			continue
		}

		// cf. https://tools.ietf.org/html/rfc8259#section-7
		// ... MUST be escaped: quotation mark, reverse solidus, and the control characters (U+0000 through U+001F).
		switch s[i] {
		case '"', '\\':
			dst = append(dst, '\\', s[i])
		case '\b' /* 0x08 */ :
			dst = append(dst, '\\', 'b')
		case '\f' /* 0x0C */ :
			dst = append(dst, '\\', 'f')
		case '\n' /* 0x0A */ :
			dst = append(dst, '\\', 'n')
		case '\r' /* 0x0D */ :
			dst = append(dst, '\\', 'r')
		case '\t' /* 0x09 */ :
			dst = append(dst, '\\', 't')
		default:
			const hexTable string = "0123456789abcdef"
			// cf. https://github.com/golang/go/blob/70deaa33ebd91944484526ab368fa19c499ff29f/src/encoding/hex/hex.go#L28-L29
			dst = append(dst, '\\', 'u', '0', '0', hexTable[s[i]>>4], hexTable[s[i]&0x0f])
		}
	}

	return dst
}

func appendFloatFieldValue(dst []byte, value float64, bitSize int) []byte {
	switch {
	case math.IsNaN(value):
		return append(dst, `"NaN"`...)
	case math.IsInf(value, 1):
		return append(dst, `"+Inf"`...)
	case math.IsInf(value, -1):
		return append(dst, `"-Inf"`...)
	}

	return strconv.AppendFloat(dst, value, 'f', -1, bitSize)
}

func appendCaller(dst []byte, callerSkip int, useLongCaller bool) []byte {
	pc, put := getPCBuffer()
	defer put()

	var frame runtime.Frame
	if runtime.Callers(callerSkip, pc.pc) > 0 {
		frame, _ = runtime.CallersFrames(pc.pc).Next()
	}

	return appendCallerFromFrame(dst, frame, useLongCaller)
}

// appendCallerFromFrame was split off from appendCaller in order to test different behaviors depending on the contents of the `runtime.Frame`.
func appendCallerFromFrame(dst []byte, frame runtime.Frame, useLongCaller bool) []byte {
	if useLongCaller {
		dst = appendJSONEscapedString(dst, frame.File)
	} else {
		dst = appendJSONEscapedString(dst, extractShortPath(frame.File))
	}

	dst = append(dst, ':')
	dst = strconv.AppendInt(dst, int64(frame.Line), 10)

	return dst
}

func extractShortPath(path string) string {
	// path == /path/to/directory/file
	//                           ~ <- idx
	idx := strings.LastIndexByte(path, '/')
	if idx == -1 {
		return path
	}

	// path[:idx] == /path/to/directory
	//                       ~ <- idx
	idx = strings.LastIndexByte(path[:idx], '/')
	if idx == -1 {
		return path
	}

	// path == /path/to/directory/file
	//                  ~~~~~~~~~~~~~~ <- filepath[idx+1:]
	return path[idx+1:]
}

func appendKey(dst []byte, key string) []byte {
	dst = append(dst, '"')
	dst = appendJSONEscapedString(dst, key)
	dst = append(dst, '"', ':')

	return dst
}

func appendLevelField(dst []byte, level Level) []byte {
	switch level { //nolint:exhaustive
	case InfoLevel:
		return append(dst, `"INFO"`...)
	case WarnLevel:
		return append(dst, `"WARNING"`...)
	case ErrorLevel:
		return append(dst, `"ERROR"`...)
	default:
		return append(dst, `"DEBUG"`...)
	}
}