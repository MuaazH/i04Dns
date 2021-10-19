package util

import "fmt"
import "time"

type Module = string

const (
	MainModule   Module = "SRV"
	ConfigModule Module = "CNF"
	DnsModule    Module = "DNS"
	WebModule    Module = "WEB"
)

type LogEntry struct {
	time       string
	module     Module
	msg        string
	msgType    int
	paramNames *[]string
	params     *[]string
}

var entries [256]*LogEntry
var index int32 = 0
var count int32 = 0

const errorLog = 1
const infoLog = 2
const debugLog = 3

var logDescription [4]string
var initialized = false

func initialize() {
	logDescription[errorLog] = "ERR"
	logDescription[infoLog] = "INF"
	logDescription[debugLog] = "DBG"
	initialized = true
}

func LogInfo(module Module, msg string, paramsNames []string, params []string) {
	log(module, msg, infoLog, &paramsNames, &params)
}

func LogError(module Module, msg string, paramsNames []string, params []string) {
	log(module, msg, errorLog, &paramsNames, &params)
}

func LogDebug(module Module, msg string, paramsNames []string, params []string) {
	log(module, msg, debugLog, &paramsNames, &params)
}

func log(module Module, msg string, typ int, paramNames *[]string, params *[]string) {
	if !initialized {
		initialize()
	}
	length := int32(len(entries))
	i := (index + count) % length
	if count < length {
		count++
	} else {
		index = (index + 1) % length
	}
	entries[i] = &LogEntry{timeNow(), module, msg, typ, paramNames, params}
	entries[i].Println()
}

func timeNow() string {
	t := time.Now()
	str := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	return str
}

func (log *LogEntry) String() string {
	return fmt.Sprintf("%s [%s] [%s] %s %v %v", log.time, log.module, logDescription[log.msgType], log.msg, *log.paramNames, *log.params)
}

func (log *LogEntry) Println() {
	fmt.Println(log.String())
}
