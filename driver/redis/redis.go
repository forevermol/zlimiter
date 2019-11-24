package redis

import (
	"errors"
	"log"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/zzerroo/zlimiter/driver/common"
)

const (
	// DefaultRedisMaxIdle 连接池中即使没有redis连接仍然需要保持60个空闲连接数,建议根据突发连接数设置辞职
	DefaultRedisMaxIdle = 60
	// DefaultRedisMaxActive 连接池最大打开200个redis连接,如果高并发场景建议提高此值
	DefaultRedisMaxActive = 500
	// DefaultRedisIdleTimeout 空闲连接超时时间，超时后此空闲连接将被关闭
	DefaultRedisIdleTimeout = 0
	// DefaultRedisConnTimeout 连接超时时间，无超时
	DefaultRedisConnTimeout = 0
	// DefaultRedisReadTimeout 读超时时间,1000毫秒,建议根据实际情况调整
	DefaultRedisReadTimeout = 1000
	// DefaultRedisWriteTimeout 写超时时间,3000毫秒,建议根据实际情况调整
	DefaultRedisWriteTimeout = 3000

	// RedisAddScript Add redis.Script
	RedisAddScript = "ADD"
	// RedisDelScript Del redis.Script
	RedisDelScript = "DEL"
	// RedisGetScirpt Get redis.Script
	RedisGetScirpt = "GET"
	// RedisSetScript Set redis.Script
	RedisSetScript = "SET"

	redisScript       = "SCRIPT"
	redisScriptExists = "EXISTS"
)

// RedisInfo redis连接信息，address为ip:port格式
type RedisInfo struct {
	Address string
	Passwd  string
}

// RedisProxy redis连接代理，用于维护连接池和相关脚本
type RedisProxy struct {
	//Driver
	RedisClient *redis.Pool
	Scripts     map[int]*redis.Script
}

// RedisFixWindow 分布式固定窗口限流器
type RedisFixWindow struct {
	RedisProxy
}

// RedisSlideWindow 分布式滑动窗口限流器
type RedisSlideWindow struct {
	RedisProxy
}

// RedisBucket 分布式Bucket限流器
type RedisBucket struct {
	RedisProxy
}

// RedisToken 分布式Token限流器
type RedisToken struct {
	RedisProxy
}

// Init 负责redis连接池的创建，Add、Get、Del、Set脚本的加载。注意如果初始化的过程中发生错误，
//	将log.fatal相关信息并退出
//	Input :
//		args : RedisInfo格式，包含连接redis的必要信息，注意address为ip:port格式的字符串
//	Output :
//		error : 成功为nil，否则为相关错误信息
func (r *RedisProxy) Init(args ...interface{}) error {
	if len(args) != 1 {
		log.Fatalf("error bad param:%v", args)
	}

	// 创建连接池
	argsTmp := args[0]
	if redisInfo, ok := argsTmp.(RedisInfo); ok {
		r.RedisClient = &redis.Pool{
			MaxIdle:     DefaultRedisMaxIdle,
			MaxActive:   DefaultRedisMaxActive,
			IdleTimeout: time.Duration(DefaultRedisIdleTimeout) * time.Second,
			Wait:        false, //注意，如果请求连接数>最大连接数，会导致错误，可根据实际情况修改
			Dial: func() (redis.Conn, error) {
				con, err := redis.Dial("tcp", redisInfo.Address,
					redis.DialPassword(redisInfo.Passwd),
					redis.DialConnectTimeout(time.Duration(DefaultRedisConnTimeout)*time.Second),
					redis.DialReadTimeout(time.Duration(DefaultRedisReadTimeout)*time.Millisecond),
					redis.DialWriteTimeout(time.Duration(DefaultRedisWriteTimeout)*time.Millisecond))
				if err != nil {
					log.Fatal(err.Error())
				}

				return con, nil
			},
		}

		// 加载lua脚本至redis
		if erro := r.LoadScript(); erro != nil {
			log.Fatal(erro.Error())
		}

		return nil
	} else {
		log.Fatal(errors.New(common.ErrorInputParam))
	}

	return nil
}

// LoadScript 根据Lua脚本的sha1值判断是否已经加载脚本至redis，如果加载则返回，否则则加班相关脚本
//	Input : 需要加载的脚本的信息位于Scripts对应的map中
//	Output :
//		error : 相关错误信息或者nil
func (r *RedisProxy) LoadScript() error {
	conn := r.RedisClient.Get()
	defer conn.Close()

	for key, script := range r.Scripts {
		if loaded, erro := r.scriptLoaded(conn, script.Hash()); erro == nil && loaded == 0 {
			script.Load(conn)
		} else if erro != nil {
			log.Fatalf(common.ErrorLoadScritp+" %s", key)
		}
	}

	return nil
}

// scriptLoaded 根据sha1值判断是否已经加载过相关脚本
//	Input :
//		conn : 相关redis连接
//		sha1Str : lua脚本的sha1值
//	Output :
//		int : 0代表未加载，否则为已加载
//		error : 相关错误信息或者nil
func (r *RedisProxy) scriptLoaded(conn redis.Conn, sha1Str string) (int, error) {
	rsp, erro := conn.Do(redisScript, redisScriptExists, sha1Str)
	if erro != nil {
		return -1, erro
	}

	rspArray, ok := rsp.([]interface{})
	if !ok || len(rspArray) != 1 {
		return -1, errors.New("error bad type")
	}

	if isLoaded, ok := rspArray[0].(int64); ok {
		return int(isLoaded), nil
	}

	return -1, errors.New("error bad type")
}

// Init 固定窗口限流的初始化，创建相关的redis.Script。该函数必须在RedisProxy.Init之前调用
//	Input :
//	Output :
//		error : nil或者相关错误信息
func (r *RedisFixWindow) Init(args ...interface{}) error {
	r.Scripts = make(map[int]*redis.Script)
	r.Scripts[common.RedisAddScript] = redis.NewScript(1, FixAddStr)
	r.Scripts[common.RedisGetScript] = redis.NewScript(1, FixGetStr)
	r.Scripts[common.RedisSetScript] = redis.NewScript(1, FixSetStr)
	r.Scripts[common.RedisDelScript] = redis.NewScript(1, FixDelStr)

	return r.RedisProxy.Init(args...)
}

// Init 滑动窗口限流的初始化，创建相关的redis.Script。该函数必须在RedisProxy.Init之前调用
//	Input :
//	Output :
//		error : nil或者相关错误信息
func (r *RedisSlideWindow) Init(args ...interface{}) error {
	r.Scripts = make(map[int]*redis.Script)
	r.Scripts[common.RedisAddScript] = redis.NewScript(1, SlideAddStr)
	r.Scripts[common.RedisGetScript] = redis.NewScript(1, SlideGetStr)
	r.Scripts[common.RedisSetScript] = redis.NewScript(1, SlideSetStr)
	r.Scripts[common.RedisDelScript] = redis.NewScript(1, SlideDelStr)

	r.RedisProxy.Init(args...)
	return nil
}

// Init bucket限流的初始化，创建相关的redis.Script。该函数必须在RedisProxy.Init之前调用
//	由于bucket需要对请求量进行控制，该限流器新增了chk脚本用于检验请求量
//	Input :
//	Output :
//		error : nil或者相关错误信息
func (r *RedisBucket) Init(args ...interface{}) error {
	r.Scripts = make(map[int]*redis.Script)
	r.Scripts[common.RedisAddScript] = redis.NewScript(1, BucketAddStr)
	r.Scripts[common.RedisGetScript] = redis.NewScript(1, BucketGetStr)
	r.Scripts[common.RedisSetScript] = redis.NewScript(1, BucketSetAddr)
	r.Scripts[common.RedisDelScript] = redis.NewScript(1, BucketDelAddr)
	r.Scripts[common.ReidsChkScript] = redis.NewScript(1, BucketCheckAddr)

	r.RedisProxy.Init(args...)
	return nil
}

// Init token限流的初始化，创建相关的redis.Script。该函数必须在RedisProxy.Init之前调用
//	Input :
//	Output :
//		error : nil或者相关错误信息
func (r *RedisToken) Init(args ...interface{}) error {
	r.Scripts = make(map[int]*redis.Script)
	r.Scripts[common.RedisAddScript] = redis.NewScript(1, TokenAddStr)
	r.Scripts[common.RedisGetScript] = redis.NewScript(1, TokenGetStr)
	r.Scripts[common.RedisSetScript] = redis.NewScript(1, TokenSetStr)
	r.Scripts[common.RedisDelScript] = redis.NewScript(1, TokenDelStr)

	r.RedisProxy.Init(args...)
	return nil
}

// Add 新增一条规则，该函数会调用RedisAddScript完成相关规则的创建。注意:该函数会调用获取时间机制，请务必保证
//	集群中各主机间时间的同步，具体时间同步建议使用ntp服务,参见:https://linux.die.net/man/8/ntpdate
//	Input :
//		key : 限流标识，用于唯一标识一条限流规则
//		limit : tmDuriation时间段内的限流数，与tmDuriation同时实现tmDuriation时间段内限流limit次的语义
//		tmDuriation : 时间段, 与limit同时实现tmDuriation时间段内限流limit次的语义
//		others : 滑动窗口、固定窗口限流中未用，bucket中表示最大缓存的请求数目，token中表示最大token数量
//	Output :
//		error : nil或者相关错误信息
func (r *RedisProxy) Add(key string, limit int64, tmDuration time.Duration, others ...interface{}) error {
	conn := r.RedisClient.Get()
	defer conn.Close()

	if len(others) == 1 {
		if max, ok := others[0].(int64); ok {
			return r.Scripts[common.RedisAddScript].SendHash(conn, key, limit, tmDuration.Nanoseconds()/1e3, time.Now().UnixNano()/1e3, max)
		}
	} else if len(others) == 0 {
		return r.Scripts[common.RedisAddScript].SendHash(conn, key, limit, tmDuration.Nanoseconds()/1e3, time.Now().UnixNano()/1e3)
	}

	return errors.New(common.ErrorUnknown)
}

// Get 获取key对应规则的信息，包括：剩余请求数是否<0，剩余请求数。注意:集群中各服务器之间应保持时间同步
//	Input :
//		key : 限流标识，用于标识要获取限流相关信息的规则
//	Output :
//		bool : 当前剩余请求数是否<0
//		int64 : 当前剩余请求数目
//		error : 成功为nil，否则为具体错误信息
func (r *RedisProxy) Get(key string) (bool, int64, error) {
	conn := r.RedisClient.Get()
	defer conn.Close()

	rsp, erro := r.Scripts[common.RedisGetScript].Do(conn, key, time.Now().UnixNano()/1e3)
	if erro != nil {
		return false, -1, erro
	}

	if left, ok := rsp.(int64); ok {
		if left == -2 {
			return false, -1, errors.New(common.ErrorItemNotExist)
		} else if left == -1 {
			return true, 0, nil
		}

		return left < 0, left, nil
	}

	return false, 0, errors.New(common.ErrorUnknown)
}

// Set 重置或者新增一条规则，具体参见Add
func (r *RedisProxy) Set(key string, limit int64, tmDuration time.Duration, others ...interface{}) error {
	conn := r.RedisClient.Get()
	defer conn.Close()

	if len(others) == 1 {
		if max, ok := others[0].(int64); ok {
			return r.Scripts[common.RedisSetScript].SendHash(conn, key, limit, tmDuration.Nanoseconds()/1e3, time.Now().UnixNano()/1e3, max)
		}
	} else if len(others) == 0 {
		return r.Scripts[common.RedisSetScript].SendHash(conn, key, limit, tmDuration.Nanoseconds()/1e3, time.Now().UnixNano()/1e3)
	}

	return errors.New(common.ErrorUnknown)
}

// Del ...
func (r *RedisProxy) Del(key string) error {
	conn := r.RedisClient.Get()
	defer conn.Close()

	return r.Scripts[common.RedisDelScript].SendHash(conn, key)
}

// Get ...
func (r *RedisBucket) Get(key string) (bool, int64, error) {
	conn := r.RedisClient.Get()
	defer conn.Close()

	rsp, erro := r.Scripts[common.ReidsChkScript].Do(conn, key)
	if erro != nil {
		return false, -1, erro
	}
	if chckStatus, ok := rsp.(int64); ok {
		if chckStatus == -1 {
			return false, -1, errors.New(common.ErrorReqOverFlow)
		} else if chckStatus == -2 {
			return false, -1, errors.New(common.ErrorItemNotExist)
		}
	}

	rsp, erro = r.Scripts[common.RedisGetScript].Do(conn, key, time.Now().UnixNano()/1e3)
	if erro != nil {
		return false, -1, erro
	}

	if waitTm, ok := rsp.(int64); ok {
		if waitTm >= 0 {
			time.Sleep(time.Duration(waitTm) * time.Microsecond)
			return false, -1, nil
		} else if waitTm == -1 {
			return false, -1, errors.New(common.ErrorReqOverFlow)
		}
	}

	return false, 0, errors.New(common.ErrorUnknown)
}
