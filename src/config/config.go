package config

import (
	"encoding/json"
	"errors"
	"i04Dns/txt"
	"i04Dns/util"
	"io/ioutil"
	"os"
	"sync"
)

type HttpServerConfig struct {
	Enabled bool
	Host    *string
	Network *string
	Port    uint16
	User    *string
	Pass    *string
}

type DnsServerConfig struct {
	Enabled bool
	Host    *string
	Network *string
}

type DnsServer struct {
	Name *string
	Ip   *string
}

type DnsRecord struct {
	Type  uint16
	Name  *string
	Ip    *string
	Cname *string
	Pref  uint16
	Mx    *string
}

type NameFilter struct {
	Filter  *string
	Name     *string
	Disabled bool
}

type ConfigurationFile struct {
	Http          *HttpServerConfig
	Dns           *DnsServerConfig
	DnsServers    *[]DnsServer
	LocalDatabase *[]DnsRecord
	Whitelist     *[]NameFilter
	Blacklist     *[]NameFilter
}

func (conf *ConfigurationFile) getDatabase() (*Database, error) {
	db := NewDatabase()
	count := len(*conf.LocalDatabase)
	for i := 0; i < count; i++ {
		r0, r1 := (*conf.LocalDatabase)[i].toDnsResource()
		if r0 == nil {
			return nil, errors.New(txt.ConfBadLocalDB)
		}
		db.Add(r0)
		if r1 != nil {
			db.Add(r1)
		}
	}
	return db, nil
}

func (conf *ConfigurationFile) getWhitelist() (*util.FilterList, error) {
	return toFilterList(conf.Whitelist, txt.ConfBadWhitelist)
}

func (conf *ConfigurationFile) getBlacklist() (*util.FilterList, error) {
	return toFilterList(conf.Blacklist, txt.ConfBadBlacklist)
}

func toFilterList(array *[]NameFilter, errorMsg string) (*util.FilterList, error) {
	size := len(*array)
	list := util.NewFilterList(size)
	for i := 0; i < size; i++ {
		nf := (*array)[i]
		if nf.Disabled {
			continue
		}
		if nf.Filter == nil || len(*nf.Filter) == 0 {
			return nil, errors.New(errorMsg)
		}
		list.Add(*nf.Filter)
	}
	list.Compile()
	return list, nil
}

var (
	httpConfig   *HttpServerConfig
	dnsConfig    *DnsServerConfig
	dnsDatabase  *Database
	dnsServers   *[]DnsServer
	dnsBlacklist *util.FilterList
	dnsWhitelist *util.FilterList
)

// ####################
// # config functions #
// ####################

var mutex sync.Mutex

func GetDB() *Database {
	mutex.Lock()
	defer mutex.Unlock()
	return dnsDatabase
}

func GetDnsConf() *DnsServerConfig {
	mutex.Lock()
	defer mutex.Unlock()
	return dnsConfig
}

func GetNameFilters() (*util.FilterList, *util.FilterList) {
	mutex.Lock()
	defer mutex.Unlock()
	return dnsWhitelist, dnsBlacklist
}

func SetConfig(config *ConfigurationFile) error {
	mutex.Lock()
	defer mutex.Unlock()

	if config.Http == nil {
		return errors.New(txt.ConfMissingHttp)
	}
	if config.Dns == nil {
		return errors.New(txt.ConfMissingDns)
	}
	if config.DnsServers == nil {
		return errors.New(txt.ConfMissingDnsServers)
	}
	if config.LocalDatabase == nil {
		return errors.New(txt.ConfMissingLocalDB)
	}
	if config.Whitelist == nil {
		return errors.New(txt.ConfMissingWhitelist)
	}
	if config.Blacklist == nil {
		return errors.New(txt.ConfMissingBlacklist)
	}
	if config.Http.Enabled {
		if !util.IsValidIP(config.Http.Host) {
			return errors.New(txt.ConfBadHttpIp)
		}
		if config.Http.Port == 0 {
			return errors.New(txt.ConfBadHttpPort)
		}
		if !util.IsValidTCPNetwork(*config.Http.Network) {
			return errors.New(txt.ConfBadHttpNetwork)
		}
	}
	if config.Dns.Enabled {
		if !util.IsValidIP(config.Dns.Host) {
			return errors.New(txt.ConfBadDnsIp)
		}
		if !util.IsValidUDPNetwork(*config.Dns.Network) {
			return errors.New(txt.ConfBadDnsNetwork)
		}
	}
	dnsServersCount := len(*config.DnsServers)
	for i := 0; i < dnsServersCount; i++ {
		if !util.IsValidIP((*config.DnsServers)[i].Ip) {
			return errors.New(txt.ConfBadDnsServers)
		}
	}
	var err error = nil
	var db *Database = nil
	var wl *util.FilterList = nil
	var bl *util.FilterList = nil
	db, err = config.getDatabase()
	if err != nil {
		return err
	}
	wl, err = config.getWhitelist()
	if err != nil {
		return err
	}
	bl, err = config.getBlacklist()
	if err != nil {
		return err
	}
	// set some shit
	dnsConfig = config.Dns
	httpConfig = config.Http
	dnsServers = config.DnsServers
	dnsDatabase = db
	dnsWhitelist = wl
	dnsBlacklist = bl
	return nil
}

func Load() {
	util.LogInfo(util.ConfigModule, txt.ConfLoad, []string{}, []string{})
	if doLoad() {
		util.LogInfo(util.ConfigModule, txt.ConfLoadOk, []string{}, []string{})
	} else {
		util.LogInfo(util.ConfigModule, txt.ConfLoadFail, []string{}, []string{})
	}
}

func doLoad() bool {
	file, err := os.Open("conf.json")
	if err != nil {
		// todo: log io error
		return false
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	config := ConfigurationFile{}

	bytes, ioErr := ioutil.ReadAll(file)
	if ioErr != nil {
		// todo: log io error
		return false
	}

	jsonErr := json.Unmarshal(bytes, &config)
	if jsonErr != nil {
		// todo: log json error
		return false
	}
	confErr := SetConfig(&config)
	if confErr != nil {
		util.LogError(util.ConfigModule, txt.ConfLoadFail, []string{txt.Error}, []string{confErr.Error()})
		return false
	}
	return true
}
