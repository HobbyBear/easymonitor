package infra

import (
	"fmt"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

var (
	monitorKeys = make([]string, 0, 100)
)

type redisMonitor struct {
}

var RedisMonitor = &redisMonitor{}

func (r *redisMonitor) AddMonitorKey(keyPrefix string) {
	monitorKeys = append(monitorKeys, keyPrefix)
}

func matchKey(key string) (string, bool) {
	var (
		matchKey string
	)
	for _, k := range monitorKeys {
		if strings.Contains(key, k) {
			matchKey = k
			break
		}
	}
	if len(matchKey) == 0 {
		return "", false
	}
	return getCmdFromKey(key) + " " + matchKey, true
}

func getCmdFromKey(key string) string {
	cmds := strings.Split(key, " ")
	if len(cmds) > 0 {
		return cmds[0]
	}
	return ""
}

func (r *redisMonitor) AddRedisHook(client *redis.Client, redisInstanceName string) {
	client.WrapProcessPipeline(func(oldProcess func([]redis.Cmder) error) func([]redis.Cmder) error {
		return func(cmders []redis.Cmder) error {
			start := time.Now()
			for _, cmd := range cmders {
				dealKey, match := matchKey(truncateKey(100, strings.TrimSuffix(strings.TrimLeft(fmt.Sprintf("%v", cmd.Args()), "["), "]")))
				if match {
					RecordClientCount(TypeRedis, cmd.Name(), dealKey, redisInstanceName)
				}
			}
			err := oldProcess(cmders)
			for _, cmd := range cmders {
				cacheWrapper(cmd, start, err, redisInstanceName)
			}
			return err
		}
	})

	client.WrapProcess(func(oldProcess func(cmd redis.Cmder) error) func(cmd redis.Cmder) error {
		return func(cmd redis.Cmder) error {
			start := time.Now()
			dealKey, match := matchKey(truncateKey(100, strings.TrimSuffix(strings.TrimLeft(fmt.Sprintf("%v", cmd.Args()), "["), "]")))
			if match {
				RecordClientCount(TypeRedis, cmd.Name(), dealKey, redisInstanceName)
			}
			err := oldProcess(cmd)
			cacheWrapper(cmd, start, err, redisInstanceName)
			return err
		}
	})

}

func cacheWrapper(cmd redis.Cmder, start time.Time, err error, app string) {
	key := strings.TrimSuffix(strings.TrimLeft(fmt.Sprintf("%v", cmd.Args()), "["), "]")
	key = truncateKey(100, key)
	now := time.Now()

	dealKey, match := matchKey(truncateKey(100, strings.TrimSuffix(strings.TrimLeft(fmt.Sprintf("%v", cmd.Args()), "["), "]")))
	if match {
		MetricMonitor.RecordClientHandlerSeconds(TypeRedis, cmd.Name(), dealKey, app, float64(now.UnixNano()/1000000-start.UnixNano()/1000000)/1000)
	}
	if err != nil && err != redis.Nil {
		fields := log.Fields{
			"app":      app,
			"key":      key,
			"name":     cmd.Name(),
			"duration": time.Since(now).String(),
		}
		log.WithError(err).WithFields(fields).Error("rediserrlog")
	}
}
