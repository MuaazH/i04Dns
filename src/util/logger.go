package util

import "fmt"

// todo: use some real log shit

func LogInfo(module string, msg string) {
	log(module, "INF", msg)
}

func LogError(module string, msg string) {
	log(module, "ERR", msg)
}

func LogDebug(module string, msg string) {
	log(module, "DBG", msg)
}

func log(module string, level string, msg string) {
	l := fmt.Sprintf("%s [%s] [%s] %s", TimeNowString(), module, level, msg)
	fmt.Println(l)
}
