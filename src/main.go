package main

import (
	"fmt"
	"i04Dns/common"
	"i04Dns/dns"
	"i04Dns/http"
	"i04Dns/util"
)

const Version = "1.4.0"
const ModuleName = "Main"

func main() {
	util.LogInfo(ModuleName, fmt.Sprintf("i04Dns version %s", Version))
	appCtx := common.NewAppContext(http.New(), dns.New())
	err := appCtx.LoadConfig()
	if err != nil {
		util.LogError(ModuleName, err.Error())
		return
	}
	go appCtx.RunHttpServer()
	appCtx.RunDnsServer()
}
