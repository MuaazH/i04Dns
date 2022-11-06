package config

import (
	"fmt"
	"i04Dns/dns/dnsmessage"
	"i04Dns/util"
)

func newResourceHeader(name string, typ dnsmessage.Type, ttl uint32) *dnsmessage.ResourceHeader {
	return &dnsmessage.ResourceHeader{
		Name:  dnsmessage.MustNewName(name + "."),
		Type:  typ,
		Class: dnsmessage.ClassINET,
		TTL:   ttl,
	}
}

func newIp4Resource(name string, ip string, ttl uint32) *dnsmessage.Resource {
	head := newResourceHeader(name, dnsmessage.TypeA, ttl)
	bytes := util.Ip4Bytes(ip)
	if bytes == nil {
		return nil
	}
	body := &dnsmessage.AResource{A: *bytes}
	return &dnsmessage.Resource{Header: *head, Body: body}
}

func newIp6Resource(name string, ip string, ttl uint32) *dnsmessage.Resource {
	head := newResourceHeader(name, dnsmessage.TypeAAAA, ttl)
	bytes := util.Ip6Bytes(ip)
	if bytes == nil {
		return nil
	}
	body := &dnsmessage.AAAAResource{AAAA: *bytes}
	return &dnsmessage.Resource{Header: *head, Body: body}
}

func newCnameResource(name string, cname string, ttl uint32) *dnsmessage.Resource {
	head := newResourceHeader(name, dnsmessage.TypeCNAME, ttl)
	body := &dnsmessage.CNAMEResource{CNAME: dnsmessage.MustNewName(cname + ".")}
	return &dnsmessage.Resource{Header: *head, Body: body}
}

func newMXResource(name string, pref uint16, mx string, ttl uint32) *dnsmessage.Resource {
	head := newResourceHeader(name, dnsmessage.TypeMX, ttl)
	body := &dnsmessage.MXResource{MX: dnsmessage.MustNewName(mx + "."), Pref: pref}
	return &dnsmessage.Resource{Header: *head, Body: body}
}

func reverse4(ip4 string) string {
	bytes := util.Ip4Bytes(ip4)
	return fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa", bytes[3], bytes[2], bytes[1], bytes[0])
}

func reverse6(ip6 string) string {
	const hex = "0123456789abcdef"
	bytes := util.Ip6Bytes(ip6)
	buf := [64]byte{}
	j := 0
	for i := 0; i < 64; i++ {
		buf[i] = hex[(bytes[j]&0xF0)>>4]
		i++
		buf[i] = '.'
		i++
		buf[i] = hex[bytes[j]&0x0F]
		i++
		buf[i] = '.'
		j++
	}
	return string(buf[:]) + "ip6.arpa"
}

func newPtrResource(rip string, name string, ttl uint32) *dnsmessage.Resource {
	head := newResourceHeader(rip, dnsmessage.TypePTR, ttl)
	body := &dnsmessage.PTRResource{PTR: dnsmessage.MustNewName(name + ".")}
	return &dnsmessage.Resource{Header: *head, Body: body}
}

func (rec *DnsRecord) toDnsResource() (*dnsmessage.Resource, *dnsmessage.Resource) {
	defer func() {
		_ = recover()
	}()
	const defaultTTL uint32 = 30
	if rec != nil && util.IsValidHostName(rec.Name) {
		switch dnsmessage.Type(rec.Type) {
		case dnsmessage.TypeA:
			if util.IsValidIP4(rec.Ip) {
				return newIp4Resource(*rec.Name, *rec.Ip, defaultTTL), newPtrResource(reverse4(*rec.Ip), *rec.Name, defaultTTL)
			}
		case dnsmessage.TypeAAAA:
			if util.IsValidIP6(rec.Ip) {
				return newIp6Resource(*rec.Name, *rec.Ip, defaultTTL), newPtrResource(reverse6(*rec.Ip), *rec.Name, defaultTTL)
			}
		case dnsmessage.TypeCNAME:
			if util.IsValidHostName(rec.Cname) {
				return newCnameResource(*rec.Name, *rec.Cname, defaultTTL), nil
			}
		case dnsmessage.TypeMX:
			if util.IsValidHostName(rec.Mx) {
				return newMXResource(*rec.Name, rec.Pref, *rec.Mx, defaultTTL), nil
			}
		}
	}
	return nil, nil
}
