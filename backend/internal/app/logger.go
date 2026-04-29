package app

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

type LogFields map[string]any

func Logger(baseFields ...LogFields) *ServiceLogger {
	logger := &ServiceLogger{}
	for _, fields := range baseFields {
		logger = logger.With(fields)
	}
	return logger
}

type ServiceLogger struct {
	fields LogFields
}

func (l *ServiceLogger) With(fields LogFields) *ServiceLogger {
	merged := make(LogFields, len(l.fields)+len(fields))
	for key, value := range l.fields {
		merged[key] = value
	}
	for key, value := range fields {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" || value == nil {
			continue
		}
		merged[trimmedKey] = value
	}
	return &ServiceLogger{fields: merged}
}

func (l *ServiceLogger) Info(message string, fields ...LogFields) {
	l.log("INFO", message, fields...)
}

func (l *ServiceLogger) Error(message string, fields ...LogFields) {
	l.log("ERROR", message, fields...)
}

func (l *ServiceLogger) log(level, message string, fields ...LogFields) {
	payload := make(map[string]any, len(l.fields)+len(fields)+4)
	payload["level"] = strings.TrimSpace(level)
	payload["message"] = strings.TrimSpace(message)
	payload["service"] = strings.TrimSpace(os.Getenv("SERVICE_NAME"))
	for key, value := range l.fields {
		payload[key] = value
	}
	for _, fieldSet := range fields {
		for key, value := range fieldSet {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" || value == nil {
				continue
			}
			payload[trimmedKey] = value
		}
	}
	if strings.TrimSpace(payload["service"].(string)) == "" {
		delete(payload, "service")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		log.Printf("level=%s message=%q logger_error=%q", level, message, err.Error())
		return
	}
	log.Print(string(encoded))
}
