package infra

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"time"
)

// Hook is the hook callback signature
type Hook func(ctx context.Context, query string, args ...interface{}) (context.Context, error)

// ErrorHook is the error handling callback signature
type ErrorHook func(ctx context.Context, err error, query string, args ...interface{}) error

// Hooks instances may be passed to Wrap() to define an instrumented driver
type Hooks interface {
	Before(ctx context.Context, query string, args ...interface{}) (context.Context, error)
	After(ctx context.Context, query string, args ...interface{}) (context.Context, error)
}

// OnErrorer instances will be called if any error happens
type OnErrorer interface {
	OnError(ctx context.Context, err error, query string, args ...interface{}) error
}

func handlerErr(ctx context.Context, hooks Hooks, err error, query string, args ...interface{}) error {
	h, ok := hooks.(OnErrorer)
	if !ok {
		return err
	}

	if err := h.OnError(ctx, err, query, args...); err != nil {
		return err
	}

	return err
}

// Driver implements a database/sql/driver.Driver
type Driver struct {
	driver.Driver
	hooks Hooks
}

// Open opens a connection
func (drv *Driver) Open(name string) (driver.Conn, error) {
	conn, err := drv.Driver.Open(name)
	if err != nil {
		return conn, err
	}

	wrapped := &Conn{conn, drv.hooks}
	return wrapped, nil
}

// Conn implements a database/sql.driver.Conn
type Conn struct {
	driver.Conn
	hooks Hooks
}

func (conn *Conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	var (
		stmt driver.Stmt
		err  error
	)
	if c, ok := conn.Conn.(driver.ConnPrepareContext); ok {
		stmt, err = c.PrepareContext(ctx, query)
	} else {
		stmt, err = conn.Prepare(query)
	}
	if err != nil {
		if err != nil {
			log.WithError(err).WithField("query", query).Errorf("mysqlerrlog")
		}
		return stmt, err
	}
	return &Stmt{stmt, conn.hooks, query}, nil
}

func (conn *Conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if ciCtx, is := conn.Conn.(driver.ConnBeginTx); is {
		tx, err := ciCtx.BeginTx(ctx, opts)
		if err != nil {
			return tx, err
		}
		return &DriveTx{Tx: tx, start: time.Now()}, nil
	}
	tx, err := conn.Conn.Begin()
	if err != nil {
		return tx, err
	}
	return &DriveTx{Tx: tx, start: time.Now()}, nil
}

type DriveTx struct {
	driver.Tx
	start time.Time
	cost  int64
}

func getStack() *stack {
	return callers()
}

func (d *DriveTx) Commit() error {
	err := d.Tx.Commit()
	d.cost = time.Now().Sub(d.start).Milliseconds()
	if d.cost > 8000 {
		data := log.Fields{
			Cost:       d.cost,
			MetricType: "longTx",
			Stack:      fmt.Sprintf("%+v", getStack()),
		}
		log.WithFields(data).Errorf("mysqlongTxlog ")
	}
	return err
}

func (d *DriveTx) Rollback() error {
	err := d.Tx.Rollback()
	d.cost = time.Now().Sub(d.start).Milliseconds()
	if d.cost > 8000 {
		data := log.Fields{
			Cost:       d.cost,
			MetricType: "longTx",
			Stack:      fmt.Sprintf("%+v", getStack()),
		}
		log.WithFields(data).Errorf("mysqlongTxlog ")
	}
	return err
}

func (conn *Conn) execContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	switch c := conn.Conn.(type) {
	case driver.ExecerContext:
		return c.ExecContext(ctx, query, args)
	case driver.Execer:
		dargs, err := namedValueToValue(args)
		if err != nil {
			return nil, err
		}
		return c.Exec(query, dargs)
	default:
		// This should not happen
		return nil, errors.New("ExecerContext created for a non Execer driver.Conn")
	}
}

func (conn *Conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	var err error

	list := namedToInterface(args)

	// Exec `Before` Hooks
	if ctx, err = conn.hooks.Before(ctx, query, list...); err != nil {
		return nil, err
	}

	results, err := conn.execContext(ctx, query, args)
	if err != nil {
		return results, handlerErr(ctx, conn.hooks, err, query, list...)
	}

	if _, err := conn.hooks.After(ctx, query, list...); err != nil {
		return nil, err
	}

	return results, err
}

func (conn *Conn) queryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch c := conn.Conn.(type) {
	case driver.QueryerContext:
		return c.QueryContext(ctx, query, args)
	case driver.Queryer:
		dargs, err := namedValueToValue(args)
		if err != nil {
			return nil, err
		}
		return c.Query(query, dargs)
	default:
		// This should not happen
		return nil, errors.New("QueryerContext created for a non Queryer driver.Conn")
	}
}

func (conn *Conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	var err error

	list := namedToInterface(args)

	// Query `Before` Hooks
	if ctx, err = conn.hooks.Before(ctx, query, list...); err != nil {
		return nil, err
	}

	results, err := conn.queryContext(ctx, query, args)
	if err != nil {
		return results, handlerErr(ctx, conn.hooks, err, query, list...)
	}

	if _, err := conn.hooks.After(ctx, query, list...); err != nil {
		return nil, err
	}

	return results, err
}

// Stmt implements a database/sql/driver.Stmt
type Stmt struct {
	driver.Stmt
	hooks Hooks
	query string
}

func (stmt *Stmt) execContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if s, ok := stmt.Stmt.(driver.StmtExecContext); ok {
		return s.ExecContext(ctx, args)
	}
	values := make([]driver.Value, len(args))
	for _, arg := range args {
		values[arg.Ordinal-1] = arg.Value
	}

	return stmt.Exec(values)
}

func (stmt *Stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	var err error

	list := namedToInterface(args)

	// Exec `Before` Hooks
	if ctx, err = stmt.hooks.Before(ctx, stmt.query, list...); err != nil {
		return nil, err
	}

	results, err := stmt.execContext(ctx, args)
	if err != nil {
		return results, handlerErr(ctx, stmt.hooks, err, stmt.query, list...)
	}

	if _, err := stmt.hooks.After(ctx, stmt.query, list...); err != nil {
		return nil, err
	}

	return results, err
}

func (stmt *Stmt) queryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if s, ok := stmt.Stmt.(driver.StmtQueryContext); ok {
		return s.QueryContext(ctx, args)
	}

	values := make([]driver.Value, len(args))
	for _, arg := range args {
		values[arg.Ordinal-1] = arg.Value
	}
	return stmt.Query(values)
}

func (stmt *Stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	var err error

	list := namedToInterface(args)

	// Exec Before Hooks
	if ctx, err = stmt.hooks.Before(ctx, stmt.query, list...); err != nil {
		return nil, err
	}

	rows, err := stmt.queryContext(ctx, args)
	if err != nil {
		return rows, handlerErr(ctx, stmt.hooks, err, stmt.query, list...)
	}

	if _, err := stmt.hooks.After(ctx, stmt.query, list...); err != nil {
		return nil, err
	}

	return rows, err
}

// Wrap is used to create a new instrumented driver, it takes a vendor specific driver, and a Hooks instance to produce a new driver instance.
// It's usually used inside a sql.Register() statement
func Wrap(driver driver.Driver, hooks Hooks) driver.Driver {
	return &Driver{driver, hooks}
}

func namedToInterface(args []driver.NamedValue) []interface{} {
	list := make([]interface{}, len(args))
	for i, a := range args {
		list[i] = a.Value
	}
	return list
}

// namedValueToValue copied from database/sql
func namedValueToValue(named []driver.NamedValue) ([]driver.Value, error) {
	dargs := make([]driver.Value, len(named))
	for n, param := range named {
		if len(param.Name) > 0 {
			return nil, errors.New("sql: driver does not support the use of Named Parameters")
		}
		dargs[n] = param.Value
	}
	return dargs, nil
}
