package infra

import (
	"fmt"
	"testing"
)

func TestParseSql(t *testing.T) {
	sql := `
create index t_agency_email_index
    on t_agency (email);

`
	fmt.Println(SqlParser.parseTable(sql))
}
