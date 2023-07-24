package infra

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

func registerHookDriver(dbname string) {
	sql.Register(dbname, Wrap(mysql.MySQLDriver{}, &HookDb{dbName: dbname}))
}

// HookDb satisfies the sql hook.Hooks interface
type HookDb struct {
	dbName string
	app    string
}

const (
	ctxKeyOp         = "op"
	ctxKeySql        = "sql"
	ctxKeyMultiTable = "multi_table"
	ctxKeyTbName     = "tbname"
	ctxKeyBeginTime  = "begin"
)

// Before hook will print the query with it's args and return the context with the timestamp
func (h *HookDb) Before(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	ctx = context.WithValue(ctx, ctxKeyBeginTime, time.Now())

	tables, op, err := SqlMonitor.parseTable(query)
	if err != nil || op == Unknown {
		log.WithError(err).WithField(ctxKeySql, query).WithField("op", op).Error("parse sql fail")
	}
	if len(tables) >= 2 {
		ctx = context.WithValue(ctx, ctxKeyMultiTable, 1)
	}
	if len(tables) >= 1 {
		ctx = context.WithValue(ctx, ctxKeyTbName, tables[0])
		RecordClientCount(TypeMySQL, string(op), tables[0], h.dbName)
	}
	if op != Unknown {
		ctx = context.WithValue(ctx, ctxKeyOp, op)
	}
	return ctx, nil
}

func truncateKey(length int, key string) string {
	if len(key) > length {
		return key[:length]
	}
	return key
}

// After hook will get the timestamp registered on the Before hook and print the elapsed time
func (h *HookDb) After(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	beginTime := time.Now()
	if begin := ctx.Value(ctxKeyBeginTime); begin != nil {
		beginTime = begin.(time.Time)
	}
	now := time.Now()
	tableName := ""
	if tbnameInf := ctx.Value(ctxKeyTbName); tbnameInf != nil && len(tbnameInf.(string)) != 0 {
		tableName = tbnameInf.(string)
		MetricMonitor.RecordClientHandlerSeconds(TypeMySQL, string(ctx.Value(ctxKeyOp).(SqlOp)), tbnameInf.(string), h.dbName, now.Sub(beginTime).Seconds())
	}
	slowquery := false
	if now.Sub(beginTime).Seconds() >= 1 {
		slowquery = true
		data := log.Fields{
			Cost:        now.Sub(beginTime).Milliseconds(),
			"query":     truncateKey(1024, query),
			"args":      truncateKey(1024, fmt.Sprintf("%v", args)),
			MetricType:  "slowLog",
			"app":       h.app,
			"dbName":    h.dbName,
			"tableName": tableName,
			"op":        ctx.Value(ctxKeyOp),
		}
		log.WithFields(data).Errorf("mysqlslowlog")
	}
	op := ctx.Value(ctxKeyOp).(SqlOp)
	multitable := ctx.Value(ctxKeyMultiTable)
	if !slowquery && (multitable != nil && multitable.(int) == 1) && op == Select {
		data := log.Fields{
			Cost:        now.Sub(beginTime).Milliseconds(),
			"query":     truncateKey(1024, query),
			"args":      truncateKey(1024, fmt.Sprintf("%v", args)),
			MetricType:  "multiTables",
			"app":       h.app,
			"dbName":    h.dbName,
			"tableName": tableName,
			"op":        ctx.Value(ctxKeyOp),
		}
		log.WithFields(data).Warnf("mysqlmultitableslog")
	}
	// 对修改sql进行日志记录
	if op != Select && op != Unknown {
		data := log.Fields{
			Cost:        now.Sub(beginTime).Milliseconds(),
			"query":     truncateKey(1024, query),
			"args":      truncateKey(1024, fmt.Sprintf("%v", args)),
			MetricType:  "oplog",
			"app":       h.app,
			"dbName":    h.dbName,
			"tableName": tableName,
			"op":        ctx.Value(ctxKeyOp),
		}
		log.WithFields(data).Infof("mysqloplog")
	}
	return ctx, nil
}

func (h *HookDb) OnError(ctx context.Context, err error, query string, args ...interface{}) error {
	if err != driver.ErrSkip {
		tableName := ""
		if tbnameInf := ctx.Value(ctxKeyTbName); tbnameInf != nil && len(tbnameInf.(string)) != 0 {
			tableName = tbnameInf.(string)
		}

		beginTime := time.Now()
		if begin := ctx.Value(ctxKeyBeginTime); begin != nil {
			beginTime = begin.(time.Time)
		}
		data := log.Fields{
			Cost:        time.Now().Sub(beginTime).Milliseconds(),
			"query":     truncateKey(1024, query),
			"args":      truncateKey(1024, fmt.Sprintf("%v", args)),
			"app":       h.app,
			"dbName":    h.dbName,
			"tableName": tableName,
			"op":        ctx.Value(ctxKeyOp),
		}
		log.WithFields(data).WithError(err).Errorf("mysqlerrlog")
	}
	return err
}

var (
	LocalDbClient = make(map[string]*sql.DB)
)

type DbInfo struct {
	MaxConn int
	Timeout int
	ConnStr string
	DbName  string
}

func decorateMySQLConn(conn string) string {
	exts := []string{}
	// &timeout=1s&readTimeout=1s&writeTimeout=1s
	if !strings.Contains(conn, "charset") {
		exts = append(exts, "charset=utf8mb4")
	}
	if !strings.Contains(conn, "readTimeout") {
		exts = append(exts, "readTimeout=10s")
	}
	if !strings.Contains(conn, "timeout") {
		exts = append(exts, "timeout=3s")
	}
	if !strings.Contains(conn, "writeTimeout") {
		exts = append(exts, "writeTimeout=10s")
	}
	if len(exts) > 0 {
		if !strings.Contains(conn, "?") {
			return conn + "?" + strings.Join(exts, "&")
		}
		return conn + "&" + strings.Join(exts, "&")
	}
	return conn

}

func (s *sqlMonitor) InitHookDb(dbInfos []*DbInfo) {
	for _, dbInfo := range dbInfos {
		registerHookDriver(dbInfo.DbName)
		maxConn := dbInfo.MaxConn
		timeout := 1
		if dbInfo.Timeout > 0 {
			timeout = dbInfo.Timeout
		}
		if maxConn < 1 {
			maxConn = 5
		}
		maxIdleConn := maxConn / 5
		if maxIdleConn < 1 {
			maxIdleConn = 1
		}
		connStr := dbInfo.ConnStr
		db := LocalDbClient[dbInfo.DbName]
		if db == nil {
			var err error
			db, err = sql.Open(dbInfo.DbName, decorateMySQLConn(connStr))
			if err != nil {
				log.WithError(err).WithField("conn", connStr).Error("open mysql fail")
			}
			db.SetConnMaxLifetime(time.Duration(timeout) * time.Hour) //reconnect after 1 hour
			db.SetMaxOpenConns(maxConn)
			db.SetMaxIdleConns(maxIdleConn)
			LocalDbClient[dbInfo.DbName] = db

		}
	}

}
