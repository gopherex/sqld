// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"github.com/stephenafamo/bob/clause"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
)

var (
	SelectWhere     = Where[*dialect.SelectQuery]()
	UpdateWhere     = Where[*dialect.UpdateQuery]()
	DeleteWhere     = Where[*dialect.DeleteQuery]()
	OnConflictWhere = Where[*clause.ConflictClause]() // Used in ON CONFLICT DO UPDATE
)

func Where[Q psql.Filterable]() struct {
	Users        userWhere[Q]
	Profiles     profileWhere[Q]
	Orders       orderWhere[Q]
	Roles        roleWhere[Q]
	UserRoles    userRoleWhere[Q]
	KitchenSinks kitchenSinkWhere[Q]
	Logs         logWhere[Q]
} {
	return struct {
		Users        userWhere[Q]
		Profiles     profileWhere[Q]
		Orders       orderWhere[Q]
		Roles        roleWhere[Q]
		UserRoles    userRoleWhere[Q]
		KitchenSinks kitchenSinkWhere[Q]
		Logs         logWhere[Q]
	}{
		Users:        buildUserWhere[Q](Users.Columns),
		Profiles:     buildProfileWhere[Q](Profiles.Columns),
		Orders:       buildOrderWhere[Q](Orders.Columns),
		Roles:        buildRoleWhere[Q](Roles.Columns),
		UserRoles:    buildUserRoleWhere[Q](UserRoles.Columns),
		KitchenSinks: buildKitchenSinkWhere[Q](KitchenSinks.Columns),
		Logs:         buildLogWhere[Q](Logs.Columns),
	}
}
