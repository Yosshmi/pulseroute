package security

import (
 "context"
 "fmt"
 "net"
 "net/http"
 "net/netip"
 "net/url"
 "time"
)

func ValidateURL(raw string,allowPrivate bool)error{u,e:=url.Parse(raw);if e!=nil||u.Hostname()==""||u.User!=nil||u.Fragment!=""{return fmt.Errorf("invalid destination URL")};if u.Scheme!="https"&&!(allowPrivate&&u.Scheme=="http"){return fmt.Errorf("destination requires HTTPS")};if len(raw)>2048{return fmt.Errorf("destination URL too long")};return nil}
func PublicIP(ip net.IP)bool{a,ok:=netip.AddrFromSlice(ip);if !ok{return false};a=a.Unmap();if !a.IsGlobalUnicast()||a.IsPrivate()||a.IsLoopback()||a.IsLinkLocalUnicast(){return false};for _,s:=range []string{"100.64.0.0/10","192.0.0.0/24","192.0.2.0/24","198.18.0.0/15","198.51.100.0/24","203.0.113.0/24","240.0.0.0/4","2001:db8::/32","64:ff9b::/96"}{if netip.MustParsePrefix(s).Contains(a){return false}};return true}

// Resolve and validate at dial time, then dial the validated IP to prevent DNS rebinding.
func HTTPClient(timeout time.Duration,allowPrivate bool)*http.Client{
 dialer:=&net.Dialer{Timeout:timeout,KeepAlive:30*time.Second}
 tr:=&http.Transport{MaxIdleConns:128,MaxIdleConnsPerHost:16,IdleConnTimeout:90*time.Second,TLSHandshakeTimeout:5*time.Second,ResponseHeaderTimeout:timeout}
 tr.DialContext=func(ctx context.Context,network,address string)(net.Conn,error){host,port,e:=net.SplitHostPort(address);if e!=nil{return nil,e};ips,e:=net.DefaultResolver.LookupIP(ctx,"ip",host);if e!=nil{return nil,e};if len(ips)==0{return nil,fmt.Errorf("no destination addresses")};for _,ip:=range ips{if !allowPrivate&&!PublicIP(ip){return nil,fmt.Errorf("destination address blocked")}};return dialer.DialContext(ctx,network,net.JoinHostPort(ips[0].String(),port))}
 return &http.Client{Timeout:timeout,Transport:tr,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}
}
