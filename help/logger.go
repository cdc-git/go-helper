package help

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gorm.io/gorm/logger"
)

type LoggerConfig struct {
	SlowThreshold time.Duration
	LogLevel      logger.LogLevel
	logger        *log.Logger
	Url           string
}

func NewLogger(config LoggerConfig) *LoggerConfig {
	return &LoggerConfig{
		SlowThreshold: config.SlowThreshold,
		LogLevel:      config.LogLevel,
		logger:        log.New(os.Stdout, "\r\n", log.LstdFlags),
		Url:           config.Url, // You can customize the output here.
	}
}

func (l *LoggerConfig) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *LoggerConfig) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		l.logger.Printf(msg, data...)

		l._log("info", fmt.Sprintf(msg, data...))
	}
}

func (l *LoggerConfig) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		l.logger.Printf(msg, data...)

		l._log("error", fmt.Sprintf(msg, data...))
	}
}

func (l *LoggerConfig) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		l.logger.Printf(msg, data...)

		l._log("error", fmt.Sprintf(msg, data...))

	}
}

func (l *LoggerConfig) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel > 0 {
		
		var (
			tipe    string
			message string
		)

		elapsed := time.Since(begin)
		switch {
		case err != nil && l.LogLevel >= logger.Error:
			sql, rows := fc()
			l.logger.Printf("[%.3fms] [rows:%v] %s error: %v", float64(elapsed.Nanoseconds())/1e6, rows, sql, err)

			tipe = "error"
			message = fmt.Sprintf("[%.3fms] [rows:%v] %s error: %v", float64(elapsed.Nanoseconds())/1e6, rows, sql, err)
			
		case l.SlowThreshold != 0 && elapsed > l.SlowThreshold && l.LogLevel >= logger.Warn:
			sql, rows := fc()
			l.logger.Printf("[%.3fms] [rows:%v] SLOW SQL >= %v\n%s", float64(elapsed.Nanoseconds())/1e6, rows, l.SlowThreshold, sql)

			tipe = "info"
			message = fmt.Sprintf("[%.3fms] [rows:%v] SLOW SQL >= %v\n%s", float64(elapsed.Nanoseconds())/1e6, rows, l.SlowThreshold, sql)
		case l.LogLevel >= logger.Info:
			sql, rows := fc()
			l.logger.Printf("[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/1e6, rows, sql)

			tipe = "debug"
			message = fmt.Sprintf("[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}

		l._log(tipe, message)
	}
}

// recover log after panic
func (l *LoggerConfig) ErrorLog() {
	message := recover()

	if message != nil {
		log.Println(message)
		l._log("Fatal", message.(string))
	} else {
		fmt.Println("---- No Error have a nice day  ----")
	}
}

func (l *LoggerConfig) _log(tipe string, message string) {

	defer l.ErrorLog()

	if l.Url != "" {
		
		data, err := os.ReadFile("go.mod")
		if err != nil {
			fmt.Println("Error reading go.mod:", err)
			return
		}
	
		lines := strings.Split(string(data), "\n")
	
		jsonData, err := json.Marshal(map[string]interface{}{
			"type":    tipe,
			"message": message,
			"module":  strings.Fields(lines[0])[1],
		})
	
		if err != nil {
			panic(err)
		}
	
		// Create a new HTTP POST request.
		req, err := http.NewRequest("POST", l.Url, bytes.NewBuffer(jsonData))
		if err != nil {
			message := fmt.Sprintf("Error creating request: %s", err)
			panic(message)
		}
	
		req.Header.Set("Content-Type", "application/json")
	
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			message := fmt.Sprintf("Error sending request: %s", err)
			panic(message)
		}
	
		defer resp.Body.Close()
	
		log.Println("Response Status:", resp.Status)
	}
}
