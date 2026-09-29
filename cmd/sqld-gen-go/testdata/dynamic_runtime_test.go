package db

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type captureDB struct {
	DBTX
	sql  string
	args []any
}

func (d *captureDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	d.sql, d.args = sql, args
	return d.DBTX.Query(ctx, sql, args...)
}

func (d *captureDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	d.sql, d.args = sql, args
	return d.DBTX.Exec(ctx, sql, args...)
}

func TestGeneratedPredicates(t *testing.T) {
	var cases []struct {
		Name     string
		Mutation bool
		Want     [][]int64
	}
	data, err := os.ReadFile("cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("SQLD_DYNAMIC_TEST_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for _, tc := range cases {
		for mode, ids := range [][]int64{nil, {}, {1, 3}} {
			t.Run(tc.Name+"/"+[]string{"nil", "empty", "populated"}[mode], func(t *testing.T) {
				tx, err := conn.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				_, err = tx.Exec(ctx, `CREATE TEMP TABLE profiles (id bigint PRIMARY KEY, tenant_id bigint NOT NULL, enabled boolean NOT NULL);
				INSERT INTO profiles VALUES (1,7,false),(2,7,false),(3,8,false),(4,8,false);`)
				if err != nil {
					t.Fatal(err)
				}
				db := &captureDB{DBTX: tx}
				method := reflect.ValueOf(New(db)).MethodByName(tc.Name)
				if !method.IsValid() {
					t.Fatalf("missing method %s", tc.Name)
				}
				arg := reflect.New(method.Type().In(1)).Elem()
				for i := 0; i < arg.NumField(); i++ {
					field := arg.Field(i)
					switch arg.Type().Field(i).Name {
					case "Ids":
						field.Set(reflect.ValueOf(ids))
					case "TenantID":
						field.SetInt(7)
					case "Flag":
						if field.Kind() != reflect.Pointer {
							t.Fatalf("nullable flag lost pointer type: %v", field.Type())
						}
						if mode != 0 {
							field.Set(reflect.ValueOf(new(bool)))
						}
					case "OrderBy":
						if mode != 0 {
							field.SetString("id")
						}
					case "OrderDir":
						field.SetString("DESC")
					case "Limit":
						if field.Kind() == reflect.Pointer {
							if mode != 0 {
								limit := int64(100)
								field.Set(reflect.ValueOf(&limit))
							}
						} else {
							field.SetInt(100)
						}
					default:
						t.Fatalf("unexpected field %s", arg.Type().Field(i).Name)
					}
				}
				out := method.Call([]reflect.Value{reflect.ValueOf(ctx), arg})
				if !out[len(out)-1].IsNil() {
					t.Fatalf("execution: %v\nSQL=%s\nargs=%#v", out[len(out)-1].Interface(), db.sql, db.args)
				}
				if tc.Mutation {
					if got := out[0].Int(); got != int64(len(tc.Want[mode])) {
						t.Fatalf("affected=%d want=%d SQL=%s args=%#v", got, len(tc.Want[mode]), db.sql, db.args)
					}
					// The count must not hide writes to a different tenant.
					var untouched int
					if err := tx.QueryRow(ctx, "SELECT count(*) FROM profiles WHERE tenant_id=8 AND enabled=false").Scan(&untouched); err != nil {
						t.Fatal(err)
					}
					if untouched != 2 {
						t.Fatalf("modified another tenant: %s", db.sql)
					}
				} else {
					var got []int64
					for i := 0; i < out[0].Len(); i++ {
						got = append(got, out[0].Index(i).FieldByName("ID").Int())
					}
					slices.Sort(got)
					if !slices.Equal(got, tc.Want[mode]) {
						t.Fatalf("ids=%v want=%v\nSQL=%s\nargs=%#v", got, tc.Want[mode], db.sql, db.args)
					}
				}
			})
		}
	}
}
