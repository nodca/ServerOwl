package logging

import (
  "io"
  "os"
  "time"

  "github.com/rs/zerolog"
)

var (
  // Global logger instance
  globalLogger *Logger
)

// Logger wraps zerolog with additional features
type Logger struct {
  zl        zerolog.Logger
  sanitizer *Sanitizer
}

// Config holds logger configuration
type Config struct {
  Level      string `yaml:"level"`       // debug, info, warn, error
  Format     string `yaml:"format"`      // json, console
  Output     string `yaml:"output"`      // stdout, stderr, file path
  TimeFormat string `yaml:"time_format"` // RFC3339, Unix, etc.
  Sanitize   bool   `yaml:"sanitize"`    // Enable sensitive data masking
}

// New creates a new logger with the given configuration
func New(cfg *Config) *Logger {
  var output io.Writer = os.Stdout

  if cfg.Output == "stderr" {
    output = os.Stderr
  } else if cfg.Output != "" && cfg.Output != "stdout" {
    file, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err == nil {
      output = file
    }
  }

  // Set time format
  if cfg.TimeFormat == "" {
    cfg.TimeFormat = time.RFC3339
  }
  zerolog.TimeFieldFormat = cfg.TimeFormat

  // Create base logger
  var zl zerolog.Logger
  if cfg.Format == "console" {
    zl = zerolog.New(zerolog.ConsoleWriter{Out: output, TimeFormat: cfg.TimeFormat}).
      With().Timestamp().Logger()
  } else {
    zl = zerolog.New(output).With().Timestamp().Logger()
  }

  // Set level
  level, err := zerolog.ParseLevel(cfg.Level)
  if err != nil {
    level = zerolog.InfoLevel
  }
  zl = zl.Level(level)

  logger := &Logger{
    zl: zl,
  }

  if cfg.Sanitize {
    logger.sanitizer = NewSanitizer()
  }

  return logger
}

// Init initializes the global logger
func Init(cfg *Config) {
  globalLogger = New(cfg)
}

// Default returns the global logger, creating one if needed
func Default() *Logger {
  if globalLogger == nil {
    globalLogger = New(&Config{
      Level:    "info",
      Format:   "json",
      Output:   "stdout",
      Sanitize: true,
    })
  }
  return globalLogger
}

// With creates a child logger with additional fields
func (l *Logger) With() *LogContext {
  return &LogContext{
    ctx:       l.zl.With(),
    sanitizer: l.sanitizer,
  }
}

// Debug logs at debug level
func (l *Logger) Debug() *LogEvent {
  return &LogEvent{event: l.zl.Debug(), sanitizer: l.sanitizer}
}

// Info logs at info level
func (l *Logger) Info() *LogEvent {
  return &LogEvent{event: l.zl.Info(), sanitizer: l.sanitizer}
}

// Warn logs at warn level
func (l *Logger) Warn() *LogEvent {
  return &LogEvent{event: l.zl.Warn(), sanitizer: l.sanitizer}
}

// Error logs at error level
func (l *Logger) Error() *LogEvent {
  return &LogEvent{event: l.zl.Error(), sanitizer: l.sanitizer}
}

// Fatal logs at fatal level and exits
func (l *Logger) Fatal() *LogEvent {
  return &LogEvent{event: l.zl.Fatal(), sanitizer: l.sanitizer}
}

// LogContext wraps zerolog.Context
type LogContext struct {
  ctx       zerolog.Context
  sanitizer *Sanitizer
}

// Str adds a string field
func (c *LogContext) Str(key, val string) *LogContext {
  if c.sanitizer != nil {
    val = c.sanitizer.SanitizeValue(key, val)
  }
  c.ctx = c.ctx.Str(key, val)
  return c
}

// Int adds an int field
func (c *LogContext) Int(key string, val int) *LogContext {
  c.ctx = c.ctx.Int(key, val)
  return c
}

// Int64 adds an int64 field
func (c *LogContext) Int64(key string, val int64) *LogContext {
  c.ctx = c.ctx.Int64(key, val)
  return c
}

// Bool adds a bool field
func (c *LogContext) Bool(key string, val bool) *LogContext {
  c.ctx = c.ctx.Bool(key, val)
  return c
}

// Err adds an error field
func (c *LogContext) Err(err error) *LogContext {
  c.ctx = c.ctx.Err(err)
  return c
}

// Logger returns the logger with the context applied
func (c *LogContext) Logger() *Logger {
  return &Logger{
    zl:        c.ctx.Logger(),
    sanitizer: c.sanitizer,
  }
}

// LogEvent wraps zerolog.Event
type LogEvent struct {
  event     *zerolog.Event
  sanitizer *Sanitizer
}

// Str adds a string field
func (e *LogEvent) Str(key, val string) *LogEvent {
  if e.sanitizer != nil {
    val = e.sanitizer.SanitizeValue(key, val)
  }
  e.event = e.event.Str(key, val)
  return e
}

// Int adds an int field
func (e *LogEvent) Int(key string, val int) *LogEvent {
  e.event = e.event.Int(key, val)
  return e
}

// Int64 adds an int64 field
func (e *LogEvent) Int64(key string, val int64) *LogEvent {
  e.event = e.event.Int64(key, val)
  return e
}

// Bool adds a bool field
func (e *LogEvent) Bool(key string, val bool) *LogEvent {
  e.event = e.event.Bool(key, val)
  return e
}

// Err adds an error field
func (e *LogEvent) Err(err error) *LogEvent {
  e.event = e.event.Err(err)
  return e
}

// Dur adds a duration field
func (e *LogEvent) Dur(key string, d time.Duration) *LogEvent {
  e.event = e.event.Dur(key, d)
  return e
}

// Interface adds an interface field
func (e *LogEvent) Interface(key string, val any) *LogEvent {
  e.event = e.event.Interface(key, val)
  return e
}

// Msg sends the log event with a message
func (e *LogEvent) Msg(msg string) {
  e.event.Msg(msg)
}

// Msgf sends the log event with a formatted message
func (e *LogEvent) Msgf(format string, args ...any) {
  e.event.Msgf(format, args...)
}

// Send sends the log event without a message
func (e *LogEvent) Send() {
  e.event.Send()
}
