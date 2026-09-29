package db

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCaseAssignmentAndSentinelFilter(t *testing.T) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("SQLD_DYNAMIC_TEST_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, `CREATE TEMP TABLE profiles(id bigint PRIMARY KEY, nickname text);
INSERT INTO profiles VALUES (1,'original'), (2,NULL), (3,'other');`); err != nil {
		t.Fatal(err)
	}
	q := New(conn)
	for _, dynamic := range []bool{false, true} {
		name := "changed"
		for _, tc := range []struct {
			name  string
			set   bool
			value *string
			want  *string
		}{
			{"keep", false, nil, ptr("original")},
			{"clear", true, nil, nil},
			{"write", true, &name, &name},
		} {
			t.Run(tc.name+map[bool]string{true: "/dynamic", false: "/static"}[dynamic], func(t *testing.T) {
				if _, err := conn.Exec(ctx, "UPDATE profiles SET nickname='original' WHERE id=1"); err != nil {
					t.Fatal(err)
				}
				var rows int64
				var err error
				if dynamic {
					rows, err = q.DynamicAssignNickname(ctx, DynamicAssignNicknameParams{SetName: tc.set, Name: tc.value, Ids: []int64{1}})
				} else {
					rows, err = q.AssignNickname(ctx, AssignNicknameParams{SetName: tc.set, Name: tc.value, ID: 1})
				}
				if err != nil || rows != 1 {
					t.Fatalf("update: rows=%d error=%v", rows, err)
				}
				var got *string
				if err := conn.QueryRow(ctx, "SELECT nickname FROM profiles WHERE id=1").Scan(&got); err != nil {
					t.Fatal(err)
				}
				if (got == nil) != (tc.want == nil) || got != nil && *got != *tc.want {
					t.Fatalf("nickname=%v want=%v", got, tc.want)
				}
			})
		}
	}
	for _, tc := range []struct {
		filter string
		want   []int64
	}{
		{"", []int64{1, 2, 3}}, {"changed", []int64{1}}, {"missing", nil},
	} {
		t.Run("filter/"+tc.filter, func(t *testing.T) {
			rows, err := q.ListByNickname(ctx, tc.filter)
			if err != nil {
				t.Fatal(err)
			}
			var got []int64
			for _, row := range rows {
				got = append(got, row.ID)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("static ids=%v want=%v", got, tc.want)
			}
			dynamicRows, err := q.DynamicListByNickname(ctx, DynamicListByNicknameParams{Filter: tc.filter, Ids: []int64{1, 2, 3}})
			if err != nil {
				t.Fatal(err)
			}
			got = nil
			for _, row := range dynamicRows {
				got = append(got, row.ID)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("dynamic ids=%v want=%v", got, tc.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }
