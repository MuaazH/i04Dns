package main

import (
	"i04Dns/config"
	"i04Dns/dns"
	"i04Dns/txt"
	"i04Dns/util"
)

const Version = "0.0.1"

func main() {
	util.LogInfo(util.MainModule, txt.AppName, []string{txt.Version}, []string{Version})
	config.Load()
	dns.Run()
}
