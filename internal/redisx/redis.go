package redisx

import("context";"encoding/json";"time";"github.com/redis/go-redis/v9")

func Open(raw string)(*redis.Client,error){o,e:=redis.ParseURL(raw);if e!=nil{return nil,e};o.DialTimeout=2*time.Second;o.ReadTimeout=2*time.Second;o.WriteTimeout=2*time.Second;o.MaxRetries=0;return redis.NewClient(o),nil}

// Redis TIME keeps all API/worker replicas on one rate-limit clock.
var bucket=redis.NewScript(`
local t=redis.call('TIME')
local now=tonumber(t[1])+tonumber(t[2])/1000000
local rate=tonumber(ARGV[1])
local capacity=tonumber(ARGV[2])
local state=redis.call('HMGET',KEYS[1],'tokens','time')
local tokens=tonumber(state[1]) or capacity
local last=tonumber(state[2]) or now
tokens=math.min(capacity,tokens+math.max(0,now-last)*rate)
local allowed=0
if tokens>=1 then tokens=tokens-1; allowed=1 end
redis.call('HSET',KEYS[1],'tokens',tokens,'time',now)
redis.call('EXPIRE',KEYS[1],math.ceil(capacity/rate)+60)
return allowed
`)
func Allow(ctx context.Context,r *redis.Client,key string,rate,capacity int)(bool,error){n,e:=bucket.Run(ctx,r,[]string{"limit:"+key},rate,capacity).Int();return n==1,e}
func Cached(ctx context.Context,r *redis.Client,key string,dst any)bool{b,e:=r.Get(ctx,key).Bytes();return e==nil&&json.Unmarshal(b,dst)==nil}
func Cache(ctx context.Context,r *redis.Client,key string,value any){b,e:=json.Marshal(value);if e==nil{_ = r.Set(ctx,key,b,5*time.Second).Err()}}
